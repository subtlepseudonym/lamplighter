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
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

const (
	deviceDirectory = "secrets/devices"
	locationFile    = "secrets/home.loc"

	listenAddr = ":9000"
	retryLimit = 5
	lifxPort   = 56700

	defaultTransition = 15 * time.Minute // duration over which to turn on devices
	defaultOffset     = time.Hour        // duration before sunset to start transition

	maxuint16 = 65535
	maxuint32 = 4294967295 // in ms = ~1193 hours

	KelvinNeutral = 3000
)

var InsecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

func scanDevice(filename string) (*Device, error) {
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

	conn, err := device.Dial()
	if err != nil {
		return nil, fmt.Errorf("dial device: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bulb, err := light.Wrap(ctx, device, false)
	if err != nil {
		return nil, fmt.Errorf("device is not a light: %w", err)
	}

	err = device.GetLabel(ctx, conn)
	if err != nil {
		log.Printf("get device label: %s", err)
	}

	return &Device{bulb}, nil
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

	lamp := New(location, defaultTransition, defaultOffset)

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

		label := device.Label().String()
		key := strings.ToLower(label)
		lamp.Devices[key] = device
		log.Printf("registered device: %q", key)
	}

	now := time.Now()
	sunset, err := getSunset(location, now)
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
