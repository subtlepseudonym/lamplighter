package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

type Device struct {
	light.Device
}

func (d *Device) SetBrightness(brightness uint16, transition time.Duration) error {
	conn, err := d.Dial()
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	color := &lifxlan.Color{
		Hue:        0, // technically green
		Saturation: 0,
		Brightness: brightness,
		Kelvin:     KelvinNeutral,
	}

	err = d.SetColor(ctx, conn, color, transition, false)
	if err != nil {
		return fmt.Errorf("set color: %w", err)
	}

	return nil
}

func (d *Device) StatusHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := d.Dial()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to connect to device"}`))
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	color, err := d.GetColor(ctx, conn)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to get device brightness state"}`))
		return
	}

	fmt.Fprintf(w, `{"brightness": %.2f}`, float64(color.Brightness)/float64(math.MaxUint16))
}

func (d *Device) PowerHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var brightness uint16
	if _, ok := r.Form["brightness"]; ok {
		param := r.FormValue("brightness")
		p, err := strconv.Atoi(param)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to parse brightness parameter"}`))
			return
		}

		if p < 0 {
			p = 0
		} else if p > 100 {
			p = 100
		}

		brightness = uint16(math.Floor((float64(p) / 100.0) * float64(math.MaxUint16)))
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "brightness parameter is required"}`))
	}

	transition := 2 * time.Second
	if _, ok := r.Form["transition"]; ok {
		param := r.FormValue("transition")
		_, err := strconv.Atoi(param)
		if err == nil && param != "" {
			param = param + "ms"
		}

		parsed, err := time.ParseDuration(param)
		if err != nil {
			log.Printf("ERR: parse transition param %q: %s", param, err)
		}
		transition = parsed
	}

	err := d.SetBrightness(brightness, transition)
	if err != nil {
		log.Printf("ERR: %s: %s", r.URL.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to set brightness on device"}`))
		return
	}

	fmt.Fprintf(w, `{"brightness": %s}`, r.FormValue("brightness"))
}
