package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBasicEnv(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, ".env")
	content := `# sample
KEY1=val1
KEY2="quoted val"
ESC=hello\nworld
`
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	m, err := Load(fpath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if m["KEY1"] != "val1" {
		t.Fatalf("KEY1 mismatch: %v", m["KEY1"])
	}
	if m["KEY2"] != "quoted val" {
		t.Fatalf("KEY2 mismatch: %v", m["KEY2"])
	}
	if m["ESC"] != "hello\nworld" {
		t.Fatalf("ESC mismatch: %q", m["ESC"])
	}

	// env vars should also be set in process env
	if os.Getenv("KEY1") != "val1" {
		t.Fatalf("os.Getenv KEY1 mismatch: %v", os.Getenv("KEY1"))
	}

	// cleanup
	_ = os.Unsetenv("KEY1")
	_ = os.Unsetenv("KEY2")
	_ = os.Unsetenv("ESC")
}
