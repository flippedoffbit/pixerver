package main

import (
	"os"

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
}
