package models

import (
	"testing"
)

func TestInputToken_ValidateErrors(t *testing.T) {
	var tkn *InputToken
	if err := tkn.Validate(); err == nil {
		t.Fatalf("expected error for nil token")
	}

	tkn2 := &InputToken{}
	if err := tkn2.Validate(); err == nil {
		t.Fatalf("expected error for empty token")
	}

	tkn3 := &InputToken{CallbackURL: "not-a-url", Backends: map[string]string{"b": "v"}, ConversionJobs: []ConversionJob{{}}}
	if err := tkn3.Validate(); err == nil {
		t.Fatalf("expected error for invalid callback url")
	}

	tkn4 := &InputToken{CallbackURL: "https://example.local/callback", ConversionJobs: []ConversionJob{{}}}
	if err := tkn4.Validate(); err == nil {
		t.Fatalf("expected error for missing backends")
	}

	tkn5 := &InputToken{CallbackURL: "https://example.local/callback", Backends: map[string]string{"b": "v"}}
	if err := tkn5.Validate(); err == nil {
		t.Fatalf("expected error for missing conversion jobs")
	}
}

func TestInputToken_GetResolutionAndBackend(t *testing.T) {
	tkn := &InputToken{
		Resolutions: map[string]Resolution{"r1": {Width: 10, Height: 20}},
		Backends:    map[string]string{"b1": "local"},
	}
	if r, ok := tkn.GetResolution("r1"); !ok || r.Width != 10 || r.Height != 20 {
		t.Fatalf("GetResolution failed: %+v %v", r, ok)
	}
	if _, ok := tkn.GetResolution("missing"); ok {
		t.Fatalf("expected missing resolution to be false")
	}
	if b, ok := tkn.GetBackend("b1"); !ok || b != "local" {
		t.Fatalf("GetBackend failed: %s %v", b, ok)
	}
	if _, ok := tkn.GetBackend("missing"); ok {
		t.Fatalf("expected missing backend to be false")
	}
}
