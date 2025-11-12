package handlers

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pixerver/internal/uuidv7"
	"pixerver/logger"
)

// PostFormHandler handles multipart file uploads from the form field "file".
// It stores the uploaded file under ./uploads with the filename pattern:
// <sha256>_<uuidv7>_<base32(originalName)>.extension
func PostFormHandler(w http.ResponseWriter, r *http.Request) {
	// limit request body size to 100MB to avoid OOM from huge uploads
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		logger.Warnf("postform: parse multipart failed: %v", err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		logger.Warnf("postform: FormFile error: %v", err)
		return
	}
	defer file.Close()

	// ensure uploads dir
	outDir := "uploads"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		logger.Errorf("postform: mkdir uploads failed: %v", err)
		return
	}

	// create a temp file to stream data into
	tmp, err := os.CreateTemp(outDir, "upload-*.tmp")
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		logger.Errorf("postform: create temp file failed: %v", err)
		return
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		// if rename didn't happen, remove temp file
		_ = os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	mw := io.MultiWriter(hasher, tmp)

	if _, err := io.Copy(mw, file); err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		logger.Errorf("postform: copying uploaded data failed: %v", err)
		return
	}

	sum := hasher.Sum(nil)
	shaHex := fmt.Sprintf("%x", sum)

	// build base32-encoded original name (without extension)
	orig := header.Filename
	ext := filepath.Ext(orig)
	nameOnly := strings.TrimSuffix(orig, ext)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	b32 := enc.EncodeToString([]byte(nameOnly))

	id := uuidv7.New()
	finalName := fmt.Sprintf("%s_%s_%s%s", shaHex, id, b32, ext)
	finalPath := filepath.Join(outDir, finalName)

	// atomically move temp -> final
	if err := os.Rename(tmpPath, finalPath); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		logger.Errorf("postform: rename temp to final failed: %v", err)
		return
	}

	logger.Infof("postform: stored upload as %s", finalPath)

	// Respond with JSON containing the stored filename
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]string{"filename": finalName, "path": finalPath}
	_ = json.NewEncoder(w).Encode(resp)
}
