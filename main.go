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
	lampID       = "d073d5106815"
	tokenFile    = "secrets/lifx.token"
	locationFile = "secrets/home.loc"
	offset       = time.Hour
	retryLimit   = 5
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
	errCount  uint
}

func (l Lamplighter) Run() {
	stateReq := SetStateRequest{
		Power: "on",
		Brightness: 1.0,
		Duration: 2.0,
	}

	// TODO: generalize this for multiple bulbs
	bulb := Bulb{
		ID: lampID,
	}

	err := bulb.SetState(l.LifxToken, stateReq)
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
		if l.errCount >= retryLimit {
			return time.Time{}
		}

		l.errCount++
		return time.Now().Add(time.Minute)
	}

	lightTime := sunset.Add(-1 * offset)
	if now.After(lightTime) || now.Equal(lightTime) {
		sunset, err = getSunset(l.Location, now.AddDate(0, 0, 1))
		if err != nil {
			log.Printf("get sunset: %s", err)
			if l.errCount >= retryLimit {
				return time.Time{}
			}

			l.errCount++
			return time.Now().Add(time.Minute)
		}
		lightTime = sunset.Add(-1 * offset)
	}

	l.errCount = 0
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

	now := time.Now()
	sunset, err := getSunset(lamp.Location, now)
	if err != nil {
		panic(err)
	}

	if now.After(sunset.Add(-1 * offset)) {
		lamp.Run()
	}

	cron := cron.New()
	cron.Schedule(lamp, lamp)
	cron.Run()
}
