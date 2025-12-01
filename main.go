package main

import (
	"net/http"
	"os"

	"pixerver/handlers"
	"pixerver/internal/auth"
	"pixerver/internal/env"
	"pixerver/logger"
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

	// Wire up PostFormHandler if POSTFORM_JWT_SECRET is present.
	// This keeps main non-fatal if env isn't present (demo mode).
	if v, err := auth.NewValidatorFromEnv("POSTFORM_JWT_SECRET", "POSTFORM_JWT_AUDIENCE", "POSTFORM_JWT_ISSUER"); err == nil {
		uploadDir := os.Getenv("POSTFORM_UPLOAD_DIR")
		if uploadDir == "" {
			uploadDir = "uploads"
		}
		srv := handlers.NewServer(v, uploadDir)
		http.HandleFunc("/upload", srv.PostFormHandler)
		logger.Infof("postform handler registered at /upload, uploads -> %s", uploadDir)
	} else {
		logger.Warnf("postform: validator not configured from env: %v", err)
	}
}
