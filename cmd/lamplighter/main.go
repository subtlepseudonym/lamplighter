package main

import (
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
	log.Printf("%s: transitioning over %s", j.Device.Label, j.Transition)
	err := j.Device.Transition(j.Color, j.Transition)
	if err != nil {
		log.Printf("ERR: transition device: %s", err)
	}
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
		host := fmt.Sprintf("%s:%d", dev.IP, lifxPort)
		device, err := lamplighter.ConnectToDevice(label, host, dev.MAC)
		if err != nil {
			log.Printf("ERR: scan device: %s", err)
			continue
		}
		devices[label] = device
		log.Printf("registered device: %q %s", label, device.HardwareVersion())
	}

	now := time.Now() // used for logging cron entries
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

		log.Printf("job: %s: %s", schedule.Next(now).Local().Format(time.RFC3339), job.Device.Label)
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
