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

type Shelly struct {
	Address  string
	MAC      string
	Firmware string
	Hardware string
	label    string
	index    int // index of attached port on device
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

type ShellyKVSResponse struct {
	ETag  string `json:"etag"`
	Value string `json:"value"`
}

func (r *ShellyKVSResponse) UnmarshalJSON(b []byte) error {
	kvs := struct {
		ETag  string `json:"etag"`
		Value string `json:"value"`
	}{}

	err := json.Unmarshal(b, &kvs)
	if err != nil {
		return err
	}

	unescaped, err := url.QueryUnescape(kvs.Value)
	if err != nil {
		return fmt.Errorf("unescape value: %w", err)
	}

	r.ETag = kvs.ETag
	r.Value = unescaped
	return nil
}

type ShellySwitchStatusResponse struct {
	Source      string `json:"source"`
	Output      bool   `json:"output"`
	Temperature struct {
		Celsius   float64 `json:"tC"`
		Farenheit float64 `json:"tF"`
	} `json:"temperature"`
}

func ConnectShelly(label, addr, mac string, index int) (Device, error) {
	shelly := &Shelly{
		Address: addr,
		MAC:     mac,
		label:   label,
		index:   index,
	}

	query := fmt.Sprintf("http://%s/rpc/Sys.GetConfig?id=0", shelly.Address)
	res, err := http.Get(query)
	if err != nil {
		return nil, fmt.Errorf("%s: query config: %w", shelly.label, err)
	}

	var config ShellyConfigResponse
	err = json.NewDecoder(res.Body).Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("%s: decode config: %w", shelly.label, err)
	}
	shelly.Firmware = config.Device.Firmware

	query = fmt.Sprintf("http://%s/rpc/KVS.Get?key=model", shelly.Address)
	res, err = http.Get(query)
	if err != nil {
		return nil, fmt.Errorf("%s: query model: %w", shelly.label, err)
	}

	var kvs ShellyKVSResponse
	err = json.NewDecoder(res.Body).Decode(&kvs)
	if err != nil {
		return nil, fmt.Errorf("%s: decode model: %w", shelly.label, err)
	}
	shelly.Hardware = kvs.Value

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
