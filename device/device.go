package device

import (
	"net/http"
	"time"
)

const (
	defaultPowerTransition = 2 * time.Second
	defaultRetryBackoff    = 250 * time.Millisecond
	defaultRetryLimit      = 5
)

type Color struct {
	Hue        uint16
	Saturation uint16
	Brightness uint16
	Kelvin     uint16
}

type Device interface {
	StatusHandler(http.ResponseWriter, *http.Request)
	PowerHandler(http.ResponseWriter, *http.Request)
	Transition(*Color, time.Duration) error
	Label() string
	String() string
}
