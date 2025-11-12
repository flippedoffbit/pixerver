package handlers

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostFormHandler(t *testing.T) {
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
	rec := httptest.NewRecorder()

	PostFormHandler(rec, req)

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
