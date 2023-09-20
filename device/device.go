package device

import (
	"fmt"
	"net/http"
	"time"
)

const (
	defaultLifxPort        = 56700
	defaultPowerTransition = 2 * time.Second
	defaultRetryBackoff    = 250 * time.Millisecond
	defaultRetryLimit      = 5
)

type Type string

const (
	TypeLifx Type = "lifx"
	TypeS31  Type = "s31"
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

func Connect(t Type, label, host, mac string) (Device, error) {
	switch t {
	case TypeLifx:
		addr := fmt.Sprintf("%s:%d", host, defaultLifxPort)
		return ConnectLifx(label, addr, mac)
	case TypeS31:
		return ConnectS31(label, host, mac)
	default:
		return nil, fmt.Errorf("unknown device type: %s", t)
	}
}
