package encoders

import (
	"fmt"
	"strings"
)

type Encoder map[string]func(input string, settings map[string]string) error

func (e *Encoder) registerEncoder(name string, handler func(input string, settings map[string]string) error) {
	(*e)[name] = handler
}

var encoders = make(Encoder)

func init() {
	encoders.registerEncoder("jpg", HandleJPEG)
	encoders.registerEncoder("jpeg", HandleJPEG)
	encoders.registerEncoder("webp", HandleWEBP)
	encoders.registerEncoder("avif", HandleAVIF)
	for _, format := range genericFormats {
		encoders.registerEncoder(format, HandleGeneric(format))
	}
}

// Encode runs the registered encoder for typ and returns the expected output
// path. typ accepts common aliases such as "jpeg" and "jpg".
func Encode(input, typ string, settings map[string]string) (string, error) {
	normalized := normalizeType(typ)
	handler, ok := encoders[normalized]
	if !ok {
		return "", fmt.Errorf("unsupported encoder type %q", typ)
	}
	if settings == nil {
		settings = map[string]string{}
	}
	width, height := parseResolution(settings)
	outPath := buildOutputPath(input, normalized, width, height)
	if err := handler(input, settings); err != nil {
		return "", err
	}
	return outPath, nil
}

func normalizeType(typ string) string {
	typ = strings.ToLower(strings.TrimSpace(typ))
	switch typ {
	case "jpeg", "jpe":
		return "jpg"
	case "tif":
		return "tiff"
	}
	return typ
}

var genericFormats = []string{
	"png",
	"gif",
	"tiff",
	"bmp",
	"heic",
	"heif",
	"jp2",
	"jxl",
}

// SupportedTypes returns the configured output format names accepted by Encode.
func SupportedTypes() []string {
	types := []string{"jpg", "jpeg", "webp", "avif"}
	types = append(types, genericFormats...)
	return append([]string(nil), types...)
}
