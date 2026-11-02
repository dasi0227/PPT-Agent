package config

import (
	"errors"
	"strings"
)

// DecisionConfig selects one server-only provider for every decision use case.
type DecisionConfig struct {
	Provider string                `yaml:"provider" json:"-"`
	Jev      *JevConfig            `yaml:"jev,omitempty" json:"-"`
	OpenAI   *OpenAIDecisionConfig `yaml:"openai,omitempty" json:"-"`
}

func (DecisionConfig) String() string     { return "DecisionConfig{redacted}" }
func (c DecisionConfig) GoString() string { return c.String() }

func ValidateDecisionConfig(c *DecisionConfig) error {
	if c == nil {
		return nil
	}
	c.Provider = strings.TrimSpace(c.Provider)
	switch c.Provider {
	case "jev":
		if c.Jev == nil {
			return errors.New("decision.jev is required when decision.provider is jev")
		}
	case "openai":
		if c.OpenAI == nil {
			return errors.New("decision.openai is required when decision.provider is openai")
		}
	default:
		return errors.New("decision.provider must be jev or openai")
	}
	if err := ValidateJevConfig(c.Jev); err != nil {
		return err
	}
	return ValidateOpenAIDecisionConfig(c.OpenAI)
}
