package device

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

type S31 struct {
	Address  string
	MAC      string
	Firmware string
	Hardware string
	label    string
}

type TasmotaFirmwareStatus struct {
	Status struct {
		Version  string `json:"Version"`
		Hardware string `json:"Hardware"`
	} `json:"StatusFWR"`
}

type TasmotaPowerState struct {
	Power string `json:"POWER"`
}

func ConnectS31(label, addr, mac string) (Device, error) {
	s31 := &S31{
		Address: addr,
		MAC:     mac,
		label:   label,
	}

	query := fmt.Sprintf("http://%s/cm?cmnd=Status%202", s31.Address)
	res, err := http.Get(query)
	if err != nil {
		return nil, fmt.Errorf("%s: query status: %w", s31.label, err)
	}

	var status TasmotaFirmwareStatus
	err = json.NewDecoder(res.Body).Decode(&status)
	if err != nil {
		return nil, fmt.Errorf("%s: decode status: %w", s31.label, err)
	}

	s31.Firmware = status.Status.Version
	s31.Hardware = status.Status.Hardware

	return s31, nil
}

func (s *S31) Transition(color *Color, transition time.Duration) error {
	power := "Off"
	if color.Brightness > 0 {
		power = "On"
	}

	query := fmt.Sprintf("http://%s/cm?cmnd=Power%20%s", s.Address, power)
	res, err := http.Get(query)
	if err != nil {
		return fmt.Errorf("%s: set power state: %w", s.label, err)
	}

	var state TasmotaPowerState
	err = json.NewDecoder(res.Body).Decode(&state)
	if err != nil {
		return fmt.Errorf("%s: decode power state: %w", s.label, err)
	}

	return nil
}

func (s *S31) StatusHandler(w http.ResponseWriter, r *http.Request) {
	query := fmt.Sprintf("http://%s/cm?cmnd=State", s.Address)
	res, err := http.Get(query)
	if err != nil {
		log.Printf("ERR: %s: query state: %w", s.label, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to connect to device"}`))
		return
	}

	var state TasmotaPowerState
	err = json.NewDecoder(res.Body).Decode(&state)
	if err != nil {
		log.Printf("ERR: %s: decode state: %w", s.label, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to decode device state"}`))
		return
	}

	fmt.Fprintf(
		w,
		`{"power": %q}`,
		state.Power,
	)
}

func (s *S31) PowerHandler(w http.ResponseWriter, r *http.Request) {
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
		`{"brightness": %q}`,
		float64(color.Brightness)/math.MaxUint16*100,
	)
}

func (s *S31) Label() string {
	return s.label
}

func (s *S31) String() string {
	return fmt.Sprintf("Sonoff S31 %s", s.Label)
}
