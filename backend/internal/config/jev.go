package config

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

// JevConfig is server-only and never part of public model settings.
type JevConfig struct {
	BaseURL string `yaml:"base_url" json:"-"`
	Model   string `yaml:"model" json:"-"`
	Key     string `yaml:"key" json:"-"`
}

func (c JevConfig) String() string   { return "JevConfig{redacted}" }
func (c JevConfig) GoString() string { return c.String() }

func ValidateJevConfig(c *JevConfig) error {
	if c == nil {
		return nil
	}
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model, c.Key = strings.TrimSpace(c.Model), strings.TrimSpace(c.Key)
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.HasSuffix(u.Path, "/systemone") {
		return errors.New("jev.base_url must be an HTTP API prefix without credentials, query, fragment or /systemone")
	}
	if c.Model == "" || c.Key == "" {
		return errors.New("jev requires base_url, model and key")
	}
	for _, value := range []string{c.Model, c.Key} {
		for _, ch := range value {
			if unicode.IsControl(ch) {
				return errors.New("jev.model/key must be single-line values")
			}
		}
	}
	return nil
}
