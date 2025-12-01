package encoders

import (
	"fmt"
	"strconv"
)

// HandleWEBP creates a WebP variant from input file. It writes a new file
// named <base>_<width>_<height>.webp (or _orig when width/height are zero).
// Supported settings:
//   - quality: integer 0-100
//   - lossless: "true"/"false"
//   - method: integer (encoder method/effort)
func HandleWEBP(name string, settings map[string]string) error {
	quality := 80
	if q, ok := settings["quality"]; ok {
		if v, err := strconv.Atoi(q); err == nil {
			quality = v
		}
	}

	lossless := false
	if l, ok := settings["lossless"]; ok && (l == "true" || l == "1") {
		lossless = true
	}

	method := -1
	if m, ok := settings["method"]; ok {
		if v, err := strconv.Atoi(m); err == nil {
			method = v
		}
	}

	// find binary
	bin, err := findImageMagickBinary()
	if err != nil {
		return err
	}

	// prepare output name
	width, height := parseResolution(settings)
	outName := buildOutputPath(name, "webp", width, height)
	tmp := outName + ".tmp"

	var args []string
	args = append(args, name)
	if lossless {
		args = append(args, "-define", "webp:lossless=true")
	}
	args = append(args, "-quality", strconv.Itoa(quality))
	if method >= 0 {
		args = append(args, "-define", fmt.Sprintf("webp:method=%d", method))
	}
	if width != 0 || height != 0 {
		args = append(args, "-resize", fmt.Sprintf("%dx%d", width, height))
	}
	args = append(args, tmp)

	return execImageMagick(bin, args, tmp, outName, name, "webp")
}
