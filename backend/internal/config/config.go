package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
	"gopkg.in/yaml.v3"
)

type LLMProfile struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	URL      string `yaml:"url"`
	Model    string `yaml:"model"`
	Key      string `yaml:"key"`
}

type LLMConfig struct {
	Default  string       `yaml:"default"`
	Profiles []LLMProfile `yaml:"profiles"`
}

// Config combines non-LLM application environment with the explicit profile
// YAML. Provider credentials are never sourced implicitly from .env.
type Config struct {
	WorkAddr string
	WorkRoot string
	DBPath   string
	LLM      LLMConfig
}

func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}
	v := viper.New()
	v.SetDefault("work_addr", "127.0.0.1:8787")
	v.SetDefault("work_root", defaultWorkRoot())
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv("work_addr", "WORK_ADDR")
	_ = v.BindEnv("work_root", "WORK_ROOT")

	llmConfig, err := loadLLMConfig()
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		WorkAddr: v.GetString("work_addr"),
		WorkRoot: v.GetString("work_root"),
		LLM:      llmConfig,
	}
	cfg.DBPath = filepath.Join(cfg.WorkRoot, "db", "ppt.db")
	return cfg, nil
}

func loadLLMConfig() (LLMConfig, error) {
	path := strings.TrimSpace(os.Getenv("LLM_CONFIG_PATH"))
	if path == "" {
		path = "config.yaml"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LLMConfig{}, fmt.Errorf("LLM profile config %q was not found", path)
		}
		return LLMConfig{}, fmt.Errorf("read LLM profile config: %w", err)
	}
	var document struct {
		LLM LLMConfig `yaml:"llm"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		// Do not include parser excerpts: a malformed line may contain a key.
		return LLMConfig{}, errors.New("LLM profile config contains invalid YAML")
	}
	if err := validateLLMConfig(document.LLM); err != nil {
		return LLMConfig{}, err
	}
	return document.LLM, nil
}

func validateLLMConfig(cfg LLMConfig) error {
	if len(cfg.Profiles) == 0 {
		return errors.New("llm.profiles must contain at least one profile")
	}
	seen := make(map[string]struct{}, len(cfg.Profiles))
	for index, profile := range cfg.Profiles {
		label := fmt.Sprintf("llm.profiles[%d]", index)
		trimmedName := strings.TrimSpace(profile.Name)
		if trimmedName == "" {
			return fmt.Errorf("%s.name must not be empty", label)
		}
		if utf8.RuneCountInString(trimmedName) > 80 {
			return fmt.Errorf("%s.name must contain 1 to 80 characters", label)
		}
		for _, char := range profile.Name {
			if unicode.IsControl(char) {
				return fmt.Errorf("%s.name must not contain control characters", label)
			}
		}
		if _, exists := seen[profile.Name]; exists {
			return fmt.Errorf("llm profile names must be unique: %q", profile.Name)
		}
		seen[profile.Name] = struct{}{}
		switch profile.Provider {
		case "deepseek", "kimi", "openai":
		default:
			return fmt.Errorf("MODEL_PROVIDER_UNSUPPORTED: %s.provider is unsupported", label)
		}
		if err := validateProfileURL(profile.URL); err != nil {
			return fmt.Errorf("%s.url is invalid", label)
		}
		if strings.TrimSpace(profile.Model) == "" {
			return fmt.Errorf("%s.model must not be empty", label)
		}
		if strings.TrimSpace(profile.Key) == "" {
			return fmt.Errorf("%s.key must not be empty", label)
		}
	}
	if _, ok := seen[cfg.Default]; !ok {
		return errors.New("llm.default must exactly match one configured profile name")
	}
	return nil
}

func validateProfileURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() == false || parsed.Host == "" || parsed.User != nil {
		return errors.New("invalid URL")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		if host == "localhost" || (ip != nil && ip.IsLoopback()) {
			return nil
		}
	}
	return errors.New("URL must use https or local development http")
}

func defaultWorkRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".dasi", "ppt")
	}
	return filepath.Join(home, ".dasi", "ppt")
}

func loadDotEnv() error {
	if err := gotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
