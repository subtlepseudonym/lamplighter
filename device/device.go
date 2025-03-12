package device

import (
	"fmt"
	"net/http"
	"time"

	"github.com/subtlepseudonym/lamplighter/config"
)

const (
	defaultLifxPort        = 56700
	defaultPowerTransition = 2 * time.Second
	defaultRetryBackoff    = 250 * time.Millisecond
	defaultRetryLimit      = 5
)

type Type string

const (
	TypeLifx   Type = "lifx"
	TypeS31    Type = "s31"
	TypeShelly Type = "shelly"
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

func Connect(label string, device config.Device) (Device, error) {
	switch Type(device.Type) {
	case TypeLifx:
		addr := fmt.Sprintf("%s:%d", device.Host, defaultLifxPort)
		return ConnectLifx(label, addr, device.MAC)
	case TypeS31:
		return ConnectS31(label, device.Host, device.MAC)
	case TypeShelly:
		return ConnectShelly(label, device.Host, device.MAC, device.Index)
	default:
		return nil, fmt.Errorf("unknown device type: %s", device.Type)
	}
}
