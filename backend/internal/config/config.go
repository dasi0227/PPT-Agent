package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
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
	Protocol string `yaml:"protocol"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
	Key      string `yaml:"key"`
}

type MainRoadLLMConfig struct {
	Default  string `yaml:"default" json:"default"`
	Fallback string `yaml:"fallback" json:"fallback"`
}

type SideRoadLLMConfig struct {
	Default  string `yaml:"default" json:"default"`
	Fallback string `yaml:"fallback" json:"fallback"`
	Rename   string `yaml:"rename" json:"rename"`
	Compact  string `yaml:"compact" json:"compact"`
	Commit   string `yaml:"commit" json:"commit"`
	Polish   string `yaml:"polish" json:"polish"`
	Handoff  string `yaml:"handoff" json:"handoff"`
	Kickoff  string `yaml:"kickoff" json:"kickoff"`
}

func (s SideRoadLLMConfig) Uses() map[string]string {
	return map[string]string{"rename": s.Rename, "compact": s.Compact, "commit": s.Commit, "polish": s.Polish, "handoff": s.Handoff, "kickoff": s.Kickoff}
}

type LLMConfig struct {
	Profiles []LLMProfile      `yaml:"llm"`
	MainRoad MainRoadLLMConfig `yaml:"main-road"`
	SideRoad SideRoadLLMConfig `yaml:"side-road"`
}

// Config combines the single configurable listen port with fixed local runtime
// paths and the backend-local LLM profile YAML.
type Config struct {
	WorkAddr string
	WorkRoot string
	DBPath   string
	LogLevel string
	LLM      LLMConfig
	LLMPath  string
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
		LLMPath:  llmConfigPath,
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
	return ParseLLMConfig(raw)
}

func ParseLLMConfig(raw []byte) (LLMConfig, error) {
	var cfg LLMConfig
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, errors.New("LLM profile config contains invalid YAML")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, errors.New("LLM profile config must contain one YAML document")
	}
	NormalizeLLMConfig(&cfg)
	return cfg, validateLLMConfig(cfg)
}

func NormalizeLLMConfig(cfg *LLMConfig) {
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		p.Name = strings.TrimSpace(p.Name)
		p.Provider = strings.TrimSpace(p.Provider)
		p.Protocol = strings.TrimSpace(p.Protocol)
		p.BaseURL = NormalizeModelBaseURL(p.BaseURL)
		p.Model = strings.TrimSpace(p.Model)
		p.Key = strings.TrimSpace(p.Key)
	}
	fields := []*string{&cfg.MainRoad.Default, &cfg.MainRoad.Fallback, &cfg.SideRoad.Default, &cfg.SideRoad.Fallback, &cfg.SideRoad.Rename, &cfg.SideRoad.Compact, &cfg.SideRoad.Commit, &cfg.SideRoad.Polish, &cfg.SideRoad.Handoff, &cfg.SideRoad.Kickoff}
	for _, field := range fields {
		*field = strings.TrimSpace(*field)
	}
}

func ValidateLLMConfig(cfg LLMConfig) error { return validateLLMConfig(cfg) }
func validateLLMConfig(cfg LLMConfig) error {
	if len(cfg.Profiles) == 0 {
		return errors.New("llm must contain at least one profile")
	}
	seen := make(map[string]bool, len(cfg.Profiles))
	for i, profile := range cfg.Profiles {
		if err := validateLLMProfile(profile, fmt.Sprintf("llm[%d]", i)); err != nil {
			return err
		}
		if seen[profile.Name] {
			return fmt.Errorf("llm[%d].name must be unique", i)
		}
		seen[profile.Name] = true
	}
	refs := map[string]string{"main-road.default": cfg.MainRoad.Default, "side-road.default": cfg.SideRoad.Default}
	for path, name := range refs {
		if !seen[name] {
			return fmt.Errorf("%s must reference a configured model", path)
		}
	}
	refs = map[string]string{"main-road.fallback": cfg.MainRoad.Fallback, "side-road.fallback": cfg.SideRoad.Fallback}
	for purpose, name := range cfg.SideRoad.Uses() {
		refs["side-road."+purpose] = name
	}
	for path, name := range refs {
		if name != "" && !seen[name] {
			return fmt.Errorf("%s must reference a configured model", path)
		}
	}
	return nil
}

func validateLLMProfile(profile LLMProfile, label string) error {
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
	if err := ValidateModelAccess(profile.Provider, profile.Protocol, profile.BaseURL); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if strings.TrimSpace(profile.Model) == "" {
		return fmt.Errorf("%s.model must not be empty", label)
	}
	if strings.TrimSpace(profile.Key) == "" {
		return fmt.Errorf("%s.key must not be empty", label)
	}
	for _, value := range []string{profile.Model, profile.Key} {
		for _, char := range value {
			if unicode.IsControl(char) {
				return fmt.Errorf("%s.model/key must be single-line values", label)
			}
		}
	}
	return nil
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
