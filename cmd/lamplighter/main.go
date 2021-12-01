package main

import (
	"encoding/json"
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
	log.Printf(
		`{"device": %q, "hue": %.2f, "saturation": %.2f, "brightness": %.2f, "kelvin": %d, "transition": %q}`,
		j.Device.Label,
		float64(j.Color.Hue)*360.0/0x10000,
		float64(j.Color.Saturation)/math.MaxUint16*100,
		float64(j.Color.Brightness)/math.MaxUint16*100,
		j.Color.Kelvin,
		j.Transition,
	)

	err := j.Device.Transition(j.Color, j.Transition)
	if err != nil {
		log.Printf("ERR: transition device: %s", err)
	}
}

type Entry struct {
	Next       string  `json:"next"`
	Device     string  `json:"device"`
	Hue        float64 `json:"hue"`
	Saturation float64 `json:"saturation"`
	Brightness float64 `json:"brightness"`
	Kelvin     uint16  `json:"kelvin"`
	Transition string  `json:"transition"`
}

func entryHandler(lightCron *cron.Cron) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var entries []Entry
		for _, entry := range lightCron.Entries() {
			job, ok := entry.Job.(Job)
			if !ok {
				log.Printf("ERR: cast cron job to lamplighter job: %t", entry.Job)
				continue
			}

			entry := Entry{
				Next:       entry.Schedule.Next(time.Now()).Local().Format(time.RFC3339),
				Device:     job.Device.Label,
				Hue:        float64(job.Color.Hue) * 360.0 / 0x10000,
				Saturation: float64(job.Color.Saturation) / math.MaxUint16 * 100,
				Brightness: float64(job.Color.Brightness) / math.MaxUint16 * 100,
				Kelvin:     job.Color.Kelvin,
				Transition: job.Transition.String(),
			}
			entries = append(entries, entry)
		}

		err := json.NewEncoder(w).Encode(entries)
		if err != nil {
			log.Printf("ERR: write entries: %s", err)
		}
	})
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

	mux.HandleFunc("/entries", entryHandler(lightCron))

	srv := http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	log.Printf("listening on %s", srv.Addr)

	lightCron.Start()
	log.Fatal(srv.ListenAndServe())
}
