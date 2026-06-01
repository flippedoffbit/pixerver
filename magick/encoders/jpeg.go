package encoders

import (
	"strconv"

	"pixerver/logger"
)

// HandleJPEG handles JPEG encoding using ImageMagick CLI.
// name is the path to the input file.
// settings may contain:
//   - "quality" : integer JPEG quality (0-100)
//   - "progressive" : "true"/"false" (use progressive/interlace)
//   - "strip" : "true"/"false" (strip metadata)
//   - "optimize" : "true"/"false" (try to enable jpeg optimization)
//
// The function writes a temporary output file then atomically moves it into
// place next to the input.
func HandleJPEG(name string, settings map[string]string) error {
	// defaults
	quality := 80
	if q, ok := settings["quality"]; ok {
		if v, err := strconv.Atoi(q); err == nil && v >= 0 && v <= 100 {
			quality = v
		} else {
			logger.Warnf("invalid quality %q, using %d", q, quality)
		}
	}

	progressive := true
	if p, ok := settings["progressive"]; ok && (p == "false" || p == "0") {
		progressive = false
	}

	strip := true
	if s, ok := settings["strip"]; ok && (s == "false" || s == "0") {
		strip = false
	}

	optimize := false
	if o, ok := settings["optimize"]; ok && (o == "true" || o == "1") {
		optimize = true
	}

	logger.Debugf("jpeg encoder: file=%s quality=%d progressive=%v strip=%v optimize=%v", name, quality, progressive, strip, optimize)

	// find ImageMagick binary (magick v7) or fallback to convert
	bin, err := findImageMagickBinary()
	if err != nil {
		return err
	}

	// determine output format and target filename
	outExt := "jpg"
	if f, ok := settings["format"]; ok && f != "" {
		outExt = f
	}

	// parse width/height from settings if provided
	width, height := parseResolution(settings)

	outName := buildOutputPath(name, outExt, width, height)
	tmp := buildTempOutputPath(outName)

	// build args: [input ...options... output]
	var args []string
	// when using 'magick' the binary takes input then options then output;
	// when using 'convert' it's the same layout.
	args = append(args, name)
	args = appendCommonArgs(args, mapWithout(settings, "quality", "width", "height", "resizeMode", "ignoreAspect"))
	args = append(args, "-quality", strconv.Itoa(quality))
	if progressive {
		// use Plane which is progressive JPEG
		args = append(args, "-interlace", "Plane")
	}
	if optimize {
		args = append(args, "-define", "jpeg:optimize-coding=true")
	}
	args = appendResizeArgs(args, settings, width, height)

	args = append(args, tmp)

	if err := execImageMagick(bin, args, tmp, outName, name, outExt); err != nil {
		return err
	}

	logger.Debugf("jpeg encoding completed for %s", name)
	return nil
}
