package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"pixerver/handlers"
	"pixerver/internal/auth"
	"pixerver/internal/env"
	"pixerver/logger"
	"pixerver/pipeline"
)

func main() {
	// load .env early (if present) so other packages can rely on env vars
	if _, err := os.Stat(".env"); err == nil {
		if _, err := env.Load(".env"); err != nil {
			// don't fail startup for an empty or partially malformed .env; just warn
			logger.Warnf("failed to load .env: %v", err)
		} else {
			logger.Infof("loaded .env")
		}
	}

	// Initialize package logger with defaults; enable debug for demo
	cfg := logger.Config{}
	trueVal := true
	cfg.Debug.Enabled = &trueVal
	logger.Init(cfg)

	logger.Debug("debug message: starting application")
	logger.Info("info message: application running")
	logger.Warn("warn message: demo warning")
	logger.Error("error message: demo error")

	if v, err := auth.NewValidatorFromEnv("POSTFORM_JWT_SECRET", "POSTFORM_JWT_AUDIENCE", "POSTFORM_JWT_ISSUER"); err == nil {
		uploadDir := os.Getenv("POSTFORM_UPLOAD_DIR")
		if uploadDir == "" {
			uploadDir = "uploads"
		}
		outputDir := os.Getenv("PIXERVER_OUTPUT_DIR")
		if outputDir == "" {
			outputDir = uploadDir + "/processed"
		}
		tokenPath := os.Getenv("PIXERVER_TOKEN_PATH")
		if tokenPath == "" {
			if _, err := os.Stat("baseToken.json"); err == nil {
				tokenPath = "baseToken.json"
			}
		}
		srv := handlers.NewServer(v, uploadDir)
		srv.TokenPath = tokenPath
		srv.HTTPClient = &http.Client{Timeout: 30 * time.Second}
		resolver, resolverErr := pipeline.NewRedisBackendResolver(os.Getenv("PIXERVER_BACKEND_CONFIG_PREFIX"))
		if resolverErr != nil {
			logger.Warnf("backend config resolver: redis unavailable, env/direct configs only: %v", resolverErr)
		} else {
			logger.Infof("backend config resolver: redis enabled")
		}
		srv.Processor = pipeline.Processor{
			OutputDir:       outputDir,
			HTTPClient:      srv.HTTPClient,
			BackendResolver: resolver,
		}
		http.HandleFunc("/upload", srv.PostFormHandler)
		logger.Infof("postform handler registered at /upload, uploads -> %s, processed -> %s", uploadDir, outputDir)
		if tokenPath != "" {
			logger.Infof("postform default token path -> %s", tokenPath)
		}
	} else {
		logger.Warnf("postform: validator not configured from env: %v", err)
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	addr := os.Getenv("PIXERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	logger.Infof("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Errorf("server stopped: %v", err)
		os.Exit(1)
	}
}
