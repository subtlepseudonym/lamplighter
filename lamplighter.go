package lamplighter

import (
	"log"
	"time"
)

const (
	retryLimit = 5
)

// Lamplighter is a set of lifx devices that should be turned on
// at a given offest from sunset each day over a given period of
// duration
type Lamplighter struct {
	Devices map[string]*Device

	location Location
	offset   time.Duration
	errCount uint
}

func New(location Location, offset time.Duration) Lamplighter {
	return Lamplighter{
		Devices:  make(map[string]*Device),
		location: location,
		offset:   offset,
	}
}

// Run turns on all devices over the Lamplighter's duration
//
// This implements robfig/go-cron.Job
func (l Lamplighter) Run() {
	for _, device := range l.Devices {
		go func(lamp Device) {
			err := lamp.Light()
			if err != nil {
				log.Printf("ERR: %s\n", err)
			}
		}(*device)
	}
}

// Next determines the next time the Lamplighter's devices should be
// turned on
//
// This implements robfig/go-cron.Schedule
func (l Lamplighter) Next(now time.Time) time.Time {
	var sunset time.Time
	var err error

	sunset, err = GetSunset(l.location, now)
	if err != nil {
		log.Printf("get sunset: %s", err)
		if l.errCount >= retryLimit {
			return time.Time{}
		}

		l.errCount++
		return time.Now().Add(time.Minute)
	}

	lightTime := sunset.Add(-1 * l.offset)
	if now.After(lightTime) || now.Equal(lightTime) {
		sunset, err = GetSunset(l.location, now.AddDate(0, 0, 1))
		if err != nil {
			log.Printf("get sunset: %s", err)
			if l.errCount >= retryLimit {
				return time.Time{}
			}

			l.errCount++
			return time.Now().Add(time.Minute)
		}
		lightTime = sunset.Add(-1 * l.offset)
	}

	l.errCount = 0
	log.Printf("next lamp: %s", lightTime.Local().Format(time.RFC3339))
	return lightTime
}
