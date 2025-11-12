package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestConversionJobs_ToJobs builds an InputToken via JSON marshalling of
// structs (to simulate receiving a JSON token), then converts the
// ConversionJobs into a flat []Job and asserts expected fields.
func TestConversionJobs_ToJobs(t *testing.T) {
	// Build a sample token struct
	token := InputToken{
		CallbackURL:  "https://example.local/callback",
		Backends:     map[string]string{"b1": "local"},
		Transformers: map[string]string{"t1": "identity"},
		Resolutions: map[string]Resolution{
			"thumb": {Width: 100, Height: 80},
			"large": {Width: 1600, Height: 1200},
		},
		ConversionJobs: []ConversionJob{
			{
				Type:                "jpeg",
				Resolutions:         []string{"thumb", "large"},
				Transformers:        []string{"t1"},
				DestinationBackends: []string{"b1"},
				Settings:            map[string]string{"quality": "80"},
			},
		},
	}

	// Marshal -> Unmarshal to simulate JSON input (ensures tags work)
	b, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal sample token: %v", err)
	}
	var parsed InputToken
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal sample token: %v", err)
	}

	cjs := ConversionJobs(parsed.ConversionJobs)
	jobs := cjs.ToJobs(parsed.Resolutions)

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// job 0 should be thumb, job 1 large (order preserved)
	wantResThumb := parsed.Resolutions["thumb"]
	if jobs[0].Resolution != wantResThumb {
		t.Fatalf("job0 resolution: want %+v got %+v", wantResThumb, jobs[0].Resolution)
	}
	wantResLarge := parsed.Resolutions["large"]
	if jobs[1].Resolution != wantResLarge {
		t.Fatalf("job1 resolution: want %+v got %+v", wantResLarge, jobs[1].Resolution)
	}

	// common checks
	for i, j := range jobs {
		if j.Type != "jpeg" {
			t.Fatalf("job %d: unexpected type %s", i, j.Type)
		}
		if j.Status != "pending" {
			t.Fatalf("job %d: expected pending status, got %s", i, j.Status)
		}
		if !reflect.DeepEqual(j.Settings, map[string]string{"quality": "80"}) {
			t.Fatalf("job %d: settings mismatch: %+v", i, j.Settings)
		}
		if j.TransformerID != "t1" {
			t.Fatalf("job %d: expected transformer t1, got %s", i, j.TransformerID)
		}
		if !reflect.DeepEqual(j.DestinationBackendIDs, []string{"b1"}) {
			t.Fatalf("job %d: destination backends mismatch: %+v", i, j.DestinationBackendIDs)
		}
		if j.ID == "" {
			t.Fatalf("job %d: expected non-empty ID", i)
		}
	}
}
