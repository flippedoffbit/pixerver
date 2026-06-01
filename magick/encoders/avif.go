package encoders

import (
	"fmt"
	"strconv"
)

// HandleAVIF creates an AVIF variant from input file. It writes a new file
// named <base>_<width>_<height>.avif (or _orig when width/height are zero).
// Supported settings:
//   - quality: integer 0-100
//   - effort: integer (encoder effort/speed)
func HandleAVIF(name string, settings map[string]string) error {
	quality := 50
	if q, ok := settings["quality"]; ok {
		if v, err := strconv.Atoi(q); err == nil {
			quality = v
		}
	}

	effort := -1
	if e, ok := settings["effort"]; ok {
		if v, err := strconv.Atoi(e); err == nil {
			effort = v
		}
	}

	// find binary
	bin, err := findImageMagickBinary()
	if err != nil {
		return err
	}

	// prepare output name
	width, height := parseResolution(settings)
	outName := buildOutputPath(name, "avif", width, height)
	tmp := buildTempOutputPath(outName)

	var args []string
	args = append(args, name)
	args = appendCommonArgs(args, mapWithout(settings, "quality", "effort"))
	// image magick avif options - we'll set quality and effort if present
	args = append(args, "-quality", strconv.Itoa(quality))
	if effort >= 0 {
		args = append(args, "-define", fmt.Sprintf("avif:effort=%d", effort))
	}
	args = append(args, tmp)

	return execImageMagick(bin, args, tmp, outName, name, "avif")
}
