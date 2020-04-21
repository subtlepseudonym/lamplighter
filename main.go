package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	tokenFile    = "lifx.token"
	locationFile = "home.loc"
)

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
	tomorrow := now.AddDate(0, 0, 1) // add one day
	sunset, err := getSunset(l.Location, tomorrow)
	if err != nil {
		log.Printf("get sunset: %s", err)
		return time.Time{}
	}

	log.Printf("next lamp: %s", sunset.Local().Format(time.RFC3339))
	return sunset
}

func main() {
	tokenBytes, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		log.Fatalf("read token file failed: %w", err)
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
