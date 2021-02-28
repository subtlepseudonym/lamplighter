package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.yhsif.com/lifxlan"
)

const (
	deviceDirectory = "secrets/devices"
	locationFile    = "secrets/home.loc"

	listenAddr    = ":9000"
	offset        = 4 * time.Hour
	runTransition = 15 * time.Minute
	retryLimit    = 5

	lifxPort            = 56700
	setPowerMessageType = 117
	maxuint16           = 65535
	maxuint32           = 4294967295 // in ms = ~1193 hours
)

var InsecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

type Lamplighter struct {
	Devices  map[string]lifxlan.Device
	Location Location
	errCount uint
}

type SetPowerPayload struct {
	Level    uint16
	Duration uint32
}

func (l Lamplighter) Run() {
	for _, device := range l.Devices {
		go func() {
			err := setPower(device, maxuint16, runTransition)
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

func setPower(device lifxlan.Device, desiredPower uint16, transition time.Duration) error {
	conn, err := device.Dial()
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	duration := transition.Milliseconds()
	if duration < 0 {
		duration *= -1
	}
	if duration > maxuint32 {
		duration = maxuint32
	}

	payload := SetPowerPayload{
		Level:    desiredPower,
		Duration: uint32(duration),
	}

	_, err = device.Send(ctx, conn, lifxlan.FlagAckRequired, setPowerMessageType, &payload)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}

	return nil
}

func scanDevice(filename string) (lifxlan.Device, error) {
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

	err = device.GetLabel(ctx, nil)
	if err != nil {
		log.Printf("ERR: get device label: %s", err)
	}

	return device, nil
}

func newDeviceHandler(device lifxlan.Device, desiredPower uint16) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

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

		err := setPower(device, desiredPower, transition)
		if err != nil {
			log.Printf("ERR: %s: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to set power on device"}`))
			return
		}

		if desiredPower > 0 {
			w.Write([]byte(`{"status": "on"}`))
		} else {
			w.Write([]byte(`{"status": "off"}`))
		}
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

	devices := make(map[string]lifxlan.Device)
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
		on := fmt.Sprintf("/%s/on", label)
		mux.Handle(on, newDeviceHandler(device, maxuint16))

		off := fmt.Sprintf("/%s/off", label)
		mux.Handle(off, newDeviceHandler(device, uint16(0)))
	}

	srv := http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	log.Printf("listening on %s", srv.Addr)

	cron.Start()
	log.Fatal(srv.ListenAndServe())
}
