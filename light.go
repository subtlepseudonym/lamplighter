package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	lampID = "d073d5106815"
)

// StateRequest is the format for a JSON body
// that is sent to Lifx to determine the desired
// state of the bulbs specified by the request path
// selector
type StateRequest struct {
	Power      string  `json:"power"`
	Color      string  `json:"color"`
	Brightness float64 `json:"brightness"` // from 0.0 to 1.0
	Duration   float64 `json:"duration"`   // state transition time in seconds
	Fast       bool    `json:"fast"`       // request w/o state checks or waiting for response
}

// StateResult is the response from the Lifx API
// specifying the state of the bulb specified by
// the preceding request
type StateResult struct {
	Results []BulbState `json:"results"`
}

// BulbState is the state of an individual bulb
// after a set-state request
type BulbState struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
}

func lightLamp(token []byte) error {
	state := StateRequest{
		Power:      "on",
		Brightness: 1.0,
		Duration:   2.0,
	}

	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	buf := bytes.NewBuffer(b)

	url := fmt.Sprintf("https://api.lifx.com/v1/lights/id:%s/state", lampID)
	req, err := http.NewRequest(http.MethodPut, url, buf)
	if err != nil {
		return fmt.Errorf("new request failed: %w", err)
	}
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", token))

	res, err := InsecureClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}

	if res.StatusCode == http.StatusOK {
		return nil
	}

	fmt.Println("status: ", res.Status)
	resBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	fmt.Println(string(resBytes))
	return nil
}
