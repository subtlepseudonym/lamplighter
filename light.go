package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	Results []Bulb `json:"results"`
}

// Bulb is an individual lifx bulb
type Bulb struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Label  string `json:"label"`
}

func (bulb Bulb) SetState(token []byte, stateReq StateRequest) error {
	b, err := json.Marshal(stateReq)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	buf := bytes.NewBuffer(b)

	url := fmt.Sprintf("https://api.lifx.com/v1/lights/id:%s/state", bulb.ID)
	req, err := http.NewRequest(http.MethodPut, url, buf)
	if err != nil {
		return fmt.Errorf("new request failed: %w", err)
	}
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", token))

	res, err := InsecureClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}

	switch res.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		return nil
	case http.StatusMultiStatus:
		var state StateResult
		err = json.NewDecoder(res.Body).Decode(&state)
		if err != nil {
			return fmt.Errorf("decode response body: %w", err)
		}

		var bad []string
		for _, bulb := range state.Results {
			if bulb.Status != "ok" {
				bad = append(bad, fmt.Sprintf("%s:%s", bulb.ID, bulb.Status))
			}
		}
		if len(bad) != 0 {
			return fmt.Errorf("bad bulb statuses: %s", bad)
		}

		return nil
	default:
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
		return fmt.Errorf("status: %s\nbody: %s", res.Status, b)
	}
}
