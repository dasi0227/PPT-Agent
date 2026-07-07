package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearEnvForTest(t *testing.T, keys ...string) {
	t.Helper()
	old := make(map[string]*string, len(keys))
	for _, key := range keys {
		if v, ok := os.LookupEnv(key); ok {
			vv := v
			old[key] = &vv
		} else {
			old[key] = nil
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if old[key] == nil {
				_ = os.Unsetenv(key)
				continue
			}
			_ = os.Setenv(key, *old[key])
		}
	})
}

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
	clearEnvForTest(t, "WORK_ADDR", "WORK_ROOT", "DEEPSEEK_API_KEY", "DEEPSEEK_BASE_URL", "DEEPSEEK_MODEL")
	writeEnvFile(t, dir, "WORK_ADDR=127.0.0.1:9999\nWORK_ROOT=./runtime-data\nDEEPSEEK_API_KEY=test-key\nDEEPSEEK_BASE_URL=https://api.deepseek.com\nDEEPSEEK_MODEL=deepseek-chat\n")
	chdirForTest(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.WorkAddr != "127.0.0.1:9999" {
		t.Fatalf("want work addr from .env, got %q", cfg.WorkAddr)
	}
	if cfg.WorkRoot != "./runtime-data" {
		t.Fatalf("want work_root from .env, got %q", cfg.WorkRoot)
	}
	if cfg.DBPath != filepath.Join("./runtime-data", "db", "ppt.db") {
		t.Fatalf("want db path derived from .env work_root, got %q", cfg.DBPath)
	}
	if cfg.DeepSeekKey != "test-key" {
		t.Fatalf("want deepseek key from .env, got %q", cfg.DeepSeekKey)
	}
}

func TestLoadProcessEnvOverridesDotEnv(t *testing.T) {
	dir := t.TempDir()
	clearEnvForTest(t, "WORK_ADDR")
	writeEnvFile(t, dir, "WORK_ADDR=127.0.0.1:9999\n")
	chdirForTest(t, dir)
	t.Setenv("WORK_ADDR", "127.0.0.1:8788")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.WorkAddr != "127.0.0.1:8788" {
		t.Fatalf("want process env to override .env, got %q", cfg.WorkAddr)
	}
}

func TestLoadDefaultWorkRootUsesUserDirLayout(t *testing.T) {
	dir := t.TempDir()
	clearEnvForTest(t, "WORK_ADDR", "WORK_ROOT", "DEEPSEEK_API_KEY", "DEEPSEEK_BASE_URL", "DEEPSEEK_MODEL")
	chdirForTest(t, dir)
	t.Setenv("HOME", "/Users/tester")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.WorkRoot != filepath.Join("/Users/tester", ".dasi", "ppt") {
		t.Fatalf("want default work_root in home dir, got %q", cfg.WorkRoot)
	}
	if cfg.DBPath != filepath.Join("/Users/tester", ".dasi", "ppt", "db", "ppt.db") {
		t.Fatalf("want default db path under work_root/db, got %q", cfg.DBPath)
	}
}
