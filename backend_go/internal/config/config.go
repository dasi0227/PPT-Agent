package config

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ListenHost = "127.0.0.1"
	FilePath   = "config.yaml"
)

// UserWorkRoot 强制固定使用用户主目录下的 .dasi/ppt，不接受配置文件或命令行覆盖
func UserWorkRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dasi", "ppt"), nil
}

// LLMProfile 记录大语言模型的调用配置，其中 name 是用户自定义的标识
type LLMProfile struct {
	Name     string `yaml:"name" json:"-"`
	Protocol string `yaml:"protocol" json:"-"`
	BaseURL  string `yaml:"base_url" json:"-"`
	Model    string `yaml:"model" json:"-"`
	Key      string `yaml:"key" json:"-"`
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

// JevConfig 记录必填的 JEV 调用配置
type JevConfig struct {
	BaseURL string `yaml:"base_url" json:"-"`
	Model   string `yaml:"model" json:"-"`
	Key     string `yaml:"key" json:"-"`
}

// LLMConfig 记录所有与模型相关的配置
type LLMConfig struct {
	Profiles []LLMProfile      `yaml:"profiles" json:"-"`
	MainRoad MainRoadLLMConfig `yaml:"main-road" json:"-"`
	SideRoad SideRoadLLMConfig `yaml:"side-road" json:"-"`
	Jev      JevConfig         `yaml:"jev" json:"-"`
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

	cfg.Jev.BaseURL = NormalizeModelBaseURL(cfg.Jev.BaseURL)
	cfg.Jev.Model = strings.TrimSpace(cfg.Jev.Model)
	cfg.Jev.Key = strings.TrimSpace(cfg.Jev.Key)
}

// AppConfig 记录应用的运行配置
type AppConfig struct {
	Port     int    `yaml:"port" json:"-"`
	LogLevel string `yaml:"log_level" json:"-"`
}

func DefaultAppConfig() AppConfig {
	return AppConfig{
		Port:     8787,
		LogLevel: "debug",
	}
}

// AgentConfig 记录智能体执行相关的配置
type AgentConfig struct {
	AutoRenameEnabled       bool `yaml:"auto_rename_enabled" json:"auto_rename_enabled"`
	AutoSuggestEnabled      bool `yaml:"auto_suggest_enabled" json:"auto_suggest_enabled"`
	ShowToolFailures        bool `yaml:"show_tool_failures" json:"show_tool_failures"`
	RequireResourceApproval bool `yaml:"require_resource_approval" json:"require_resource_approval"`
	RequireHighRiskApproval bool `yaml:"require_high_risk_approval" json:"require_high_risk_approval"`
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		AutoRenameEnabled:       true,
		AutoSuggestEnabled:      true,
		RequireResourceApproval: true,
		RequireHighRiskApproval: true,
	}
}

// SandboxConfig 记录命令沙箱策略
type SandboxConfig struct {
	ReadRoots []string               `yaml:"read_roots" json:"read_roots"`
	Network   SandboxNetworkConfig   `yaml:"network" json:"network"`
	Execution SandboxExecutionConfig `yaml:"execution" json:"execution"`
}

type SandboxNetworkConfig struct {
	AllowedDomains []string `yaml:"allowed_domains" json:"allowed_domains"`
}

type SandboxExecutionConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
	MaxOutputBytes int `yaml:"max_output_bytes" json:"max_output_bytes"`
}

func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		ReadRoots: []string{}, Network: SandboxNetworkConfig{AllowedDomains: []string{}},
		Execution: SandboxExecutionConfig{TimeoutSeconds: 120, MaxOutputBytes: 8 << 20},
	}
}

// RuntimeConfig 记录所有运行时配置
type RuntimeConfig struct {
	App        AppConfig     `yaml:"app" json:"-"`
	LLM        LLMConfig     `yaml:"llm" json:"-"`
	Agent      AgentConfig   `yaml:"agent" json:"-"`
	Sandbox    SandboxConfig `yaml:"sandbox" json:"-"`
	WorkAddr   string        `yaml:"-" json:"-"`
	WorkRoot   string        `yaml:"-" json:"-"`
	DBPath     string        `yaml:"-" json:"-"`
	ConfigPath string        `yaml:"-" json:"-"`
}

func LoadConfig() (*RuntimeConfig, error) {
	path, err := filepath.Abs(FilePath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := ParseConfigFile(raw)
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = path
	cfg.WorkAddr = net.JoinHostPort(ListenHost, strconv.Itoa(cfg.App.Port))
	cfg.WorkRoot, err = UserWorkRoot()
	if err != nil {
		return nil, err
	}
	cfg.DBPath = filepath.Join(cfg.WorkRoot, "db", "ppt.db")
	return &cfg, nil
}

// ParseConfigFile 读取唯一的 YAML 文档，并按配置块校验
func ParseConfigFile(raw []byte) (RuntimeConfig, error) {
	// 初始化配置
	cfg := RuntimeConfig{
		App:     DefaultAppConfig(),
		Agent:   DefaultAgentConfig(),
		Sandbox: DefaultSandboxConfig(),
	}

	// 解析 YAML
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, errors.New("config.yaml is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, errors.New("config.yaml contains more than one document")
	}
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil || len(document.Content) == 0 {
		return cfg, errors.New("config.yaml is invalid")
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return cfg, errors.New("config.yaml must be a settings object")
	}

	// 解析每个顶层字段
	blocks := make(map[string]*yaml.Node, len(root.Content)/2)
	for i := 0; i+1 < len(root.Content); i += 2 {
		blocks[root.Content[i].Value] = root.Content[i+1]
	}

	for _, field := range []string{"app", "llm", "agent", "sandbox"} {
		block := blocks[field]
		var err error
		switch field {
		case "app":
			err = validateApp(block, &cfg.App)
		case "llm":
			err = validateLLM(block, &cfg.LLM)
		case "agent":
			err = validateAgent(block)
		case "sandbox":
			err = validateSandbox(block, cfg.Sandbox)
		}
		if err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func NormalizeModelBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func InferModelProvider(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(model, "deepseek"):
		return "deepseek"
	case strings.Contains(model, "kimi"):
		return "kimi"
	case strings.Contains(model, "mimo"):
		return "mimo"
	case strings.Contains(model, "gpt"):
		return "openai"
	case strings.Contains(model, "claude"):
		return "anthropic"
	case strings.Contains(model, "gemini"):
		return "gemini"
	case strings.Contains(model, "qwen"):
		return "qwen"
	case strings.Contains(model, "minimax"):
		return "minimax"
	case strings.Contains(model, "glm"):
		return "zai"
	default:
		return "custom"
	}
}
