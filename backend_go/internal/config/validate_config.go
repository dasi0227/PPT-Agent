package config

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

const (
	ProtocolResponses = "responses"
	ProtocolAnthropic = "anthropic"
)

func validateApp(block *yaml.Node, cfg *AppConfig) error {
	var port, logLevel *yaml.Node
	if block != nil {
		if block.Kind != yaml.MappingNode {
			return errors.New("app is not a object")
		}
		for i := 0; i+1 < len(block.Content); i += 2 {
			switch block.Content[i].Value {
			case "port":
				port = block.Content[i+1]
			case "log_level":
				logLevel = block.Content[i+1]
			}
		}
	}
	if (port != nil && port.Tag != "!!int") || cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("app.port is not an integer between 1 and 65535")
	}
	if logLevel != nil && logLevel.Tag != "!!str" {
		return errors.New("app.log_level is not a string")
	}
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	if cfg.LogLevel == "" {
		return errors.New("app.log_level is required")
	}
	var level zapcore.Level
	if err := level.Set(cfg.LogLevel); err != nil {
		return errors.New("app.log_level is invalid")
	}
	return nil
}

func validateLLM(block *yaml.Node, cfg *LLMConfig) error {
	if block == nil {
		return errors.New("llm is required")
	}
	if block.Kind != yaml.MappingNode {
		return errors.New("llm is not a object")
	}
	NormalizeLLMConfig(cfg)
	return ValidateLLMConfig(*cfg)
}

func validateAgent(block *yaml.Node) error {
	if block == nil {
		return nil
	}
	if block.Kind != yaml.MappingNode {
		return errors.New("agent is not a object")
	}
	for i := 0; i+1 < len(block.Content); i += 2 {
		if block.Content[i+1].Tag != "!!bool" {
			return fmt.Errorf("agent.%s is not a boolean", block.Content[i].Value)
		}
	}
	return nil
}

func validateSandbox(block *yaml.Node, cfg SandboxConfig) error {
	if block != nil && block.Kind != yaml.MappingNode {
		return errors.New("sandbox is not a object")
	}
	return ValidateSandboxConfig(cfg)
}

func ValidateSandboxConfig(cfg SandboxConfig) error {
	if cfg.Execution.TimeoutSeconds < 1 || cfg.Execution.TimeoutSeconds > 900 {
		return errors.New("sandbox.execution.timeout_seconds must be between 1 and 900")
	}
	if cfg.Execution.MaxOutputBytes < 1024 || cfg.Execution.MaxOutputBytes > 64<<20 {
		return errors.New("sandbox.execution.max_output_bytes must be between 1024 and 67108864")
	}
	for _, root := range cfg.ReadRoots {
		if !filepath.IsAbs(root) || filepath.Clean(root) == "/" || strings.ContainsRune(root, 0) {
			return errors.New("sandbox.read_roots must contain absolute paths below the filesystem root")
		}
	}
	for _, domain := range cfg.Network.AllowedDomains {
		if domain == "" || strings.TrimSpace(domain) != domain {
			return errors.New("sandbox.network.allowed_domains must contain nonempty domain names")
		}
	}
	return nil
}

func ValidateLLMConfig(cfg LLMConfig) error {
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
	return ValidateJevConfig(&cfg.Jev)
}

func ValidateJevConfig(c *JevConfig) error {
	if c == nil {
		return errors.New("llm.jev is required")
	}
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model, c.Key = strings.TrimSpace(c.Model), strings.TrimSpace(c.Key)
	if c.BaseURL == "" || c.Model == "" || c.Key == "" {
		return errors.New("llm.jev requires base_url, model and key")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.HasSuffix(u.Path, "/systemone") {
		return errors.New("llm.jev.base_url must be an HTTP API prefix without credentials, query, fragment or /systemone")
	}
	for _, value := range []string{c.Model, c.Key} {
		for _, ch := range value {
			if unicode.IsControl(ch) {
				return errors.New("llm.jev.model/key must be single-line values")
			}
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

func ValidateModelAccess(protocol, baseURL string) error {
	if protocol != ProtocolResponses && protocol != ProtocolAnthropic {
		return errors.New("protocol must be responses or anthropic")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u == nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(baseURL, " \t\r\n\\") {
		return errors.New("base_url must be an HTTP(S) API base address without credentials, query or fragment")
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/chat/completions") {
		return errors.New("base_url must be the API prefix, without /responses, /messages or /chat/completions")
	}
	return nil
}
