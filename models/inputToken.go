package models

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
)

// ConversionJob describes a single conversion to perform.
type ConversionJob struct {
	Type                string   `json:"type"`
	Resolutions         []string `json:"resolutions"`
	Transformers        []string `json:"transformers"`
	DestinationBackends []string `json:"destinationBackends"`
	KeepOriginal        bool     `json:"keepOriginal"`
	// Settings are intentionally a map[string]any to allow numeric or string values
	// (quality: 80, effort: 6, etc.). Caller should assert types as needed.
	Settings map[string]string `json:"settings"`
}

// Resolution represents an image size.
type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// InputToken is the top-level structure of baseToken.json supplied to the service.
type InputToken struct {
	CallbackURL    string                `json:"callbackUrl"`
	Backends       map[string]string     `json:"backends"`
	Transformers   map[string]string     `json:"transformers"`
	Resolutions    map[string]Resolution `json:"resolutions"`
	ConversionJobs []ConversionJob       `json:"conversionJobs"`
}

// LoadInputTokenFromFile parses the JSON file at path into an InputToken.
func LoadInputTokenFromFile(path string) (*InputToken, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t InputToken
	if err := json.Unmarshal(f, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Validate performs basic sanity checks on the token.
func (t *InputToken) Validate() error {
	if t == nil {
		return errors.New("nil token")
	}
	if t.CallbackURL == "" {
		return errors.New("callbackUrl is required")
	}
	if _, err := url.ParseRequestURI(t.CallbackURL); err != nil {
		return err
	}
	if len(t.Backends) == 0 {
		return errors.New("at least one backend is required")
	}
	if len(t.ConversionJobs) == 0 {
		return errors.New("at least one conversion job is required")
	}
	return nil
}

// GetResolution looks up a named resolution.
func (t *InputToken) GetResolution(name string) (Resolution, bool) {
	if t == nil || t.Resolutions == nil {
		return Resolution{}, false
	}
	r, ok := t.Resolutions[name]
	return r, ok
}

// GetBackend returns the backend key for the provided name.
func (t *InputToken) GetBackend(name string) (string, bool) {
	if t == nil || t.Backends == nil {
		return "", false
	}
	v, ok := t.Backends[name]
	return v, ok
}
