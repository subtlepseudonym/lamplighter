package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/subtlepseudonym/lamplighter"
	"github.com/subtlepseudonym/lamplighter/cmd/lamplighter/config"

	"github.com/robfig/cron/v3"
	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

const (
	configFile = "secrets/lamp.cfg"

	listenAddr   = ":9000"
	lifxPort     = 56700
	sunsetPrefix = "@sunset"
)

type Job struct {
	Device     *lamplighter.Device
	Color      *lifxlan.Color
	Transition time.Duration
}

func (j Job) Run() {
	log.Printf("transitioning %q over %s", j.Device.Label, j.Transition)
	err := j.Device.Transition(j.Color, j.Transition)
	if err != nil {
		log.Printf("ERR: transition device: %s", err)
	}
}

func scanDevice(label string, dev config.Device) (*lamplighter.Device, error) {
	host := fmt.Sprintf("%s:%d", dev.IP, lifxPort)
	target, err := lifxlan.ParseTarget(dev.MAC)
	if err != nil {
		return nil, fmt.Errorf("%s: parse mac address: %w", label, err)
	}

	device := lifxlan.NewDevice(host, lifxlan.ServiceUDP, target)
	conn, err := device.Dial()
	if err != nil {
		return nil, fmt.Errorf("%s: dial device: %w", label, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = device.GetHardwareVersion(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("%s: get hardware version: %w", label, err)
	}

	bulb, err := light.Wrap(ctx, device, false)
	if err != nil {
		return nil, fmt.Errorf("%s: device is not a light: %w", label, err)
	}

	if j.Device.Label() == nil {
		err := j.Device.GetLabel(ctx, conn)
		if err != nil {
			log.Printf("ERR: get device label: %w", err)
			return
		}
	}

	label := strings.ToLower(j.Device.Label().String())
	dev := &lamplighter.Device{
		Device: bulb,
		Label: label,
	}
	return dev, nil
}

func main() {
	// manually set local timezone for docker container
	if tz := os.Getenv("TZ"); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			log.Fatalf("ERR: load tz location: %s", err)
		}
		time.Local = loc
	}

	config, err := config.Open(configFile)
	if err != nil {
		log.Fatalf("ERR: read config file failed: %s", err)
	}

	devices := make(map[string]*lamplighter.Device)
	for label, dev := range config.Devices {
		device, err := scanDevice(label, dev)
		if err != nil {
			log.Printf("ERR: scan device: %s", err)
			continue
		}
		devices[label] = device
		log.Printf("registered device: %q %s", label, device.HardwareVersion())
	}

	lightCron := cron.New()
	for _, job := range config.Jobs {
		var schedule cron.Schedule
		var err error

		if strings.HasPrefix(job.Schedule, sunsetPrefix) {
			s := strings.Split(job.Schedule, " ")

			var duration time.Duration
			if len(s) > 1 {
				duration, err = time.ParseDuration(s[1])
				if err != nil {
					log.Printf("ERR: parse sunset offset: %s", err)
					continue
				}
			}
			schedule = lamplighter.SunsetSchedule{
				Location: config.Location,
				Offset:   duration,
			}
		} else {
			schedule, err = cron.ParseStandard(job.Schedule)
			if err != nil {
				log.Printf("ERR: parse schedule: %s", err)
				continue
			}
		}

		// conversion formulas are defined by lifx LAN documentation
		// https://lan.developer.lifx.com/docs/representing-color-with-hsbk
		color := &lifxlan.Color{
			Hue:        uint16((job.Hue * 0x10000 / 360.0) % 0x10000),
			Saturation: uint16(job.Saturation * math.MaxUint16 / 100.0),
			Brightness: uint16(job.Brightness * math.MaxUint16 / 100.0),
			Kelvin:     uint16(job.Kelvin),
		}
		color.Sanitize()

		transition, err := time.ParseDuration(job.Transition)
		if err != nil {
			log.Printf("ERR: parse job transition: %s", err)
			continue
		}

		job := Job{
			Device:     devices[job.Device],
			Color:      color,
			Transition: transition,
		}
		lightCron.Schedule(schedule, job)
	}

	mux := http.NewServeMux()
	for label, device := range devices {
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

	lightCron.Start()
	log.Fatal(srv.ListenAndServe())
}
