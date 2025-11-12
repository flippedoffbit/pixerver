package models

import (
	"errors"
	"net/url"
)

// ConversionJob describes a single conversion to perform.
type ConversionJob struct {
	Type                string   `json:"type"`
	Resolutions         []string `json:"resolutions"`
	Transformers        []string `json:"transformers"`
	DestinationBackends []string `json:"destinationBackends"`
	KeepOriginal        bool     `json:"keepOriginal"`
	// Settings is a map[string]string with values encoded as strings to match
	// the expected Go types. Numeric values in JSON should be quoted so they
	// unmarshal as strings (e.g. "quality": "80").
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
