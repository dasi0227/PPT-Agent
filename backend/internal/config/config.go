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

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

const (
	listenHost = "127.0.0.1"
	configPath = "config.yaml"
)

// LLMProfile 记录不同模型的调用信息，其中 name 是用户自定义的标识
type LLMProfile struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
	Key      string `yaml:"key"`
}

// MainRoadLLMConfig 记录主路对话使用的模型，值为 LLMProfile 中的 name 标识
type MainRoadLLMConfig struct {
	Default  string `yaml:"default" json:"default"`
	Fallback string `yaml:"fallback" json:"fallback"`
}

// SideRoadLLMConfig 记录旁路对话使用的模型，值为 LLMProfile 中的 name 标识，支持根据不同类型操作划分
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

// ModelsByPurpose 负责返回具体操作类型对应的模型名称
func (s SideRoadLLMConfig) ModelsByPurpose() map[string]string {
	return map[string]string{
		"rename":  s.Rename,
		"compact": s.Compact,
		"commit":  s.Commit,
		"polish":  s.Polish,
		"handoff": s.Handoff,
		"suggest": s.Suggest,
	}
}

// LLMConfig 记录所有与模型相关的配置
type LLMConfig struct {
	Profiles []LLMProfile      `yaml:"profiles"`
	MainRoad MainRoadLLMConfig `yaml:"main-road"`
	SideRoad SideRoadLLMConfig `yaml:"side-road"`
	Jev      *JevConfig        `yaml:"jev,omitempty" json:"-"`
}

type AppConfig struct {
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`
}

type AgentConfig struct {
	AutoRenameEnabled       bool `yaml:"auto_rename_enabled" json:"auto_rename_enabled"`
	AutoSuggestEnabled      bool `yaml:"auto_suggest_enabled" json:"auto_suggest_enabled"`
	ShowToolFailures        bool `yaml:"show_tool_failures" json:"show_tool_failures"`
	RequireResourceApproval bool `yaml:"require_resource_approval" json:"require_resource_approval"`
	RequireHighRiskApproval bool `yaml:"require_high_risk_approval" json:"require_high_risk_approval"`
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{AutoRenameEnabled: true, AutoSuggestEnabled: true, RequireResourceApproval: true, RequireHighRiskApproval: true}
}

// RuntimeConfig 统一保存 YAML 配置与加载后生成的运行参数。
type RuntimeConfig struct {
	App     AppConfig     `yaml:"app"`
	LLM     LLMConfig     `yaml:"llm"`
	Agent   AgentConfig   `yaml:"agent"`
	Sandbox SandboxConfig `yaml:"sandbox,omitempty"`

	// 派生字段在加载时计算，不参与 YAML 读写。
	WorkAddr   string `yaml:"-" json:"-"`
	WorkRoot   string `yaml:"-" json:"-"`
	DBPath     string `yaml:"-" json:"-"`
	ConfigPath string `yaml:"-" json:"-"`
}

func Load() (*RuntimeConfig, error) {
	return LoadFile(configPath)
}

func LoadFile(path string) (*RuntimeConfig, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("read config.yaml failed")
	}
	cfg, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = path
	cfg.WorkAddr = net.JoinHostPort(listenHost, strconv.Itoa(cfg.App.Port))
	cfg.WorkRoot, err = UserWorkRoot()
	if err != nil {
		return nil, err
	}
	cfg.DBPath = filepath.Join(cfg.WorkRoot, "db", "ppt.db")
	return &cfg, nil
}

// UserWorkRoot 固定使用用户主目录下的 .dasi/ppt，不接受配置文件或命令行覆盖。
func UserWorkRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dasi", "ppt"), nil
}

// Parse 读取并校验唯一的配置结构；文件路径等派生参数由 LoadFile 填充。
func Parse(raw []byte) (RuntimeConfig, error) {
	cfg := RuntimeConfig{
		Agent: DefaultAgentConfig(), Sandbox: DefaultSandboxConfig(),
	}
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
					if block.Content[j].Value == "log_level" && block.Content[j+1].Tag != "!!str" {
						return cfg, errors.New("app.log_level must be a string")
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
			if root.Content[i].Value == "llm" {
				block := root.Content[i+1]
				if block.Kind != yaml.MappingNode {
					return cfg, errors.New("llm must be a settings object")
				}
				for j := 0; j+1 < len(block.Content); j += 2 {
					if block.Content[j].Value == "jev" && block.Content[j+1].Kind != yaml.MappingNode {
						return cfg, errors.New("llm.jev must be a settings object; omit the block to disable")
					}
				}
			}
		}
	}
	if cfg.App.Port < 1 || cfg.App.Port > 65535 {
		return cfg, errors.New("app.port must be an integer between 1 and 65535")
	}
	cfg.App.LogLevel = strings.ToLower(strings.TrimSpace(cfg.App.LogLevel))
	if cfg.App.LogLevel == "" {
		return cfg, errors.New("app.log_level is required")
	}
	var level zapcore.Level
	if err := level.Set(cfg.App.LogLevel); err != nil {
		return cfg, errors.New("app.log_level is invalid")
	}
	NormalizeLLMConfig(&cfg.LLM)
	if err := ValidateSandboxConfig(cfg.Sandbox); err != nil {
		return cfg, err
	}
	return cfg, validateLLMConfig(cfg.LLM)
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
		return errors.New("llm.profiles must contain at least one profile")
	}
	seen := make(map[string]bool, len(cfg.Profiles))
	for i, profile := range cfg.Profiles {
		if err := validateLLMProfile(profile, fmt.Sprintf("llm.profiles[%d]", i)); err != nil {
			return err
		}
		if seen[profile.Name] {
			return fmt.Errorf("llm.profiles[%d].name must be unique", i)
		}
		seen[profile.Name] = true
	}
	refs := map[string]string{"llm.main-road.default": cfg.MainRoad.Default, "llm.side-road.default": cfg.SideRoad.Default}
	for path, name := range refs {
		if !seen[name] {
			return fmt.Errorf("%s must reference a configured model", path)
		}
	}
	refs = map[string]string{"llm.main-road.fallback": cfg.MainRoad.Fallback, "llm.side-road.fallback": cfg.SideRoad.Fallback}
	for purpose, name := range cfg.SideRoad.ModelsByPurpose() {
		refs["llm.side-road."+purpose] = name
	}
	for path, name := range refs {
		if name != "" && !seen[name] {
			return fmt.Errorf("%s must reference a configured model", path)
		}
	}
	return ValidateJevConfig(cfg.Jev)
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
