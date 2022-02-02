package lamplighter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

const (
	defaultPowerTransition = 2 * time.Second
	defaultRetryBackoff    = 250 * time.Millisecond
	defaultRetryLimit      = 5
)

type Device struct {
	light.Device
	Label string // prevent need to contact device for logging
}

// ConnectToDevice takes a label (for logging), a host in ip:port
// format and a mac address to locate a device on the network, connect
// to it, and retrieve the label and hardware version
func ConnectToDevice(label, host, mac string) (*Device, error) {
	target, err := lifxlan.ParseTarget(mac)
	if err != nil {
		return nil, fmt.Errorf("%s: parse mac address: %w", label, err)
	}

	dev := lifxlan.NewDevice(host, lifxlan.ServiceUDP, target)
	conn, err := dev.Dial()
	if err != nil {
		return nil, fmt.Errorf("%s: dial device: %w", label, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// This is lifxlan.Device.Echo, not lamplighter.Device.Echo
	//
	// The expectation is that ConnectToDevice is called once when
	// lamplighter starts and any failed connections will be caught by
	// the user at that time.
	err = dev.Echo(ctx, conn, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: echo device: %w", label, err)
	}

	bulb, err := light.Wrap(ctx, dev, false)
	if err != nil {
		return nil, fmt.Errorf("%s: device is not a light: %w", label, err)
	}

	device := &Device{
		Device: bulb,
	}

	err = device.GetHardwareVersion(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("%s: get hardware version: %w", label, err)
	}

	if device.Device.Label().String() != lifxlan.EmptyLabel {
		device.Label = strings.ToLower(device.Device.Label().String())
	}

	return device, nil
}

// Echo wraps the underlying method of the same name and adds retry logic
func (d *Device) Echo(ctx context.Context, conn net.Conn, payload []byte) error {
	var err error
	for i := 0; i < defaultRetryLimit; i++ {
		err = d.Device.Echo(ctx, conn, payload)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			break
		}

		// Retry backoff
		time.Sleep(time.Duration(i+1) * defaultRetryBackoff)
	}

	return err
}

func (d *Device) Transition(desired *lifxlan.Color, transition time.Duration) error {
	conn, err := d.Dial()
	if err != nil {
		return fmt.Errorf("%s: dial: %w", d.Label, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = d.Echo(ctx, conn, nil)
	if err != nil {
		return fmt.Errorf("%s: echo device: %w", d.Label, err)
	}

	if desired.Brightness == 0 {
		err = d.SetLightPower(ctx, conn, lifxlan.PowerOff, transition, true)
		if err != nil {
			return fmt.Errorf("%s: set light power: %w", d.Label, err)
		}
		return nil
	}

	power, err := d.GetPower(ctx, conn)
	if err != nil {
		return fmt.Errorf("%s: get power: %w", d.Label, err)
	}

	// if power is off, reset bulb brightness to 0 and turn on
	if power == lifxlan.PowerOff {
		color := *desired
		color.Brightness = 0

		err = d.SetColor(ctx, conn, &color, time.Millisecond, true)
		if err != nil {
			return fmt.Errorf("%s: reset color: %w", d.Label, err)
		}

		err = d.SetPower(ctx, conn, lifxlan.PowerOn, true)
		if err != nil {
			return fmt.Errorf("%s: set power: %w", d.Label, err)
		}
	}

	err = d.SetColor(ctx, conn, desired, transition, true)
	if err != nil {
		return fmt.Errorf("%s: set color: %w", d.Label, err)
	}

	return nil
}

func (d *Device) StatusHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := d.Dial()
	if err != nil {
		log.Printf("ERR: %s: dial: %s", d.Label, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to connect to device"}`))
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	color, err := d.GetColor(ctx, conn)
	if err != nil {
		log.Printf("ERR: %s: get color: %s", d.Label, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to get device brightness state"}`))
		return
	}

	fmt.Fprintf(
		w,
		`{"hue": %.2f, "saturation": %.2f, "brightness": %.2f, "kelvin": %d}`,
		float64(color.Hue)*360.0/0x10000,
		float64(color.Saturation)/math.MaxUint16*100,
		float64(color.Brightness)/math.MaxUint16*100,
		color.Kelvin,
	)
}

func (d *Device) PowerHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var hue uint16
	if _, ok := r.Form["hue"]; ok {
		param := r.FormValue("hue")
		p, err := strconv.ParseFloat(param, 64)
		if err != nil {
			log.Printf("ERR: %s: parse hue param %q: %s", d.Label, param, err)
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
			log.Printf("ERR: %s: parse saturation param %q: %s", d.Label, param, err)
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
			log.Printf("ERR: %s: parse brightness param %q: %s", d.Label, param, err)
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
			log.Printf("ERR: %s: parse kelvin param %q: %s", d.Label, param, err)
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
			log.Printf("ERR: %s: parse transition param %q: %s", d.Label, param, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "unable to parse transition parameter"}`))
			return
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
		log.Printf("ERR: transition: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "unable to set brightness on device"}`))
		return
	}

	fmt.Fprintf(
		w,
		`{"hue": %.2f, "saturation": %.2f, "brightness": %.2f, "kelvin": %d, "transition": %q}`,
		float64(color.Hue)*360.0/0x10000,
		float64(color.Saturation)/math.MaxUint16*100,
		float64(color.Brightness)/math.MaxUint16*100,
		color.Kelvin,
		transition,
	)
}
