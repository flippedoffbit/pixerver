package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerLevelToggle(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{}
	cfg.Info.Enabled = boolPtr(true)
	cfg.Info.Writer = &buf
	cfg.Debug.Enabled = boolPtr(false)
	Init(cfg)

	Info("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("expected info log to be written")
	}

	buf.Reset()
	Debug("should-not-appear")
	if buf.Len() != 0 {
		t.Fatalf("expected debug to be disabled")
	}
}
