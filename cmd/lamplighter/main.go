package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/subtlepseudonym/lamplighter"

	"github.com/robfig/cron/v3"
	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

const (
	deviceDirectory = "secrets/devices"
	locationFile    = "secrets/home.loc"

	listenAddr = ":9000"
	lifxPort   = 56700

	defaultTransition = 15 * time.Minute // duration over which to turn on devices
	defaultOffset     = time.Hour        // duration before sunset to start transition
)

func scanDevice(filename string) (*lamplighter.Device, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open device file: %w", err)
	}

	scanner := bufio.NewScanner(f)

	scanner.Scan()
	host := fmt.Sprintf("%s:%d", scanner.Text(), lifxPort)

	name := path.Base(filename)
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan device file %q: %w", name, err)
	}

	scanner.Scan()
	target, err := lifxlan.ParseTarget(scanner.Text())
	if err != nil {
		return nil, fmt.Errorf("%s: parse mac address: %w", name, err)
	}

	device := lifxlan.NewDevice(host, lifxlan.ServiceUDP, target)

	conn, err := device.Dial()
	if err != nil {
		return nil, fmt.Errorf("%s: dial device: %w", name, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = device.GetHardwareVersion(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("%s: get hardware version: %w", name, err)
	}

	bulb, err := light.Wrap(ctx, device, false)
	if err != nil {
		return nil, fmt.Errorf("%s: device is not a light: %w", name, err)
	}

	scanner.Scan()
	var brightness uint16
	b, err := strconv.ParseUint(scanner.Text(), 10, 16)
	if err != nil {
		brightness = math.MaxUint16
	} else {
		brightness = uint16(b)
	}

	scanner.Scan()
	var transition time.Duration
	transition, err = time.ParseDuration(scanner.Text())
	if err != nil {
		transition = defaultTransition
	}

	d := &lamplighter.Device{
		Device:     bulb,
		Name:       name,
		Brightness: brightness,
		Transition: transition,
	}
	return d, nil
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

	var location lamplighter.Location
	err = json.Unmarshal(locationBytes, &location)
	if err != nil {
		log.Printf("unmarshal location: %s", err)
	}

	lamp := lamplighter.New(location, defaultOffset)

	devDir, err := os.Open(deviceDirectory)
	if err != nil {
		log.Fatalf("open devices file failed: %s", err)
	}

	deviceFiles, err := devDir.Readdirnames(0)
	if err != nil {
		log.Fatalf("read device directory: %s", err)
	}

	for _, filename := range deviceFiles {
		filepath := path.Join(deviceDirectory, filename)
		device, err := scanDevice(filepath)
		if err != nil {
			log.Printf("ERR: %s", err)
			continue
		}

		lamp.Devices[device.Name] = device
		log.Printf("registered device: %q %s %v", device.Name, device.HardwareVersion(), device)
	}

	now := time.Now()
	sunset, err := lamplighter.GetSunset(location, now)
	if err != nil {
		panic(err)
	}

	if now.After(sunset.Add(-1 * defaultOffset)) {
		lamp.Run()
	}

	cron := cron.New()
	cron.Schedule(lamp, lamp)

	mux := http.NewServeMux()
	for label, device := range lamp.Devices {
		dev := fmt.Sprintf("/%s", label)
		mux.HandleFunc(dev, device.PowerHandler)

		status := fmt.Sprintf("/%s/status", label)
		mux.HandleFunc(status, device.StatusHandler)
	}

	srv := http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	log.Printf("listening on %s", srv.Addr)

	cron.Start()
	log.Fatal(srv.ListenAndServe())
}
