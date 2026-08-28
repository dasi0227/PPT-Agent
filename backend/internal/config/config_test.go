package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearEnvForTest(t *testing.T, keys ...string) {
	t.Helper()
	old := make(map[string]*string, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			copy := value
			old[key] = &copy
		}
		_ = os.Unsetenv(key)
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if old[key] == nil {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, *old[key])
			}
		}
	})
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func validConfig(secret string) string {
	return `llm:
  default: Kimi Vision
  profiles:
    - name: Kimi Vision
      provider: kimi
      url: https://api.moonshot.cn/v1
      model: kimi-k3
      key: ` + secret + `
    - name: Kimi Text
      provider: kimi
      url: https://api.moonshot.cn/v1
      model: kimi-k2
      key: another-secret
`
}

func TestLoadReadsDefaultConfigAndKeepsAppEnvSeparate(t *testing.T) {
	dir := t.TempDir()
	clearEnvForTest(t, "LLM_CONFIG_PATH", "WORK_ADDR", "WORK_ROOT", "LOG_LEVEL", "DEEPSEEK_API_KEY")
	chdirForTest(t, dir)
	writeFile(t, filepath.Join(dir, "config.yaml"), validConfig("sk-test-secret"))
	writeFile(t, filepath.Join(dir, ".env"), "WORK_ADDR=127.0.0.1:9999\nWORK_ROOT=./runtime-data\nDEEPSEEK_API_KEY=must-not-be-an-llm-source\n")
	t.Setenv("LLM_CONFIG_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkAddr != "127.0.0.1:9999" || cfg.WorkRoot != "./runtime-data" ||
		cfg.DBPath != filepath.Join("./runtime-data", "db", "ppt.db") || cfg.LogLevel != "debug" {
		t.Fatal("application environment was not preserved")
	}
	if cfg.LLM.Default != "Kimi Vision" || len(cfg.LLM.Profiles) != 2 ||
		cfg.LLM.Profiles[0].Key != "sk-test-secret" {
		t.Fatal("profile YAML was not loaded")
	}
}

func TestLoadReadsLogLevelFromEnv(t *testing.T) {
	dir := t.TempDir()
	clearEnvForTest(t, "LLM_CONFIG_PATH", "LOG_LEVEL")
	chdirForTest(t, dir)
	writeFile(t, filepath.Join(dir, "config.yaml"), validConfig("secret"))
	t.Setenv("LOG_LEVEL", "warn")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LOG_LEVEL was not loaded: %q", cfg.LogLevel)
	}
}

func TestLoadUsesExplicitLLMConfigPath(t *testing.T) {
	dir := t.TempDir()
	clearEnvForTest(t, "LLM_CONFIG_PATH", "WORK_ADDR", "WORK_ROOT")
	chdirForTest(t, dir)
	path := filepath.Join(dir, "profiles.yaml")
	writeFile(t, path, validConfig("secret"))
	t.Setenv("LLM_CONFIG_PATH", path)
	t.Setenv("WORK_ADDR", "127.0.0.1:8788")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkAddr != "127.0.0.1:8788" || cfg.LLM.Default != "Kimi Vision" {
		t.Fatal("explicit configuration was not used")
	}
}

func TestLoadRejectsMissingProfilesInsteadOfFallingBack(t *testing.T) {
	dir := t.TempDir()
	clearEnvForTest(t, "LLM_CONFIG_PATH", "DEEPSEEK_API_KEY")
	chdirForTest(t, dir)
	writeFile(t, filepath.Join(dir, "config.yaml"), "llm:\n  default: anything\n  profiles: []\n")
	t.Setenv("LLM_CONFIG_PATH", "")
	t.Setenv("DEEPSEEK_API_KEY", "legacy-key-must-not-enable-fallback")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("missing profiles did not fail startup: %v", err)
	}
}

func TestLLMConfigValidationAndSecretRedaction(t *testing.T) {
	const secret = "sk-never-leak-config-secret"
	cases := []struct {
		name   string
		config string
	}{
		{"duplicate name", `llm:
  default: Same
  profiles:
    - {name: Same, provider: kimi, url: https://api.moonshot.cn/v1, model: kimi-k3, key: ` + secret + `}
    - {name: Same, provider: openai, url: https://api.openai.com/v1, model: gpt-5, key: other}
`},
		{"blank name", `llm:
  default: " "
  profiles:
    - {name: " ", provider: kimi, url: https://api.moonshot.cn/v1, model: kimi-k3, key: ` + secret + `}
`},
		{"control name", "llm:\n  default: \"bad\\u0001name\"\n  profiles:\n    - {name: \"bad\\u0001name\", provider: kimi, url: https://api.moonshot.cn/v1, model: kimi-k3, key: " + secret + "}\n"},
		{"long name", "llm:\n  default: " + strings.Repeat("名", 81) + "\n  profiles:\n    - {name: " + strings.Repeat("名", 81) + ", provider: kimi, url: https://api.moonshot.cn/v1, model: kimi-k3, key: " + secret + "}\n"},
		{"unknown provider", `llm:
  default: Bad
  profiles:
    - {name: Bad, provider: unknown, url: https://example.com, model: x, key: ` + secret + `}
`},
		{"missing default", `llm:
  default: Missing
  profiles:
    - {name: Present, provider: kimi, url: https://api.moonshot.cn/v1, model: kimi-k3, key: ` + secret + `}
`},
		{"empty model", `llm:
  default: Bad
  profiles:
    - {name: Bad, provider: kimi, url: https://api.moonshot.cn/v1, model: "", key: ` + secret + `}
`},
		{"empty key", `llm:
  default: Bad
  profiles:
    - {name: Bad, provider: kimi, url: https://api.moonshot.cn/v1, model: kimi-k3, key: ""}
`},
		{"invalid url", `llm:
  default: Bad
  profiles:
    - {name: Bad, provider: kimi, url: http://provider.example.com, model: kimi-k3, key: ` + secret + `}
`},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profiles.yaml")
			writeFile(t, path, test.config)
			t.Setenv("LLM_CONFIG_PATH", path)
			_, err := Load()
			if err == nil {
				t.Fatal("expected startup validation error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("configuration error leaked a key: %v", err)
			}
		})
	}
}

func TestLocalHTTPProfileURLIsAllowedForDevelopment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	writeFile(t, path, `llm:
  default: Local
  profiles:
    - {name: Local, provider: openai, url: http://127.0.0.1:8080/v1, model: gpt-5, key: secret}
`)
	t.Setenv("LLM_CONFIG_PATH", path)
	if _, err := Load(); err != nil {
		t.Fatalf("local development URL was rejected: %v", err)
	}
}
