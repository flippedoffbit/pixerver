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
	"pixerver/uploads"
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
	inputToken := `{"callbackUrl":"http://127.0.0.1:1/callback","backends":{"directory":"./public"},"resolutions":{"small":{"width":12,"height":9}},"conversionJobs":[{"type":"webp","resolutions":["small"],"destinationBackends":["directory"]}]}`
	if err := w.WriteField("token", inputToken); err != nil {
		t.Fatalf("write token: %v", err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.PostFormHandler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp uploads.UploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	fname := resp.Filename
	if fname == "" || resp.UploadID == "" || len(resp.Jobs) != 1 {
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
		t.Fatalf("encoder should not run during upload request")
		return "", nil
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

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp uploads.UploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if resp.UploadID == "" || len(resp.Jobs) != 1 || resp.Jobs[0].Status != uploads.JobStatusQueued {
		t.Fatalf("unexpected queue response: %+v", resp)
	}
	if len(testQueueMessages(t, srv)) != 1 {
		t.Fatalf("expected one queued message")
	}
}

func TestUploadStatusHandler(t *testing.T) {
	srv, secret := setupTestValidator(t)
	token := mustMakeToken(t, secret)

	now := time.Now().UTC()
	repo := testRepo(t, srv)
	upload := &uploads.UploadRecord{
		UploadID:       "upload-1",
		SourceFileName: "source.png",
		SourcePath:     "/tmp/source.png",
		Status:         uploads.UploadStatusQueued,
		JobIDs:         []string{"job-1"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	job := &uploads.JobRecord{
		JobID:     "job-1",
		UploadID:  "upload-1",
		Type:      "webp",
		Status:    uploads.JobStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.SaveUpload(upload); err != nil {
		t.Fatalf("save upload: %v", err)
	}
	if err := repo.SaveJob(job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	req := httptest.NewRequest("GET", "/uploads/upload-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.UploadStatusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp uploads.UploadStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.UploadID != "upload-1" || len(resp.Jobs) != 1 {
		t.Fatalf("unexpected status response: %+v", resp)
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
	repo := newMemoryRepo()
	q := &memoryQueue{}
	s.Uploads = &uploads.Service{
		Repo:  repo,
		Queue: q,
	}
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

func testRepo(t *testing.T, srv *Server) *memoryRepo {
	t.Helper()
	repo, ok := srv.Uploads.Repo.(*memoryRepo)
	if !ok {
		t.Fatalf("unexpected repo type")
	}
	return repo
}

func testQueueMessages(t *testing.T, srv *Server) []map[string]interface{} {
	t.Helper()
	q, ok := srv.Uploads.Queue.(*memoryQueue)
	if !ok {
		t.Fatalf("unexpected queue type")
	}
	return q.messages
}

type memoryQueue struct {
	messages []map[string]interface{}
}

func (q *memoryQueue) Produce(values map[string]interface{}) (string, error) {
	q.messages = append(q.messages, values)
	return "msg", nil
}

type memoryRepo struct {
	uploads map[string]*uploads.UploadRecord
	jobs    map[string]*uploads.JobRecord
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		uploads: map[string]*uploads.UploadRecord{},
		jobs:    map[string]*uploads.JobRecord{},
	}
}

func (r *memoryRepo) SaveUpload(upload *uploads.UploadRecord) error {
	cp := *upload
	r.uploads[upload.UploadID] = &cp
	return nil
}

func (r *memoryRepo) GetUpload(uploadID string) (*uploads.UploadRecord, error) {
	upload, ok := r.uploads[uploadID]
	if !ok {
		return nil, uploads.ErrNotFound
	}
	cp := *upload
	return &cp, nil
}

func (r *memoryRepo) SaveJob(job *uploads.JobRecord) error {
	cp := *job
	r.jobs[job.JobID] = &cp
	return nil
}

func (r *memoryRepo) GetJob(jobID string) (*uploads.JobRecord, error) {
	job, ok := r.jobs[jobID]
	if !ok {
		return nil, uploads.ErrNotFound
	}
	cp := *job
	return &cp, nil
}

func (r *memoryRepo) JobsForUpload(upload *uploads.UploadRecord) ([]uploads.JobRecord, error) {
	jobs := make([]uploads.JobRecord, 0, len(upload.JobIDs))
	for _, jobID := range upload.JobIDs {
		job, err := r.GetJob(jobID)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

func (r *memoryRepo) Close() error {
	return nil
}
