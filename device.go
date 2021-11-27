package lamplighter

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

const (
	defaultPowerTransition = 2 * time.Second

	KelvinNeutral = 3000
)

// Device defines a desired state for a given lifx device
type Device struct {
	light.Device

	Name       string
	Brightness uint16
	Transition time.Duration
}

func (d *Device) Light() error {
	return d.setBrightness(d.Brightness, d.Transition)
}

func (d *Device) setBrightness(brightness uint16, transition time.Duration) error {
	conn, err := d.Dial()
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	color, err := d.GetColor(ctx, conn)
	if err != nil {
		return fmt.Errorf("%s: get color: %w", d.Name, err)
	}

	power, err := d.GetPower(ctx, conn)
	if err != nil {
		return fmt.Errorf("%s: get power: %w", d.Label(), err)
	}

	// if power is off, reset bulb brightness to 0 and turn on
	if power == lifxlan.PowerOff {
		color.Brightness = 0
		err = d.SetColor(ctx, conn, color, time.Millisecond, false)
		if err != nil {
			return fmt.Errorf("%s: reset color: %w", d.Label(), err)
		}

		err = d.SetPower(ctx, conn, lifxlan.PowerOn, false)
		if err != nil {
			return fmt.Errorf("%s: set power: %w", d.Label(), err)
		}
	}

	color.Brightness = brightness
	err = d.SetColor(ctx, conn, color, transition, false)
	if err != nil {
		return fmt.Errorf("%s: set color: %w", d.Label(), err)
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
		`{"hue": %.2f, "saturation": %.4, "brightness": %.2f, "kelvin": %d}`,
		float64(color.Hue)*360.0/0x10000,
		float64(color.Saturation)/math.MaxUint16*100,
		int(float64(color.Brightness)/math.MaxUint16*100),
		color.Kelvin,
	)

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

	err := d.setBrightness(brightness, transition)
	if err != nil {
		log.Printf("ERR: %s: %s", r.URL.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to set brightness on device"}`))
		return
	}

	fmt.Fprintf(w, `{"brightness": %s}`, r.FormValue("brightness"))
}
