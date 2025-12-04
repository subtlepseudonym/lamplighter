package device

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type ShellyDeviceConfig struct {
	Index int
}

func parseDeviceConfig(config map[string]interface{}) (*ShellyDeviceConfig, error) {
	val, ok := config["index"]
	if !ok {
		return &ShellyDeviceConfig{}, nil
	}

	var index int
	switch idx := val.(type) {
	case int:
		index = idx
	case float64:
		index = int(idx)
	default:
		return nil, fmt.Errorf("parse index value: %v (%T)", val, val)
	}

	return &ShellyDeviceConfig{
		Index: int(index),
	}, nil
}

type Shelly struct {
	Address  string
	MAC      string
	Firmware string
	Hardware string
	label    string
	index    int // index of attached port on device
}

type ShellyDeviceInfo struct {
	ID         string `json:"id"`
	MAC        string `json:"mac"`
	Model      string `json:"model"`
	Generation int    `json:"gen"`
	FirmwareID string `json:"fw_id"`
	Version    string `json:"ver"`
	App        string `json:"app"`
	Profile    string `json:"profile"`
}

type ShellyConfigResponse struct {
	Device struct {
		Name         string `json:"name"`
		MAC          string `json:"mac"`
		Firmware     string `json:"fw_id"`
		Discoverable bool   `json:"discoverable"`
		EcoMode      bool   `json:"eco_mode"`
	} `json:"device"`
	Location struct {
		Timezone  string  `json:"tz"`
		Latitude  float64 `json:"lat"`
		Longitude float64 `json:"lon"`
	} `json:"location"`
}

type ShellySwitchStatusResponse struct {
	Source      string `json:"source"`
	Output      bool   `json:"output"`
	Temperature struct {
		Celsius   float64 `json:"tC"`
		Farenheit float64 `json:"tF"`
	} `json:"temperature"`
}

func ConnectShelly(label, addr, mac string, deviceConfig map[string]interface{}) (Device, error) {
	cfg, err := parseDeviceConfig(deviceConfig)
	if err != nil {
		return nil, fmt.Errorf("%s: parse device config: %w", label, err)
	}

	shelly := &Shelly{
		Address: addr,
		MAC:     mac,
		label:   label,
		index:   cfg.Index,
	}



	query := fmt.Sprintf("http://%s/rpc/Shelly.GetDeviceInfo", shelly.Address)
	res, err := http.Get(query)
	if err != nil {
		return nil, fmt.Errorf("%s: query info: %w", shelly.label, err)
	}

	var info ShellyDeviceInfo
	err = json.NewDecoder(res.Body).Decode(&info)
	if err != nil {
		return nil, fmt.Errorf("%s: decode info: %w", shelly.label, err)
	}
	shelly.Firmware = info.FirmwareID
	shelly.Hardware = info.App

	return shelly, nil
}

func (s *Shelly) Transition(color *Color, transition time.Duration) error {
	on := false
	if color.Brightness > 0 {
		on = true
	}

	query := fmt.Sprintf("http://%s/rpc/Switch.Set?id=%d&on=%t", s.Address, s.index, on)
	_, err := http.Get(query)
	if err != nil {
		return fmt.Errorf("%s: set power state: %w", s.label, err)
	}

	return nil
}

func (s *Shelly) StatusHandler(w http.ResponseWriter, r *http.Request) {
	query := fmt.Sprintf("http://%s/rpc/Switch.GetStatus?id=0", s.Address)
	res, err := http.Get(query)
	if err != nil {
		log.Printf("ERR: %s: query status: %w", s.label, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to connect to device"}`))
		return
	}

	var status ShellySwitchStatusResponse
	err = json.NewDecoder(res.Body).Decode(&status)
	if err != nil {
		log.Printf("ERR: %s: decode status: %w", s.label, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to decode device status"}`))
		return
	}

	fmt.Fprintf(
		w,
		`{"output": %t}`,
		status.Output,
	)
}

func (s *Shelly) PowerHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var brightness uint16
	if _, ok := r.Form["brightness"]; ok {
		param := r.FormValue("brightness")
		p, err := strconv.ParseFloat(param, 64)
		if err != nil {
			log.Printf("ERR: %s: parse brightness param %q: %s", s.label, param, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to parse brightness parameter"}`))
			return
		}

		// FIXME: this logic is unnecessary for relays (which are boolean)
		if p < 0 {
			p = 0
		} else if p > 100 {
			p = 100
		}

		brightness = uint16(math.Floor((p / 100.0) * float64(math.MaxUint16)))
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "brightness parameter is required"}`))
	}

	color := &Color{
		Brightness: brightness,
	}
	err := s.Transition(color, time.Second)
	if err != nil {
		log.Printf("ERR: transition: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to toggle device power"}`))
		return
	}

	fmt.Fprintf(
		w,
		`{"brightness": %.2f}`,
		float64(color.Brightness)/math.MaxUint16*100,
	)
}

func (s *Shelly) Label() string {
	return s.label
}

func (s *Shelly) String() string {
	return fmt.Sprintf("Shelly %s %s", s.Hardware, s.Firmware)
}
