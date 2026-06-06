package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pixerver/logger"
	"pixerver/magick/encoders"
	"pixerver/models"
)

// EncoderFunc converts input into the requested type and returns the artifact path.
type EncoderFunc func(input, typ string, settings map[string]string) (string, error)

// Processor runs token-defined image conversion jobs and stores their artifacts.
type Processor struct {
	OutputDir       string
	HTTPClient      *http.Client
	Encoder         EncoderFunc
	BackendResolver BackendResolver
}

// Artifact describes one stored output created by a job/backend pair.
type Artifact struct {
	JobID     string            `json:"jobId"`
	Type      string            `json:"type"`
	Backend   string            `json:"backend"`
	Status    string            `json:"status"`
	Location  string            `json:"location"`
	Width     int               `json:"width,omitempty"`
	Height    int               `json:"height,omitempty"`
	Settings  map[string]string `json:"settings,omitempty"`
	SourceJob string            `json:"sourceJob,omitempty"`
}

// Result is returned to HTTP callers and sent to callbackUrl when configured.
type Result struct {
	SourceFileName string     `json:"sourceFileName"`
	Artifacts      []Artifact `json:"artifacts"`
}

// Process validates the token, expands conversion jobs, encodes variants, stores
// them to each requested backend, and posts a callback.
func (p Processor) Process(ctx context.Context, token *models.InputToken, sourcePath string) (Result, error) {
	if err := token.Validate(); err != nil {
		return Result{}, err
	}
	if sourcePath == "" {
		return Result{}, errors.New("source path is required")
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return Result{}, fmt.Errorf("source file unavailable: %w", err)
	}

	outputDir := p.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(filepath.Dir(sourcePath), "processed")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output dir: %w", err)
	}

	encoder := p.Encoder
	if encoder == nil {
		encoder = encoders.Encode
	}

	jobs := models.ConversionJobs(token.ConversionJobs).ToJobs(token.Resolutions, filepath.Base(sourcePath))
	result := Result{SourceFileName: filepath.Base(sourcePath)}
	for _, job := range jobs {
		artifactPath, settings, err := p.encodeJob(outputDir, sourcePath, job, encoder)
		if err != nil {
			return result, fmt.Errorf("job %s encode failed: %w", job.ID, err)
		}
		for _, backendID := range job.DestinationBackendIDs {
			location, status, err := p.storeArtifact(ctx, token, backendID, artifactPath)
			if err != nil {
				return result, fmt.Errorf("job %s backend %s failed: %w", job.ID, backendID, err)
			}
			result.Artifacts = append(result.Artifacts, Artifact{
				JobID:     job.ID,
				Type:      job.Type,
				Backend:   backendID,
				Status:    status,
				Location:  location,
				Width:     job.Resolution.Width,
				Height:    job.Resolution.Height,
				Settings:  maps.Clone(settings),
				SourceJob: filepath.Base(artifactPath),
			})
		}
	}

	if err := p.sendCallback(ctx, token.CallbackURL, result); err != nil {
		return result, err
	}
	return result, nil
}

// ProcessJob encodes and stores one concrete job. It is used by queue workers;
// Process remains as the synchronous upload-level compatibility wrapper.
func (p Processor) ProcessJob(ctx context.Context, token *models.InputToken, sourcePath string, job models.Job) ([]Artifact, error) {
	if err := token.Validate(); err != nil {
		return nil, err
	}
	if sourcePath == "" {
		return nil, errors.New("source path is required")
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return nil, fmt.Errorf("source file unavailable: %w", err)
	}

	outputDir := p.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(filepath.Dir(sourcePath), "processed")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	encoder := p.Encoder
	if encoder == nil {
		encoder = encoders.Encode
	}

	artifactPath, settings, err := p.encodeJob(outputDir, sourcePath, job, encoder)
	if err != nil {
		return nil, fmt.Errorf("job %s encode failed: %w", job.ID, err)
	}

	artifacts := make([]Artifact, 0, len(job.DestinationBackendIDs))
	for _, backendID := range job.DestinationBackendIDs {
		location, status, err := p.storeArtifact(ctx, token, backendID, artifactPath)
		if err != nil {
			return artifacts, fmt.Errorf("job %s backend %s failed: %w", job.ID, backendID, err)
		}
		artifacts = append(artifacts, Artifact{
			JobID:     job.ID,
			Type:      job.Type,
			Backend:   backendID,
			Status:    status,
			Location:  location,
			Width:     job.Resolution.Width,
			Height:    job.Resolution.Height,
			Settings:  maps.Clone(settings),
			SourceJob: filepath.Base(artifactPath),
		})
	}
	return artifacts, nil
}

func (p Processor) encodeJob(outputDir, sourcePath string, job models.Job, encoder EncoderFunc) (string, map[string]string, error) {
	workPath := filepath.Join(outputDir, fmt.Sprintf("%s%s", job.ID, filepath.Ext(sourcePath)))
	if err := copyFile(sourcePath, workPath); err != nil {
		return "", nil, err
	}

	settings := maps.Clone(job.Settings)
	if settings == nil {
		settings = make(map[string]string)
	}
	if job.Resolution.Width > 0 {
		settings["width"] = fmt.Sprintf("%d", job.Resolution.Width)
	}
	if job.Resolution.Height > 0 {
		settings["height"] = fmt.Sprintf("%d", job.Resolution.Height)
	}

	outPath, err := encoder(workPath, job.Type, settings)
	if err != nil {
		_ = os.Remove(workPath)
		return "", settings, err
	}
	if outPath != workPath {
		_ = os.Remove(workPath)
	}
	return outPath, settings, nil
}

func (p Processor) storeArtifact(ctx context.Context, token *models.InputToken, backendID, artifactPath string) (string, string, error) {
	spec, ok := token.GetBackend(backendID)
	if !ok {
		return "", "", fmt.Errorf("unknown backend %q", backendID)
	}
	resolved, configured, err := p.resolveBackendSpec(ctx, spec)
	if err != nil {
		return "", "", err
	}
	kind := backendKindFromID(backendID)
	target := resolved
	if configured && isDirectURLBackendSpec(resolved) {
		urlKind, urlTarget := parseBackendSpec(kind, resolved)
		kind = urlKind
		target = urlTarget
	}
	switch kind {
	case "directory":
		location, err := storeDirectory(target, artifactPath)
		return location, "stored", err
	case "http":
		if !configured || target == "" {
			return "unconfigured:http:" + spec, "unconfigured", nil
		}
		location, err := p.storeHTTP(ctx, target, artifactPath)
		return location, "stored", err
	case "s3", "gcs", "azure", "ftp":
		if !configured {
			return "unconfigured:" + kind + ":" + spec, "unconfigured", nil
		}
		location, err := p.storeExternal(ctx, kind, resolved, artifactPath)
		return location, "stored", err
	default:
		return "unsupported:" + kind + ":" + spec, "unsupported", nil
	}
}

func storeDirectory(target, artifactPath string) (string, error) {
	if target == "" {
		target = filepath.Join(filepath.Dir(artifactPath), "directory")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(target, filepath.Base(artifactPath))
	if sameFilePath(dst, artifactPath) {
		return dst, nil
	}
	if err := copyFile(artifactPath, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func (p Processor) storeHTTP(ctx context.Context, target, artifactPath string) (string, error) {
	if target == "" {
		return "", errors.New("http backend requires a target URL")
	}
	if _, err := url.ParseRequestURI(target); err != nil {
		return "", fmt.Errorf("invalid http backend URL: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(artifactPath))
	if err != nil {
		return "", err
	}
	file, err := os.Open(artifactPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("http backend returned %s", resp.Status)
	}
	return target, nil
}

func (p Processor) sendCallback(ctx context.Context, callbackURL string, result Result) error {
	return SendCallback(ctx, p.HTTPClient, callbackURL, result, "")
}

// SendCallback posts callback JSON and optionally attaches a bearer token.
// Delivery is best-effort for network and non-2xx failures.
func SendCallback(ctx context.Context, client *http.Client, callbackURL string, payload interface{}, bearerToken string) error {
	if callbackURL == "" {
		return nil
	}
	if _, err := url.ParseRequestURI(callbackURL); err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warnf("pipeline: callback failed: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logger.Warnf("pipeline: callback returned %s", resp.Status)
		return nil
	}
	logger.Infof("pipeline: callback delivered to %s", callbackURL)
	return nil
}

func (p Processor) resolveBackendSpec(ctx context.Context, spec string) (string, bool, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", false, nil
	}
	if isDirectBackendSpec(spec) {
		return spec, true, nil
	}
	if p.BackendResolver != nil {
		resolved, ok, err := p.BackendResolver.ResolveBackendConfig(ctx, spec)
		if err != nil {
			return "", false, err
		}
		if ok {
			return strings.TrimSpace(resolved), true, nil
		}
	}
	if v := os.Getenv(spec); v != "" {
		return strings.TrimSpace(v), true, nil
	}
	return spec, false, nil
}

func isDirectBackendSpec(spec string) bool {
	if strings.HasPrefix(spec, "{") || strings.Contains(spec, "://") {
		return true
	}
	if filepath.IsAbs(spec) || strings.HasPrefix(spec, ".") {
		return true
	}
	return false
}

func isDirectURLBackendSpec(spec string) bool {
	return strings.Contains(strings.TrimSpace(spec), "://")
}

func backendKindFromID(backendID string) string {
	backendID = strings.TrimSpace(backendID)
	for _, sep := range []string{":", ".", "/"} {
		if idx := strings.Index(backendID, sep); idx > 0 {
			return backendID[:idx]
		}
	}
	return backendID
}

func parseBackendSpec(defaultKind, spec string) (kind, target string) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return defaultKind, ""
	}
	if strings.Contains(spec, "://") {
		u, err := url.Parse(spec)
		if err == nil && u.Scheme != "" {
			if u.Scheme == "http" || u.Scheme == "https" {
				return "http", spec
			}
			return u.Scheme, spec
		}
	}
	if strings.HasPrefix(spec, "directory:") {
		return "directory", strings.TrimPrefix(spec, "directory:")
	}
	if strings.HasPrefix(spec, "http:") || strings.HasPrefix(spec, "https:") {
		return "http", spec
	}
	if filepath.IsAbs(spec) || strings.HasPrefix(spec, ".") {
		return "directory", spec
	}
	if defaultKind == "directory" {
		return "directory", ""
	}
	if defaultKind == "http" {
		return "http", ""
	}
	return defaultKind, spec
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if st, err := os.Stat(src); err == nil {
		_ = os.Chmod(dst, st.Mode())
	}
	return nil
}

func sameFilePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return aa == bb
}
