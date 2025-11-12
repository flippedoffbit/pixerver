package uuidv7

import (
	"strings"
	"testing"
)

// Quick sanity test ensuring New returns a well-formed UUID-like string.
func TestNewFormat(t *testing.T) {
	id := New()
	if id == "" {
		t.Fatalf("empty id")
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("unexpected uuid format: %s", id)
	}
}
