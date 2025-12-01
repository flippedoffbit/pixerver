package encoders

import (
	"fmt"
	"os"
	"strconv"

	"pixerver/logger"
)

// HandleJPEG handles JPEG encoding using ImageMagick CLI.
// name is the path to the input file (will be replaced in-place).
// settings may contain:
//   - "quality" : integer JPEG quality (0-100)
//   - "progressive" : "true"/"false" (use progressive/interlace)
//   - "strip" : "true"/"false" (strip metadata)
//   - "optimize" : "true"/"false" (try to enable jpeg optimization)
//
// The function writes a temporary output file then atomically replaces
// the original file when successful.
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
	tmp := outName + ".tmp"

	// build args: [input ...options... output]
	var args []string
	// when using 'magick' the binary takes input then options then output;
	// when using 'convert' it's the same layout.
	args = append(args, name)
	if strip {
		args = append(args, "-strip")
	}
	args = append(args, "-quality", strconv.Itoa(quality))
	if progressive {
		// use Plane which is progressive JPEG
		args = append(args, "-interlace", "Plane")
	}
	if optimize {
		args = append(args, "-define", "jpeg:optimize-coding=true")
	}
	// if resize specified, add resize option (encoder handles efficient resize)
	if width != 0 || height != 0 {
		resizeArg := fmt.Sprintf("%dx%d", width, height)
		args = append(args, "-resize", resizeArg)
	}

	args = append(args, tmp)

	// use execImageMagick but then rename to original name (JPEG replaces in-place)
	if err := execImageMagick(bin, args, tmp, outName, name, outExt); err != nil {
		return err
	}

	// JPEG encoder replaces original file, so rename outName to original name
	if outName != name {
		if err := os.Rename(outName, name); err != nil {
			// attempt copy-on-fail fallback
			logger.Warnf("rename tmp -> original failed: %v, attempting copy fallback", err)
			in, rerr := os.ReadFile(outName)
			if rerr == nil {
				werr := os.WriteFile(name, in, 0o644)
				_ = os.Remove(outName)
				if werr != nil {
					return fmt.Errorf("failed to replace original file: %v", werr)
				}
				return nil
			}
			return fmt.Errorf("failed to replace original file: %v", err)
		}
	}

	logger.Debugf("jpeg encoding completed for %s", name)
	return nil
}
