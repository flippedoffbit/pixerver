package encoders

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"pixerver/logger"
)

// findImageMagickBinary locates the ImageMagick binary (magick v7 or convert).
func findImageMagickBinary() (string, error) {
	bin, err := exec.LookPath("magick")
	if err != nil {
		bin, err = exec.LookPath("convert")
		if err != nil {
			return "", fmt.Errorf("image magick not found (tried 'magick' and 'convert'): %w", err)
		}
	}
	return bin, nil
}

// parseResolution extracts width and height from settings.
func parseResolution(settings map[string]string) (width, height int) {
	if w, ok := settings["width"]; ok {
		if v, err := strconv.Atoi(w); err == nil {
			width = v
		}
	}
	if h, ok := settings["height"]; ok {
		if v, err := strconv.Atoi(h); err == nil {
			height = v
		}
	}
	return
}

// buildOutputPath constructs the output filename with size suffix.
func buildOutputPath(inputName string, ext string, width, height int) string {
	base := inputName
	if e := filepath.Ext(inputName); e != "" {
		base = inputName[:len(inputName)-len(e)]
	}

	sizeSuffix := "orig"
	if width != 0 || height != 0 {
		sizeSuffix = fmt.Sprintf("%d_%d", width, height)
	}

	return filepath.Join(filepath.Dir(inputName), fmt.Sprintf("%s_%s.%s", filepath.Base(base), sizeSuffix, ext))
}

func buildTempOutputPath(finalPath string) string {
	ext := filepath.Ext(finalPath)
	base := strings.TrimSuffix(finalPath, ext)
	if ext == "" {
		return finalPath + ".tmp"
	}
	return base + ".tmp" + ext
}

func appendCommonArgs(args []string, settings map[string]string) []string {
	strip := true
	if s, ok := settings["strip"]; ok && (s == "false" || s == "0") {
		strip = false
	}
	if strip {
		args = append(args, "-strip")
	}

	if q, ok := settings["quality"]; ok && q != "" {
		if v, err := strconv.Atoi(q); err == nil && v >= 0 && v <= 100 {
			args = append(args, "-quality", strconv.Itoa(v))
		} else {
			logger.Warnf("invalid quality %q, skipping", q)
		}
	}

	width, height := parseResolution(settings)
	args = appendResizeArgs(args, settings, width, height)

	if settings["flatten"] == "true" || settings["flatten"] == "1" {
		if bg := settings["background"]; bg != "" {
			args = append(args, "-background", bg)
		}
		args = append(args, "-flatten")
	}

	return args
}

func appendResizeArgs(args []string, settings map[string]string, width, height int) []string {
	if width == 0 && height == 0 {
		return args
	}
	resizeArg := fmt.Sprintf("%dx%d", width, height)
	if settings["ignoreAspect"] == "true" || settings["ignoreAspect"] == "1" {
		resizeArg += "!"
	} else if settings["resizeMode"] == "fill" {
		resizeArg += "^"
	}
	args = append(args, "-resize", resizeArg)
	if settings["resizeMode"] == "fill" {
		args = append(args, "-gravity", "center", "-extent", fmt.Sprintf("%dx%d", width, height))
	}
	return args
}

func mapWithout(settings map[string]string, keys ...string) map[string]string {
	out := make(map[string]string, len(settings))
	skip := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		skip[key] = struct{}{}
	}
	for key, value := range settings {
		if _, ok := skip[key]; ok {
			continue
		}
		out[key] = value
	}
	return out
}

// execImageMagick runs ImageMagick with given args and handles temp file cleanup.
func execImageMagick(bin string, args []string, tmpPath, finalPath, inputName string, logPrefix string) error {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("%s conversion failed: %v output=%s", logPrefix, err, string(out))
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%s conversion failed: %v: %s", logPrefix, err, string(out))
	}

	// preserve file mode from input
	if st, err := os.Stat(inputName); err == nil {
		_ = os.Chmod(tmpPath, st.Mode())
	}

	// atomic move
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to move %s output into place: %v", logPrefix, err)
	}

	logger.Debugf("%s created: %s", logPrefix, finalPath)
	return nil
}
