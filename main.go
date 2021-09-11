package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

const (
	deviceDirectory = "secrets/devices"
	locationFile    = "secrets/home.loc"

	listenAddr    = ":9000"
	offset        = time.Hour
	runTransition = 15 * time.Minute
	retryLimit    = 5

	lifxPort            = 56700
	maxuint16           = 65535
	maxuint32           = 4294967295 // in ms = ~1193 hours

	KelvinNeutral = 3500
)

var InsecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

type Lamplighter struct {
	Devices  map[string]light.Device
	Location Location
	errCount uint
}

func (l Lamplighter) Run() {
	for _, device := range l.Devices {
		go func() {
			err := setBrightness(device, maxuint16, runTransition)
			if err != nil {
				log.Printf("ERR: %s\n", err)
			}
		}()
	}
}

func (l Lamplighter) Next(now time.Time) time.Time {
	var sunset time.Time
	var err error

	sunset, err = getSunset(l.Location, now)
	if err != nil {
		log.Printf("get sunset: %s", err)
		if l.errCount >= retryLimit {
			return time.Time{}
		}

		l.errCount++
		return time.Now().Add(time.Minute)
	}

	lightTime := sunset.Add(-1 * offset)
	if now.After(lightTime) || now.Equal(lightTime) {
		sunset, err = getSunset(l.Location, now.AddDate(0, 0, 1))
		if err != nil {
			log.Printf("get sunset: %s", err)
			if l.errCount >= retryLimit {
				return time.Time{}
			}

			l.errCount++
			return time.Now().Add(time.Minute)
		}
		lightTime = sunset.Add(-1 * offset)
	}

	l.errCount = 0
	log.Printf("next lamp: %s", lightTime.Local().Format(time.RFC3339))
	return lightTime
}

func setBrightness(device light.Device, brightness uint16, transition time.Duration) error {
	conn, err := device.Dial()
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	color := &lifxlan.Color{
		Hue: 0, // technically green
		Saturation: 0,
		Brightness: brightness,
		Kelvin: KelvinNeutral,
	}

	err = device.SetColor(ctx, conn, color, transition, false)
	if err != nil {
		return fmt.Errorf("set color: %w", err)
	}

	return nil
}

func scanDevice(filename string) (light.Device, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open device file: %w", err)
	}

	scanner := bufio.NewScanner(f)

	scanner.Scan()
	addr := scanner.Text()

	scanner.Scan()
	mac := scanner.Text()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan device file: %w", err)
	}

	host := fmt.Sprintf("%s:%d", addr, lifxPort)
	target, err := lifxlan.ParseTarget(mac)
	if err != nil {
		return nil, fmt.Errorf("parse mac address: %w", err)
	}

	device := lifxlan.NewDevice(host, lifxlan.ServiceUDP, target)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bulb, err := light.Wrap(ctx, device, false)
	if err != nil {
		return nil, fmt.Errorf("device is not a light: %w", err)
	}

	err = device.GetLabel(ctx, nil)
	if err != nil {
		log.Printf("ERR: get device label: %s", err)
	}

	return bulb, nil
}

func newPowerHandler(device light.Device) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		var brightness uint16
		if _, ok := r.Form["brightness"]; ok {
			param := r.FormValue("brightness")
			p, err := strconv.Atoi(param)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "unable to parse brightness parameter"}`))
				return
			}

			if p < 0 {
				p = 0
			} else if p > 100 {
				p = 100
			}

			brightness = uint16(math.Floor((float64(p) / 100.0) * float64(maxuint16)))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "brightness parameter is required"}`))
		}

		transition := 2 * time.Second
		if _, ok := r.Form["transition"]; ok {
			param := r.FormValue("transition")
			_, err := strconv.Atoi(param)
			if err == nil && param != "" {
				param = param + "ms"
			}

			parsed, err := time.ParseDuration(param)
			if err != nil {
				log.Printf("ERR: parse transition param %q: %s", param, err)
			}
			transition = parsed
		}

		err := setBrightness(device, brightness, transition)
		if err != nil {
			log.Printf("ERR: %s: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to set brightness on device"}`))
			return
		}

		fmt.Fprintf(w, `{"brightness": %s}`, r.FormValue("brightness"))
	})
}

func newStatusHandler(device light.Device) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := device.Dial()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to connect to device"}`))
			return
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		color, err := device.GetColor(ctx, conn)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to get device brightness state"}`))
			return
		}

		fmt.Fprintf(w, `{"brightness": %.2f}`, float64(color.Brightness) / float64(maxuint16))
	})
}

func main() {
	// manually set local timezone for docker container
	if tz := os.Getenv("TZ"); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			log.Fatalf("load tz location: %s", err)
		}
		time.Local = loc
	}

	locationBytes, err := ioutil.ReadFile(locationFile)
	if err != nil {
		log.Fatalf("read location file failed: %s", err)
	}

	var location Location
	err = json.Unmarshal(locationBytes, &location)
	if err != nil {
		log.Printf("unmarshal location: %s", err)
	}

	devDir, err := os.Open(deviceDirectory)
	if err != nil {
		log.Fatalf("open devices file failed: %s", err)
	}

	deviceFiles, err := devDir.Readdirnames(0)
	if err != nil {
		log.Fatalf("read device directory: %s", err)
	}

	devices := make(map[string]light.Device)
	for _, filename := range deviceFiles {
		filepath := path.Join(deviceDirectory, filename)
		device, err := scanDevice(filepath)
		if err != nil {
			log.Printf("ERR: %s", err)
			continue
		}

		label := device.Label().String()
		key := strings.ToLower(label)
		devices[key] = device
		log.Printf("registered device: %q", key)
	}

	lamp := Lamplighter{
		Devices:  devices,
		Location: location,
	}

	now := time.Now()
	sunset, err := getSunset(lamp.Location, now)
	if err != nil {
		panic(err)
	}

	if now.After(sunset.Add(-1 * offset)) {
		lamp.Run()
	}

	cron := cron.New()
	cron.Schedule(lamp, lamp)

	mux := http.NewServeMux()
	for label, device := range devices {
		dev := fmt.Sprintf("/%s", label)
		mux.Handle(dev, newPowerHandler(device))

		status := fmt.Sprintf("/%s/status", label)
		mux.Handle(status, newStatusHandler(device))
	}

	srv := http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	log.Printf("listening on %s", srv.Addr)

	cron.Start()
	log.Fatal(srv.ListenAndServe())
}
