package encoders

import (
	"testing"
)

// Quick compile-time check: ensure the encoder package exposes handlers.
func TestEncoderSymbols(t *testing.T) {
	// We simply call the function variable zero-values to ensure they compile
	// and are present. We won't execute ImageMagick during tests.
	_ = HandleJPEG
}
