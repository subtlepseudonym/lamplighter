package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/subtlepseudonym/lamplighter"
)

type Config struct {
	Devices  map[string]Device    `json:"devices"`
	Jobs     []Job                `json:"jobs"`
	Location lamplighter.Location `json:"location"`
}

type Job struct {
	Schedule string `json:"schedule"`
	Device   string `json:"device"`

	Hue        int `json:"hue"`        // 0-360
	Saturation int `json:"saturation"` // 0-100
	Brightness int `json:"brightness"` // 0-100
	Kelvin     int `json:"kelvin"`     // 1500-9000

	Transition string `json:"transition"`
}

type Device struct {
	IP  string `json:"ip"`
	MAC string `json:"mac"`
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
