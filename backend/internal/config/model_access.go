package config

import (
	"errors"
	"net/url"
	"strings"
)

const (
	ProtocolResponses = "responses"
	ProtocolAnthropic = "anthropic"
)

// ProviderDefinition describes branding and form presets, never adapter selection.
type ProviderDefinition struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	DefaultProtocol string            `json:"default_protocol"`
	BaseURLs        map[string]string `json:"base_urls"`
}

func ModelProviders() []ProviderDefinition {
	return []ProviderDefinition{
		{"openai", "OpenAI", ProtocolResponses, map[string]string{ProtocolResponses: "https://api.openai.com/v1"}},
		{"anthropic", "Anthropic", ProtocolAnthropic, map[string]string{ProtocolAnthropic: "https://api.anthropic.com/v1"}},
		{"deepseek", "DeepSeek", ProtocolAnthropic, map[string]string{ProtocolResponses: "https://api.deepseek.com/v1", ProtocolAnthropic: "https://api.deepseek.com/anthropic/v1"}},
		{"kimi", "Kimi", ProtocolAnthropic, map[string]string{ProtocolAnthropic: "https://api.moonshot.cn/anthropic/v1"}},
		{"custom", "自定义", ProtocolResponses, map[string]string{}},
	}
}

func NormalizeModelBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

// BaseURL is the complete API prefix. Adapters append only /responses or /messages.
func ValidateModelAccess(provider, protocol, baseURL string) error {
	known := false
	for _, item := range ModelProviders() {
		known = known || item.ID == provider
	}
	if !known {
		return errors.New("MODEL_PROVIDER_UNSUPPORTED: provider is unsupported")
	}
	if protocol != ProtocolResponses && protocol != ProtocolAnthropic {
		return errors.New("MODEL_PROTOCOL_UNSUPPORTED: protocol must be responses or anthropic")
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
