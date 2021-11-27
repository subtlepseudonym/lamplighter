package lamplighter

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

var insecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

const (
	solarURL        = "https://api.sunrise-sunset.org/json"
	solarTimeFormat = "3:04:05 PM"
	dateString      = "2006-01-02"
	retryLimit      = 3
)

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type SolarResponse struct {
	Results SolarData `json:"results"`
	Status  string    `json:"status"`
}

// SolarData is the sunset data for a given location
type SolarData struct {
	Sunset time.Time `json:"sunset"`
}

type SunsetSchedule struct {
	Location Location      `json:"location"`
	Offset   time.Duration `json:"offset"`

	errCount int `json:"-"`
}

// Next returns the time of next sunset, given the SunsetSchedule's
// location value
//
// This implements robfic/cron.Schedule
func (s SunsetSchedule) Next(now time.Time) time.Time {
	sunset, err := GetSunset(s.Location, now)
	if err != nil {
		log.Printf("get sunset: %s", err)
		if s.errCount >= retryLimit {
			return time.Time{}
		}

		s.errCount++
		return time.Now().Add(time.Minute)
	}

	lightTime := sunset.Add(s.Offset)
	if now.After(lightTime) || now.Equal(lightTime) {
		sunset, err = GetSunset(s.Location, now.AddDate(0, 0, 1))
		if err != nil {
			log.Printf("get sunset: %s", err)
			if s.errCount >= retryLimit {
				return time.Time{}
			}

			s.errCount++
			return time.Now().Add(time.Minute)
		}
		lightTime = sunset.Add(s.Offset)
	}

	s.errCount = 0
	log.Printf("next sunset %s: %s", s.Offset, lightTime.Local().Format(time.RFC3339))
	return lightTime
}

func GetSunset(location Location, date time.Time) (time.Time, error) {
	url := fmt.Sprintf("%s?lat=%f&lng=%f&date=%s&formatted=0", solarURL, location.Latitude, location.Longitude, date.Format(dateString))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("new request: %w", err)
	}

	res, err := insecureClient.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("sunset request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("response: %s", res.Status)
	}

	var solarResponse SolarResponse
	err = json.NewDecoder(res.Body).Decode(&solarResponse)
	if err != nil {
		return time.Time{}, fmt.Errorf("decode response: %w", err)
	}

	return solarResponse.Results.Sunset, nil
}
