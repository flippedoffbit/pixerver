package handlers

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"pixerver/internal/auth"
)

func TestPostFormHandler(t *testing.T) {
	srv, secret := setupTestValidator(t)
	token := mustMakeToken(t, secret)

	// create temp dir and chdir so uploads/ is local to temp
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(fw, strings.NewReader("dummycontent")); err != nil {
		t.Fatalf("write content: %v", err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.PostFormHandler(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	fname, ok := resp["filename"]
	if !ok || fname == "" {
		t.Fatalf("missing filename in response")
	}

	// ensure file exists in uploads/
	finalPath := filepath.Join(dir, "uploads", fname)
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("uploaded file missing: %v", err)
	}

	// verify filename pattern: sha_uuid_base32.ext
	parts := strings.Split(fname, "_")
	if len(parts) != 3 {
		t.Fatalf("unexpected filename parts: %v", parts)
	}
	// check base32 decodes to original name
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	base32part := strings.TrimSuffix(parts[2], filepath.Ext(parts[2]))
	dec, err := enc.DecodeString(base32part)
	if err != nil {
		t.Fatalf("base32 decode failed: %v", err)
	}
	if string(dec) != "test" {
		t.Fatalf("decoded base32 mismatch: %s", string(dec))
	}
}

func TestPostFormHandlerProcessesToken(t *testing.T) {
	srv, secret := setupTestValidator(t)
	token := mustMakeToken(t, secret)

	dir := t.TempDir()
	srv.UploadDir = filepath.Join(dir, "uploads")
	srv.Processor.OutputDir = filepath.Join(dir, "processed")
	srv.Processor.Encoder = func(input, typ string, settings map[string]string) (string, error) {
		out := input + ".webp"
		return out, os.WriteFile(out, []byte("encoded"), 0o644)
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(fw, strings.NewReader("dummycontent")); err != nil {
		t.Fatalf("write content: %v", err)
	}
	inputToken := `{"callbackUrl":"http://127.0.0.1:1/callback","backends":{"directory":"` + filepath.Join(dir, "public") + `"},"resolutions":{"small":{"width":12,"height":9}},"conversionJobs":[{"type":"webp","resolutions":["small"],"destinationBackends":["directory"]}]}`
	if err := w.WriteField("token", inputToken); err != nil {
		t.Fatalf("write token: %v", err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.PostFormHandler(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if _, ok := resp["processing"]; !ok {
		t.Fatalf("expected processing result in response")
	}
}

func TestPostFormHandlerUnauthorized(t *testing.T) {
	srv, _ := setupTestValidator(t)

	req := httptest.NewRequest("POST", "/upload", nil)
	rec := httptest.NewRecorder()

	srv.PostFormHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}

func setupTestValidator(t *testing.T) (*Server, string) {
	t.Helper()
	secret := "unit-test-secret"
	v, err := auth.NewValidator(auth.Config{Secret: secret})
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	s := NewServer(v, "uploads")
	return s, secret
}

func mustMakeToken(t *testing.T, secret string) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   "tester",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return signed
}
