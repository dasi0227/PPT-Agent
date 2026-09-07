package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/subosito/gotenv"
	"gopkg.in/yaml.v3"
)

const (
	listenHost    = "127.0.0.1"
	defaultPort   = "8787"
	llmConfigPath = "config.yaml"
	logLevel      = "debug"
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

// Config combines the single configurable listen port with fixed local runtime
// paths and the backend-local LLM profile YAML.
type Config struct {
	WorkAddr string
	WorkRoot string
	DBPath   string
	LogLevel string
	LLM      LLMConfig
}

func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}
	port, err := loadPort()
	if err != nil {
		return nil, err
	}
	llmConfig, err := loadLLMConfig(llmConfigPath)
	if err != nil {
		return nil, err
	}
	workRoot := defaultWorkRoot()
	cfg := &Config{
		WorkAddr: net.JoinHostPort(listenHost, port),
		WorkRoot: workRoot,
		LogLevel: logLevel,
		LLM:      llmConfig,
	}
	cfg.DBPath = filepath.Join(cfg.WorkRoot, "db", "ppt.db")
	return cfg, nil
}

func loadPort() (string, error) {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return defaultPort, nil
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return "", errors.New("PORT must be an integer between 1 and 65535")
	}
	return strconv.Itoa(value), nil
}

func loadLLMConfig(path string) (LLMConfig, error) {
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
