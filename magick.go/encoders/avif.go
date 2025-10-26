package encoders

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"pixerver/logger"
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
	bin, err := exec.LookPath("magick")
	if err != nil {
		bin, err = exec.LookPath("convert")
		if err != nil {
			return fmt.Errorf("image magick not found: %w", err)
		}
	}

	// prepare output name
	ext := "avif"
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
	// image magick avif options - we'll set quality and effort if present
	args = append(args, "-quality", strconv.Itoa(quality))
	if effort >= 0 {
		args = append(args, "-define", fmt.Sprintf("avif:effort=%d", effort))
	}
	if width != 0 || height != 0 {
		args = append(args, "-resize", fmt.Sprintf("%dx%d", width, height))
	}
	args = append(args, tmp)

	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("avif conversion failed: %v output=%s", err, string(out))
		_ = os.Remove(tmp)
		return fmt.Errorf("avif conversion failed: %v: %s", err, string(out))
	}

	if st, err := os.Stat(name); err == nil {
		_ = os.Chmod(tmp, st.Mode())
	}
	if err := os.Rename(tmp, outName); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to move avif output into place: %v", err)
	}

	logger.Debugf("avif created: %s", outName)
	return nil
}
