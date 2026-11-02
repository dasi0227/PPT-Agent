package decision

import "github.com/dasi0227/PPT-Agent/backend/internal/config"

// NewSnapshot pins the selected provider and credentials for all decision calls.
// An omitted decision block disables decisions; providers never fall back to one another.
func NewSnapshot(cfg *config.DecisionConfig) (Snapshot, error) {
	if cfg == nil {
		return Snapshot{}, nil
	}
	value := *cfg
	if cfg.Jev != nil {
		jev := *cfg.Jev
		value.Jev = &jev
	}
	if cfg.OpenAI != nil {
		openai := *cfg.OpenAI
		value.OpenAI = &openai
	}
	if err := config.ValidateDecisionConfig(&value); err != nil {
		return Snapshot{}, err
	}
	if value.Provider == "openai" {
		return newOpenAISnapshot(value.OpenAI)
	}
	return newJevSnapshot(value.Jev)
}
