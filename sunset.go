package lamplighter

import (
	"log"
	"time"

	"github.com/nathan-osman/go-sunrise"
)

type SunsetSchedule struct {
	Location Location      `json:"location"`
	Offset   time.Duration `json:"offset"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Next returns the time of next sunset, given the SunsetSchedule's
// location value
//
// This implements robfic/cron.Schedule
func (s SunsetSchedule) Next(now time.Time) time.Time {
	_, sunset := sunrise.SunriseSunset(
		s.Location.Latitude,
		s.Location.Longitude,
		now.Year(),
		now.Month(),
		now.Day(),
	)
	lightTime := sunset.Add(s.Offset)

	if now.After(lightTime) || now.Equal(lightTime) {
		_, sunset := sunrise.SunriseSunset(
			s.Location.Latitude,
			s.Location.Longitude,
			now.Year(),
			now.Month(),
			now.Day(),
		)
		lightTime = sunset.Add(s.Offset)
	}

	log.Printf("next sunset %s: %s", s.Offset, lightTime.Local().Format(time.RFC3339))
	return lightTime
}
