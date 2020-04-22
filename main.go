package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	tokenFile    = "secrets/lifx.token"
	locationFile = "secrets/home.loc"
)

var InsecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

type Lamplighter struct {
	LifxToken []byte
	Location  Location
}

func (l Lamplighter) Run() {
	err := lightLamp(l.LifxToken)
	if err != nil {
		log.Printf("ERR: %s\n", err)
	}
}

func (l Lamplighter) Next(now time.Time) time.Time {
	var sunset time.Time
	var err error

	sunset, err = getSunset(l.Location, now)
	if err != nil {
		log.Printf("get sunset: %s", err)
		return time.Time{}
	}

	if now.After(sunset) {
		sunset, err = getSunset(l.Location, now.AddDate(0, 0, 1))
		if err != nil {
			log.Printf("get sunset: %s", err)
			return time.Time{}
		}
	}

	lightTime := sunset.Add(-20 * time.Minute)
	log.Printf("next lamp: %s", lightTime.Local().Format(time.RFC3339))
	return lightTime
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

	tokenBytes, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		log.Fatalf("read token file failed: %s", err)
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

	lamp := Lamplighter{
		LifxToken: bytes.TrimRight(tokenBytes, "\n"),
		Location:  location,
	}

	cron := cron.New()
	cron.Schedule(lamp, lamp)
	cron.Run()
}
