package encoders

import (
	"reflect"
	"testing"
)

// Quick compile-time check: ensure the encoder package exposes handlers.
func TestEncoderSymbols(t *testing.T) {
	// We simply call the function variable zero-values to ensure they compile
	// and are present. We won't execute ImageMagick during tests.
	_ = HandleJPEG
}

func TestNormalizeType(t *testing.T) {
	tests := map[string]string{
		"jpeg":  "jpg",
		"JPE":   "jpg",
		" tif ": "tiff",
		"PNG":   "png",
	}
	for input, want := range tests {
		if got := normalizeType(input); got != want {
			t.Fatalf("normalizeType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSupportedTypes(t *testing.T) {
	got := SupportedTypes()
	want := []string{"jpg", "jpeg", "webp", "avif", "png", "gif", "tiff", "bmp", "heic", "heif", "jp2", "jxl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedTypes = %+v, want %+v", got, want)
	}
	for _, typ := range want {
		if _, ok := encoders[normalizeType(typ)]; !ok {
			t.Fatalf("type %s not registered", typ)
		}
	}
}

func TestBuildTempOutputPathPreservesExtension(t *testing.T) {
	got := buildTempOutputPath("/tmp/image_100_100.webp")
	if got != "/tmp/image_100_100.tmp.webp" {
		t.Fatalf("tmp path = %s", got)
	}
}
