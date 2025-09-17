package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/subtlepseudonym/lamplighter"
)

type Device struct {
	Type   string            `json:"type"`
	Host   string            `json:"host"`
	MAC    string            `json:"mac"`
	Config map[string]string `json:"config,omitempty"`
}

type Config struct {
	Devices  map[string]Device    `json:"devices"`
	Jobs     []Job                `json:"jobs"`
	Location lamplighter.Location `json:"location"`
}

// Job defines when to run, on which device, what the desired final
// state is, and how long to take getting there.
//
// Color state is defined using Hue, Saturation, and Brightness. This
// is referred to as HSB (or HSL) color.
// https://en.wikipedia.org/wiki/HSL_and_HSV
type Job struct {
	Schedule string `json:"schedule"`
	Device   string `json:"device"`

	Hue        int `json:"hue"`        // 0-360
	Saturation int `json:"saturation"` // 0-100
	Brightness int `json:"brightness"` // 0-100
	Kelvin     int `json:"kelvin"`     // 1500-9000

	Transition string `json:"transition"`
}

func Open(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config Config
	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("decode config file: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
	for _, job := range c.Jobs {
		if _, ok := c.Devices[job.Device]; !ok {
			return fmt.Errorf("schedule references missing device %q", job.Device)
		}
	}

	return nil
}
