package encoders

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"pixerver/logger"
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

	// reuse JPEG flow by setting format to webp and appropriate defines
	s := make(map[string]string)
	for k, v := range settings {
		s[k] = v
	}
	s["format"] = "webp"
	// pass webp-specific defines via settings so HandleJPEG will append them
	// We'll construct a -define string via settings to be appended by HandleJPEG
	// However, HandleJPEG currently doesn't process arbitrary defines, so
	// instead we call ImageMagick directly here mirroring HandleJPEG behavior.

	// find binary
	bin, err := exec.LookPath("magick")
	if err != nil {
		bin, err = exec.LookPath("convert")
		if err != nil {
			return fmt.Errorf("image magick not found: %w", err)
		}
	}

	// prepare output name
	ext := "webp"
	base := name
	if e := filepath.Ext(name); e != "" {
		base = name[:len(name)-len(e)]
	}
	width := 0
	height := 0
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
	sizeSuffix := "orig"
	if width != 0 || height != 0 {
		sizeSuffix = fmt.Sprintf("%d_%d", width, height)
	}
	outName := filepath.Join(filepath.Dir(name), fmt.Sprintf("%s_%s.%s", filepath.Base(base), sizeSuffix, ext))
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

	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("webp conversion failed: %v output=%s", err, string(out))
		_ = os.Remove(tmp)
		return fmt.Errorf("webp conversion failed: %v: %s", err, string(out))
	}

	if st, err := os.Stat(name); err == nil {
		_ = os.Chmod(tmp, st.Mode())
	}
	if err := os.Rename(tmp, outName); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to move webp output into place: %v", err)
	}

	logger.Debugf("webp created: %s", outName)
	return nil
}
