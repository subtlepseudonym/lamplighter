package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	Location Location
}

func (l Lamplighter) Run() {
	err := lamp()
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

	return sunset
}

func lamp() error {
	b, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return fmt.Errorf("read token file failed: %w", err)
	}
	token := bytes.TrimRight(b, "\n")

	err = lightLamp(token)
	if err != nil {
		return fmt.Errorf("light lamp failed: %w", err)
	}

	return nil
}

func main() {
	b, err := ioutil.ReadFile(locationFile)
	if err != nil {
		log.Fatalf("read location file failed: %s", err)
	}

	var location Location
	err = json.Unmarshal(b, &location)
	if err != nil {
		log.Printf("unmarshal location: %s", err)
	}

	lamp := Lamplighter{
		Location: location,
	}

	cron := cron.New()
	cron.Schedule(lamp, lamp)

	cron.Run()
}
