package uploads

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"pixerver/models"
	"pixerver/pipeline"
)

func TestCreateUploadPersistsJobsAndEnqueues(t *testing.T) {
	repo := newTestRepo()
	queue := &testQueue{}
	service := &Service{Repo: repo, Queue: queue}
	token := testToken(t, t.TempDir())

	resp, err := service.CreateUpload(context.Background(), "source.png", "/tmp/source.png", token)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if resp.UploadID == "" || resp.Status != UploadStatusQueued || len(resp.Jobs) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	upload, err := repo.GetUpload(resp.UploadID)
	if err != nil {
		t.Fatalf("get upload: %v", err)
	}
	if len(upload.JobIDs) != 1 {
		t.Fatalf("job ids = %v", upload.JobIDs)
	}
	if len(queue.messages) != 1 {
		t.Fatalf("messages = %d", len(queue.messages))
	}
	if queue.messages[0]["uploadId"] != resp.UploadID || queue.messages[0]["jobId"] != resp.Jobs[0].JobID {
		t.Fatalf("unexpected queue payload: %+v", queue.messages[0])
	}
}

func TestProcessJobCompletesUploadAndSendsSignedCallback(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	if err := os.WriteFile(source, []byte("image"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var callbackAuth string
	var callbackPayload UploadStatusResponse
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callbackAuth = req.Header.Get("Authorization")
		if err := json.NewDecoder(req.Body).Decode(&callbackPayload); err != nil {
			t.Fatalf("decode callback: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Status:     "204 No Content",
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
		}, nil
	})}

	repo := newTestRepo()
	queue := &testQueue{}
	service := &Service{
		Repo:  repo,
		Queue: queue,
		Processor: pipeline.Processor{
			OutputDir:  filepath.Join(dir, "processed"),
			HTTPClient: client,
			Encoder: func(input, typ string, settings map[string]string) (string, error) {
				out := input + ".webp"
				return out, os.WriteFile(out, []byte("encoded"), 0o644)
			},
		},
		Signer: &CallbackSigner{
			Secret: "callback-secret",
			Issuer: "pixerver-test",
			TTL:    time.Minute,
			Now:    time.Now,
		},
	}
	token := testToken(t, filepath.Join(dir, "public"))

	resp, err := service.CreateUpload(context.Background(), "source.png", source, token)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if err := service.ProcessJob(context.Background(), resp.UploadID, resp.Jobs[0].JobID); err != nil {
		t.Fatalf("ProcessJob: %v", err)
	}

	upload, err := repo.GetUpload(resp.UploadID)
	if err != nil {
		t.Fatalf("get upload: %v", err)
	}
	if upload.Status != UploadStatusCompleted || !upload.CallbackSent || upload.CallbackStatus != "sent" {
		t.Fatalf("unexpected upload: %+v", upload)
	}
	if callbackPayload.UploadID != resp.UploadID || len(callbackPayload.Artifacts) != 1 {
		t.Fatalf("unexpected callback payload: %+v", callbackPayload)
	}
	const prefix = "Bearer "
	if len(callbackAuth) <= len(prefix) || callbackAuth[:len(prefix)] != prefix {
		t.Fatalf("missing callback auth: %q", callbackAuth)
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(callbackAuth[len(prefix):], claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("callback-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("callback jwt invalid: %v", err)
	}
	if claims["uploadId"] != resp.UploadID || claims["sub"] != resp.UploadID {
		t.Fatalf("unexpected callback claims: %+v", claims)
	}
}

func TestProcessJobFailureRespectsMaxAttempts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	if err := os.WriteFile(source, []byte("image"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	repo := newTestRepo()
	service := &Service{
		Repo:        repo,
		Queue:       &testQueue{},
		MaxAttempts: 1,
		Processor: pipeline.Processor{
			OutputDir: filepath.Join(dir, "processed"),
			Encoder: func(input, typ string, settings map[string]string) (string, error) {
				return "", os.ErrInvalid
			},
		},
	}
	resp, err := service.CreateUpload(context.Background(), "source.png", source, testToken(t, filepath.Join(dir, "public")))
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if err := service.ProcessJob(context.Background(), resp.UploadID, resp.Jobs[0].JobID); err == nil {
		t.Fatalf("expected processing error")
	}
	job, err := repo.GetJob(resp.Jobs[0].JobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	upload, err := repo.GetUpload(resp.UploadID)
	if err != nil {
		t.Fatalf("get upload: %v", err)
	}
	if job.Status != JobStatusFailed || upload.Status != UploadStatusFailed || job.LastError == "" {
		t.Fatalf("unexpected failure state job=%+v upload=%+v", job, upload)
	}
}

func testToken(t *testing.T, destDir string) *models.InputToken {
	t.Helper()
	return &models.InputToken{
		CallbackURL: "https://example.local/callback",
		Backends:    map[string]string{"directory": destDir},
		Resolutions: map[string]models.Resolution{"small": {Width: 10, Height: 8}},
		ConversionJobs: []models.ConversionJob{{
			Type:                "webp",
			Resolutions:         []string{"small"},
			DestinationBackends: []string{"directory"},
			Settings:            map[string]string{"quality": "75"},
		}},
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type testQueue struct {
	messages []map[string]interface{}
}

func (q *testQueue) Produce(values map[string]interface{}) (string, error) {
	q.messages = append(q.messages, values)
	return "msg", nil
}

type testRepo struct {
	uploads map[string]*UploadRecord
	jobs    map[string]*JobRecord
}

func newTestRepo() *testRepo {
	return &testRepo{
		uploads: map[string]*UploadRecord{},
		jobs:    map[string]*JobRecord{},
	}
}

func (r *testRepo) SaveUpload(upload *UploadRecord) error {
	cp := *upload
	r.uploads[upload.UploadID] = &cp
	return nil
}

func (r *testRepo) GetUpload(uploadID string) (*UploadRecord, error) {
	upload, ok := r.uploads[uploadID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *upload
	return &cp, nil
}

func (r *testRepo) SaveJob(job *JobRecord) error {
	cp := *job
	r.jobs[job.JobID] = &cp
	return nil
}

func (r *testRepo) GetJob(jobID string) (*JobRecord, error) {
	job, ok := r.jobs[jobID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *job
	return &cp, nil
}

func (r *testRepo) JobsForUpload(upload *UploadRecord) ([]JobRecord, error) {
	jobs := make([]JobRecord, 0, len(upload.JobIDs))
	for _, jobID := range upload.JobIDs {
		job, err := r.GetJob(jobID)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

func (r *testRepo) Close() error {
	return nil
}
