package main

import (
	"log"
	"math"
	"time"
)

type Lamplighter struct {
	Devices map[string]*Device

	location   Location
	transition time.Duration
	offset     time.Duration
	errCount   uint
}

func New(location Location, transition, offset time.Duration) Lamplighter {
	return Lamplighter{
		Devices:    make(map[string]*Device),
		location:   location,
		transition: transition,
		offset:     offset,
	}
}

func (l Lamplighter) Run() {
	for _, device := range l.Devices {
		go func() {
			err := device.SetBrightness(math.MaxUint16, l.transition)
			if err != nil {
				log.Printf("ERR: %s\n", err)
			}
		}()
	}
}

func (l Lamplighter) Next(now time.Time) time.Time {
	var sunset time.Time
	var err error

	sunset, err = getSunset(l.location, now)
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
		sunset, err = getSunset(l.location, now.AddDate(0, 0, 1))
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
