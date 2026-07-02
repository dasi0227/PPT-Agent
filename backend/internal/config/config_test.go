package config

import (
	"os"
	"path/filepath"
	"testing"
)

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func writeEnvFile(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadReadsDotEnvFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "PPT_ENV=production\nPPT_LISTEN_ADDR=127.0.0.1:9999\nPPT_WORK_ROOT=./runtime-data\nDEEPSEEK_API_KEY=test-key\nDEEPSEEK_BASE_URL=https://api.deepseek.com\nDEEPSEEK_MODEL=deepseek-chat\n")
	chdirForTest(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Env != "production" {
		t.Fatalf("want env production, got %q", cfg.Env)
	}
	if cfg.ListenAddr != "127.0.0.1:9999" {
		t.Fatalf("want listen addr from .env, got %q", cfg.ListenAddr)
	}
	if cfg.WorkRoot != "./runtime-data" {
		t.Fatalf("want work_root from .env, got %q", cfg.WorkRoot)
	}
	if cfg.DBPath != filepath.Join("./runtime-data", "ppt.db") {
		t.Fatalf("want db path derived from .env work_root, got %q", cfg.DBPath)
	}
	if cfg.DeepSeekKey != "test-key" {
		t.Fatalf("want deepseek key from .env, got %q", cfg.DeepSeekKey)
	}
}

func TestLoadProcessEnvOverridesDotEnv(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "PPT_LISTEN_ADDR=127.0.0.1:9999\n")
	chdirForTest(t, dir)
	t.Setenv("PPT_LISTEN_ADDR", "127.0.0.1:8788")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:8788" {
		t.Fatalf("want process env to override .env, got %q", cfg.ListenAddr)
	}
}
