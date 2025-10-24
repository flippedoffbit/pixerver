package main

import (
	"pixerver/logger"
)

func main() {
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
