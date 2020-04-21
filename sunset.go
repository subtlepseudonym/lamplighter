package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	solarURL        = "https://api.sunrise-sunset.org/json"
	solarTimeFormat = "3:04:05 PM"
	dateString      = "2006-01-02"
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

func getSunset(location Location, date time.Time) (time.Time, error) {
	url := fmt.Sprintf("%s?lat=%f&lng=%f&date=%s&formatted=0", solarURL, location.Latitude, location.Longitude, date.Format(dateString))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("new request: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
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
