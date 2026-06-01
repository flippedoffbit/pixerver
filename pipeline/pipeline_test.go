package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"pixerver/models"
)

func TestProcessorProcessDirectoryBackend(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	if err := os.WriteFile(source, []byte("image"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var callbackSeen bool
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callbackSeen = true
		var result Result
		if err := json.NewDecoder(req.Body).Decode(&result); err != nil {
			t.Fatalf("decode callback: %v", err)
		}
		if len(result.Artifacts) != 1 {
			t.Fatalf("callback artifacts = %d", len(result.Artifacts))
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Status:     "204 No Content",
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
		}, nil
	})}

	outDir := filepath.Join(dir, "processed")
	destDir := filepath.Join(dir, "public")
	token := &models.InputToken{
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
	processor := Processor{
		OutputDir:  outDir,
		HTTPClient: client,
		Encoder: func(input, typ string, settings map[string]string) (string, error) {
			if typ != "webp" {
				t.Fatalf("typ = %s", typ)
			}
			if settings["width"] != "10" || settings["height"] != "8" {
				t.Fatalf("missing resolution settings: %+v", settings)
			}
			out := input + ".webp"
			return out, os.WriteFile(out, []byte("encoded"), 0o644)
		},
	}

	result, err := processor.Process(context.Background(), token, source)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !callbackSeen {
		t.Fatalf("expected callback")
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("artifacts = %d", len(result.Artifacts))
	}
	if result.Artifacts[0].Status != "stored" {
		t.Fatalf("status = %s", result.Artifacts[0].Status)
	}
	location := result.Artifacts[0].Location
	if filepath.Dir(location) != destDir {
		t.Fatalf("artifact stored in %s, want %s", filepath.Dir(location), destDir)
	}
	if b, err := os.ReadFile(location); err != nil || string(b) != "encoded" {
		t.Fatalf("stored artifact = %q err=%v", string(b), err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProcessorUnconfiguredHTTPBackendDoesNotFail(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	if err := os.WriteFile(source, []byte("image"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	token := &models.InputToken{
		CallbackURL: "http://127.0.0.1:1/callback",
		Backends:    map[string]string{"http": "some-random-key"},
		Resolutions: map[string]models.Resolution{"original": {}},
		ConversionJobs: []models.ConversionJob{{
			Type:                "jpg",
			Resolutions:         []string{"original"},
			DestinationBackends: []string{"http"},
		}},
	}
	processor := Processor{
		OutputDir: filepath.Join(dir, "processed"),
		Encoder: func(input, typ string, settings map[string]string) (string, error) {
			out := input + ".jpg"
			return out, os.WriteFile(out, []byte("encoded"), 0o644)
		},
	}

	result, err := processor.Process(context.Background(), token, source)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := result.Artifacts[0].Location; got != "unconfigured:http:some-random-key" {
		t.Fatalf("location = %s", got)
	}
	if got := result.Artifacts[0].Status; got != "unconfigured" {
		t.Fatalf("status = %s", got)
	}
}

func TestBackendKeySelectsKindValueSelectsConfigToken(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	if err := os.WriteFile(source, []byte("image"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	destDir := filepath.Join(dir, "typed-directory")
	token := &models.InputToken{
		CallbackURL: "http://127.0.0.1:1/callback",
		Backends:    map[string]string{"directory:public": "directory-public-token"},
		Resolutions: map[string]models.Resolution{"original": {}},
		ConversionJobs: []models.ConversionJob{{
			Type:                "jpg",
			Resolutions:         []string{"original"},
			DestinationBackends: []string{"directory:public"},
		}},
	}
	processor := Processor{
		OutputDir: filepath.Join(dir, "processed"),
		BackendResolver: BackendResolverFunc(func(ctx context.Context, token string) (string, bool, error) {
			if token != "directory-public-token" {
				t.Fatalf("redis token = %s", token)
			}
			return destDir, true, nil
		}),
		Encoder: func(input, typ string, settings map[string]string) (string, error) {
			out := input + ".jpg"
			return out, os.WriteFile(out, []byte("encoded"), 0o644)
		},
	}

	result, err := processor.Process(context.Background(), token, source)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Artifacts[0].Backend != "directory:public" {
		t.Fatalf("backend = %s", result.Artifacts[0].Backend)
	}
	if result.Artifacts[0].Status != "stored" {
		t.Fatalf("status = %s", result.Artifacts[0].Status)
	}
	if filepath.Dir(result.Artifacts[0].Location) != destDir {
		t.Fatalf("location = %s", result.Artifacts[0].Location)
	}
}

func TestExternalBackendConfigParsing(t *testing.T) {
	cfg, err := parseExternalBackendConfig("s3", `{"bucket":"images","prefix":"out","region":"ap-south-1","accessKeyId":"ak","secretAccessKey":"sk"}`)
	if err != nil {
		t.Fatalf("parse s3 json: %v", err)
	}
	if cfg.Type != "s3" || cfg.Bucket != "images" || cfg.Prefix != "out" || cfg.Region != "ap-south-1" {
		t.Fatalf("unexpected s3 config: %+v", cfg)
	}

	cfg, err = parseExternalBackendConfig("gcs", "gs://pixerver-assets/variants")
	if err != nil {
		t.Fatalf("parse gcs url: %v", err)
	}
	if cfg.Type != "gcs" || cfg.Bucket != "pixerver-assets" || cfg.Prefix != "variants" {
		t.Fatalf("unexpected gcs config: %+v", cfg)
	}

	cfg, err = parseExternalBackendConfig("ftp", "ftp://user:pass@example.com:2121/uploads")
	if err != nil {
		t.Fatalf("parse ftp url: %v", err)
	}
	if cfg.Type != "ftp" || cfg.Host != "example.com" || cfg.Port != 2121 || cfg.Username != "user" || cfg.Password != "pass" || cfg.RemoteDir != "uploads" {
		t.Fatalf("unexpected ftp config: %+v", cfg)
	}
}

func TestResolveBackendSpec(t *testing.T) {
	t.Setenv("PIXERVER_TEST_S3", `{"bucket":"images"}`)
	processor := Processor{}
	if got, ok, err := processor.resolveBackendSpec(context.Background(), "PIXERVER_TEST_S3"); err != nil || !ok || got != `{"bucket":"images"}` {
		t.Fatalf("env resolve = %q %v err=%v", got, ok, err)
	}
	if got, ok, err := processor.resolveBackendSpec(context.Background(), "some-random-key"); err != nil || ok || got != "some-random-key" {
		t.Fatalf("unconfigured resolve = %q %v err=%v", got, ok, err)
	}
	if got, ok, err := processor.resolveBackendSpec(context.Background(), "s3://bucket/prefix"); err != nil || !ok || got != "s3://bucket/prefix" {
		t.Fatalf("direct resolve = %q %v err=%v", got, ok, err)
	}
	processor.BackendResolver = BackendResolverFunc(func(ctx context.Context, token string) (string, bool, error) {
		if token != "s3-primary" {
			t.Fatalf("token = %s", token)
		}
		return `{"type":"s3","bucket":"primary"}`, true, nil
	})
	if got, ok, err := processor.resolveBackendSpec(context.Background(), "s3-primary"); err != nil || !ok || got != `{"type":"s3","bucket":"primary"}` {
		t.Fatalf("resolver resolve = %q %v err=%v", got, ok, err)
	}
}

func TestBackendKindFromID(t *testing.T) {
	for input, want := range map[string]string{
		"s3":               "s3",
		"s3:public":        "s3",
		"s3.archive":       "s3",
		"directory/public": "directory",
	} {
		if got := backendKindFromID(input); got != want {
			t.Fatalf("backendKindFromID(%q) = %q, want %q", input, got, want)
		}
	}
}
