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

	"gopkg.in/yaml.v3"
)

const (
	listenHost    = "127.0.0.1"
	llmConfigPath = "config.yaml"
	logLevel      = "debug"
)

type LLMProfile struct {
	Name     string `yaml:"name"`
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
	Suggest  string `yaml:"suggest" json:"suggest"`
}

func (s SideRoadLLMConfig) Uses() map[string]string {
	return map[string]string{"rename": s.Rename, "compact": s.Compact, "commit": s.Commit, "polish": s.Polish, "handoff": s.Handoff, "suggest": s.Suggest}
}

type LLMConfig struct {
	Profiles []LLMProfile      `yaml:"llm"`
	MainRoad MainRoadLLMConfig `yaml:"main-road"`
	SideRoad SideRoadLLMConfig `yaml:"side-road"`
}

// FileConfig owns every top-level field in config.yaml.
type FileConfig struct {
	App       AppConfig `yaml:"app"`
	LLMConfig `yaml:",inline"`
	Decision  *DecisionConfig `yaml:"decision,omitempty"`
	Agent     AgentConfig     `yaml:"agent"`
	Sandbox   SandboxConfig   `yaml:"sandbox,omitempty"`
}

type AppConfig struct {
	Port int `yaml:"port"`
}

type AgentConfig struct {
	ShowToolFailures        bool `yaml:"show_tool_failures" json:"show_tool_failures"`
	RequireResourceApproval bool `yaml:"require_resource_approval" json:"require_resource_approval"`
	RequireHighRiskApproval bool `yaml:"require_high_risk_approval" json:"require_high_risk_approval"`
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{RequireResourceApproval: true, RequireHighRiskApproval: true}
}

// Config combines runtime paths and the loaded server configuration.
type Config struct {
	Decision *DecisionConfig
	WorkAddr string
	WorkRoot string
	DBPath   string
	LogLevel string
	LLM      LLMConfig
	LLMPath  string
	Sandbox  SandboxConfig
}

func Load() (*Config, error) {
	return LoadFile(llmConfigPath)
}

func LoadFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("read config.yaml failed")
	}
	fileConfig, err := ParseFileConfig(raw)
	if err != nil {
		return nil, err
	}
	workRoot := defaultWorkRoot()
	cfg := &Config{
		WorkAddr: net.JoinHostPort(listenHost, strconv.Itoa(fileConfig.App.Port)),
		WorkRoot: workRoot,
		LogLevel: logLevel,
		LLM:      fileConfig.LLMConfig,
		Decision: fileConfig.Decision,
		LLMPath:  path,
		Sandbox:  fileConfig.Sandbox,
	}
	cfg.DBPath = filepath.Join(cfg.WorkRoot, "db", "ppt.db")
	return cfg, nil
}

func ParseFileConfig(raw []byte) (FileConfig, error) {
	cfg := FileConfig{Agent: DefaultAgentConfig(), Sandbox: DefaultSandboxConfig()}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, errors.New("config.yaml contains invalid YAML")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, errors.New("config.yaml must contain one YAML document")
	}
	// An explicit null is not the same as an omitted optional block.
	var document yaml.Node
	if yaml.Unmarshal(raw, &document) == nil && len(document.Content) > 0 {
		root := document.Content[0]
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "app" {
				block := root.Content[i+1]
				if block.Kind != yaml.MappingNode {
					return cfg, errors.New("app must be a settings object")
				}
				for j := 0; j+1 < len(block.Content); j += 2 {
					if block.Content[j].Value == "port" && block.Content[j+1].Tag != "!!int" {
						return cfg, errors.New("app.port must be an integer between 1 and 65535")
					}
				}
			}
			if root.Content[i].Value == "sandbox" && root.Content[i+1].Kind != yaml.MappingNode {
				return cfg, errors.New("sandbox must be a settings object")
			}
			if root.Content[i].Value == "agent" {
				block := root.Content[i+1]
				if block.Kind != yaml.MappingNode {
					return cfg, errors.New("agent must be a settings object")
				}
				for j := 1; j < len(block.Content); j += 2 {
					if block.Content[j].Tag != "!!bool" {
						return cfg, errors.New("agent settings must be boolean values")
					}
				}
			}
			if root.Content[i].Value == "decision" {
				block := root.Content[i+1]
				if block.Kind != yaml.MappingNode {
					return cfg, errors.New("decision must be a settings object; omit the block to disable")
				}
				for j := 0; j+1 < len(block.Content); j += 2 {
					if (block.Content[j].Value == "jev" || block.Content[j].Value == "openai") && block.Content[j+1].Kind != yaml.MappingNode {
						return cfg, errors.New("decision provider settings must be objects; omit unused providers")
					}
				}
			}
		}
	}
	if cfg.App.Port < 1 || cfg.App.Port > 65535 {
		return cfg, errors.New("app.port must be an integer between 1 and 65535")
	}
	NormalizeLLMConfig(&cfg.LLMConfig)
	if err := ValidateSandboxConfig(cfg.Sandbox); err != nil {
		return cfg, err
	}
	if err := ValidateDecisionConfig(cfg.Decision); err != nil {
		return cfg, err
	}
	return cfg, validateLLMConfig(cfg.LLMConfig)
}

func NormalizeLLMConfig(cfg *LLMConfig) {
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		p.Name = strings.TrimSpace(p.Name)
		p.Protocol = strings.TrimSpace(p.Protocol)
		p.BaseURL = NormalizeModelBaseURL(p.BaseURL)
		p.Model = strings.TrimSpace(p.Model)
		p.Key = strings.TrimSpace(p.Key)
	}
	fields := []*string{&cfg.MainRoad.Default, &cfg.MainRoad.Fallback, &cfg.SideRoad.Default, &cfg.SideRoad.Fallback, &cfg.SideRoad.Rename, &cfg.SideRoad.Compact, &cfg.SideRoad.Commit, &cfg.SideRoad.Polish, &cfg.SideRoad.Handoff, &cfg.SideRoad.Suggest}
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
	if err := ValidateModelAccess(profile.Protocol, profile.BaseURL); err != nil {
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
