package handlers

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"pixerver/internal/auth"
	"pixerver/internal/uuidv7"
	"pixerver/logger"
)

var (
	postFormValidatorMu     sync.RWMutex
	postFormValidator       *auth.Validator
	postFormValidatorErr    error
	postFormValidatorLoaded bool
)

// PostFormHandler handles multipart file uploads from the form field "file".
// It stores the uploaded file under ./uploads with the filename pattern:
// <sha256>_<uuidv7>_<base32(originalName)>.extension
func PostFormHandler(w http.ResponseWriter, r *http.Request) {
	// limit request body size to 100MB to avoid OOM from huge uploads
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	validator, err := getPostFormValidator()
	if err != nil {
		http.Error(w, "server configuration error", http.StatusInternalServerError)
		logger.Errorf("postform: jwt validator init failed: %v", err)
		return
	}

	token, err := extractBearerToken(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		logger.Warnf("postform: %v", err)
		return
	}

	if _, err := validator.Validate(token); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		logger.Warnf("postform: jwt validation failed: %v", err)
		return
	}

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

func extractBearerToken(r *http.Request) (string, error) {
	const prefix = "Bearer "
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return "", errors.New("missing Authorization header")
	}
	if !strings.HasPrefix(authz, prefix) {
		return "", errors.New("invalid Authorization header")
	}
	token := strings.TrimSpace(authz[len(prefix):])
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}

func getPostFormValidator() (*auth.Validator, error) {
	postFormValidatorMu.RLock()
	if postFormValidatorLoaded {
		v := postFormValidator
		err := postFormValidatorErr
		postFormValidatorMu.RUnlock()
		return v, err
	}
	postFormValidatorMu.RUnlock()

	postFormValidatorMu.Lock()
	defer postFormValidatorMu.Unlock()
	if postFormValidatorLoaded {
		return postFormValidator, postFormValidatorErr
	}
	cfg := auth.Config{
		Secret:   os.Getenv("POSTFORM_JWT_SECRET"),
		Audience: os.Getenv("POSTFORM_JWT_AUDIENCE"),
		Issuer:   os.Getenv("POSTFORM_JWT_ISSUER"),
	}
	postFormValidatorLoaded = true
	postFormValidator, postFormValidatorErr = auth.NewValidator(cfg)
	return postFormValidator, postFormValidatorErr
}

// test helpers (no-op in production codepath, but used by unit tests)
func setPostFormValidatorForTest(v *auth.Validator) {
	postFormValidatorMu.Lock()
	defer postFormValidatorMu.Unlock()
	postFormValidator = v
	postFormValidatorErr = nil
	postFormValidatorLoaded = true
}

func resetPostFormValidatorForTest() {
	postFormValidatorMu.Lock()
	defer postFormValidatorMu.Unlock()
	postFormValidator = nil
	postFormValidatorErr = nil
	postFormValidatorLoaded = false
}
