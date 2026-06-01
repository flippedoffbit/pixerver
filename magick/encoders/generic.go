package encoders

import (
	"fmt"
	"strconv"
)

// HandleGeneric creates an ImageMagick-backed encoder for formats that do not
// need special per-codec handling. Actual codec availability depends on the
// ImageMagick build installed on the host.
func HandleGeneric(format string) func(string, map[string]string) error {
	return func(name string, settings map[string]string) error {
		bin, err := findImageMagickBinary()
		if err != nil {
			return err
		}

		width, height := parseResolution(settings)
		outName := buildOutputPath(name, format, width, height)
		tmp := buildTempOutputPath(outName)

		args := []string{name}
		args = appendCommonArgs(args, settings)
		args = appendFormatArgs(args, format, settings)
		args = append(args, tmp)

		return execImageMagick(bin, args, tmp, outName, name, format)
	}
}

func appendFormatArgs(args []string, format string, settings map[string]string) []string {
	switch format {
	case "png":
		if level := settings["compressionLevel"]; level != "" {
			if v, err := strconv.Atoi(level); err == nil && v >= 0 && v <= 9 {
				args = append(args, "-define", fmt.Sprintf("png:compression-level=%d", v))
			}
		}
	case "gif":
		if delay := settings["delay"]; delay != "" {
			args = append(args, "-delay", delay)
		}
		if loop := settings["loop"]; loop != "" {
			args = append(args, "-loop", loop)
		}
	case "tiff":
		if compression := settings["compression"]; compression != "" {
			args = append(args, "-compress", compression)
		}
	}
	return args
}
