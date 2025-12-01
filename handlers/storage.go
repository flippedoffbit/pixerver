package handlers

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"pixerver/internal/uuidv7"
)

// saveUploadedFile persists an uploaded file to the configured upload
// directory for the server.
// It returns the generated filename, the full path, or an error.
func (s *Server) saveUploadedFile(file multipart.File, header *multipart.FileHeader) (filename, fullPath string, err error) {
	outDir := s.UploadDir
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir uploads failed: %w", err)
	}

	tmp, err := os.CreateTemp(outDir, "upload-*.tmp")
	if err != nil {
		return "", "", fmt.Errorf("create temp file failed: %w", err)
	}
	tmpPath := tmp.Name()

	// Ensure cleanup. If rename succeeds, this will fail harmlessly (file gone).
	// If rename fails or panic occurs, this cleans up the temp file.
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	mw := io.MultiWriter(hasher, tmp)

	if _, err := io.Copy(mw, file); err != nil {
		tmp.Close()
		return "", "", fmt.Errorf("copying uploaded data failed: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return "", "", fmt.Errorf("close temp file failed: %w", err)
	}

	sum := hasher.Sum(nil)
	shaHex := fmt.Sprintf("%x", sum)

	orig := header.Filename
	ext := filepath.Ext(orig)
	nameOnly := strings.TrimSuffix(orig, ext)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	b32 := enc.EncodeToString([]byte(nameOnly))

	id := uuidv7.New()
	finalName := fmt.Sprintf("%s_%s_%s%s", shaHex, id, b32, ext)
	finalPath := filepath.Join(outDir, finalName)

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", "", fmt.Errorf("rename temp to final failed: %w", err)
	}

	return finalName, finalPath, nil
}
