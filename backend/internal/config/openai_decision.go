package config

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

// OpenAIDecisionConfig configures the server-only OpenAI Decisions adapter.
type OpenAIDecisionConfig struct {
	BaseURL string `yaml:"base_url" json:"-"`
	Model   string `yaml:"model" json:"-"`
	Key     string `yaml:"key" json:"-"`
}

func (OpenAIDecisionConfig) String() string     { return "OpenAIDecisionConfig{redacted}" }
func (c OpenAIDecisionConfig) GoString() string { return c.String() }
func ValidateOpenAIDecisionConfig(c *OpenAIDecisionConfig) error {
	if c == nil {
		return nil
	}
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model, c.Key = strings.TrimSpace(c.Model), strings.TrimSpace(c.Key)
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.HasSuffix(u.Path, "/decisions") {
		return errors.New("decision.openai.base_url must be an HTTP API prefix without credentials, query, fragment or /decisions")
	}
	if c.Model == "" || c.Key == "" {
		return errors.New("decision.openai requires base_url, model and key")
	}
	for _, value := range []string{c.Model, c.Key} {
		for _, ch := range value {
			if unicode.IsControl(ch) {
				return errors.New("decision.openai.model/key must be single-line values")
			}
		}
	}
	return nil
}
