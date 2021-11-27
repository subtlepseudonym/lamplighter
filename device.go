package lamplighter

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

const (
	defaultPowerTransition = 2 * time.Second
)

type Device struct {
	light.Device
}

func (d *Device) Transition(desired *lifxlan.Color, transition time.Duration) error {
	conn, err := d.Dial()
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if d.Label() == nil {
		err = d.GetLabel(ctx, conn)
		if err != nil {
			return fmt.Errorf("get device label: %w", err)
		}
	}
	label := strings.ToLower(d.Label().String())

	if desired.Brightness == 0 {
		err = d.SetLightPower(ctx, conn, lifxlan.PowerOff, transition, false)
		if err != nil {
			return fmt.Errorf("%s: set light power: %w", label, err)
		}
		return nil
	}

	power, err := d.GetPower(ctx, conn)
	if err != nil {
		return fmt.Errorf("%s: get power: %w", label, err)
	}

	// if power is off, reset bulb brightness to 0 and turn on
	if power == lifxlan.PowerOff {
		color := *desired
		color.Brightness = 0

		err = d.SetColor(ctx, conn, &color, time.Millisecond, false)
		if err != nil {
			return fmt.Errorf("%s: reset color: %w", label, err)
		}

		err = d.SetPower(ctx, conn, lifxlan.PowerOn, false)
		if err != nil {
			return fmt.Errorf("%s: set power: %w", label, err)
		}
	}

	err = d.SetColor(ctx, conn, desired, transition, false)
	if err != nil {
		return fmt.Errorf("%s: set color: %w", label, err)
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

	fmt.Fprintf(
		w,
		`{"hue": %.2f, "saturation": %.4f, "brightness": %.2f, "kelvin": %d}`,
		float64(color.Hue)*360.0/0x10000,
		float64(color.Saturation)/math.MaxUint16*100,
		float64(color.Brightness)/math.MaxUint16*100,
		color.Kelvin,
	)

	fmt.Fprintf(w, `{"brightness": %.2f}`, float64(color.Brightness)/float64(math.MaxUint16))
}

func (d *Device) PowerHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var hue uint16
	if _, ok := r.Form["hue"]; ok {
		param := r.FormValue("hue")
		p, err := strconv.ParseFloat(param, 64)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to parse hue parameter"}`))
			return
		}

		if p < 0 {
			p = 0
		} else if p > 360 {
			p = 360
		}

		hue = uint16(math.Floor((p / 360.0) * float64(math.MaxUint16)))
	}

	var saturation uint16
	if _, ok := r.Form["saturation"]; ok {
		param := r.FormValue("saturation")
		p, err := strconv.ParseFloat(param, 64)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to parse saturation parameter"}`))
			return
		}

		if p < 0 {
			p = 0
		} else if p > 100 {
			p = 100
		}

		saturation = uint16(math.Floor((p / 100.0) * float64(math.MaxUint16)))
	}

	var brightness uint16
	if _, ok := r.Form["brightness"]; ok {
		param := r.FormValue("brightness")
		p, err := strconv.ParseFloat(param, 64)
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

		brightness = uint16(math.Floor((p / 100.0) * float64(math.MaxUint16)))
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "brightness parameter is required"}`))
	}

	var kelvin uint16
	if _, ok := r.Form["kelvin"]; ok {
		param := r.FormValue("kelvin")
		p, err := strconv.Atoi(param)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to parse kelvin parameter"}`))
			return
		}

		if p < 1500 {
			p = 1500
		} else if p > 9000 {
			p = 9000
		}

		kelvin = uint16(p)
	}

	transition := defaultPowerTransition
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

	color := &lifxlan.Color{
		Hue:        hue,
		Saturation: saturation,
		Brightness: brightness,
		Kelvin:     kelvin,
	}

	err := d.Transition(color, transition)
	if err != nil {
		log.Printf("ERR: %s: %s", r.URL.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to set brightness on device"}`))
		return
	}

	fmt.Fprintf(
		w,
		`{"hue": %.2f, "saturation": %.4f, "brightness": %.2f, "kelvin": %d, "transition": %q}`,
		float64(color.Hue)*360.0/0x10000,
		float64(color.Saturation)/math.MaxUint16*100,
		float64(color.Brightness)/math.MaxUint16*100,
		color.Kelvin,
		transition,
	)
}
