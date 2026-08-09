package model

import (
	"errors"
	"fmt"
	"strings"
)

type Artifact string

const (
	ArtifactSpec Artifact = "spec"
	ArtifactPPT  Artifact = "ppt"
)

type ScopeLevel string

const (
	ScopeSlide ScopeLevel = "slide"
	ScopeDeck  ScopeLevel = "deck"
)

type RunIntent string

const (
	IntentTalk    RunIntent = "talk"
	IntentAsk     RunIntent = "ask"
	IntentPlan    RunIntent = "plan"
	IntentExecute RunIntent = "execute"
)

type RunLanguage string

const (
	LanguageChinese RunLanguage = "zh-CN"
	LanguageEnglish RunLanguage = "en-US"
)

type SlideRange string

const (
	SlideRangeFiveToEight         SlideRange = "5-8"
	SlideRangeNineToFifteen       SlideRange = "9-15"
	SlideRangeSixteenToTwentyFive SlideRange = "16-25"
	SlideRangeTwentySixPlus       SlideRange = "26+"
)

type RunScope struct {
	Artifact Artifact   `json:"artifact"`
	Level    ScopeLevel `json:"level"`
	SlideID  string     `json:"slide_id,omitempty"`
}

type RunOptions struct {
	Language RunLanguage `json:"language,omitempty"`
	Range    SlideRange  `json:"range,omitempty"`
}

type RunCommand struct {
	Scope       RunScope   `json:"scope"`
	Intent      RunIntent  `json:"intent"`
	Instruction string     `json:"instruction"`
	Options     RunOptions `json:"options,omitempty"`
}

var ErrInvalidRunCommand = errors.New("invalid run command")

func (c RunCommand) Validate() error {
	if c.Scope.Artifact != ArtifactSpec && c.Scope.Artifact != ArtifactPPT {
		return fmt.Errorf("%w: unsupported artifact %q", ErrInvalidRunCommand, c.Scope.Artifact)
	}
	if c.Scope.Level != ScopeSlide && c.Scope.Level != ScopeDeck {
		return fmt.Errorf("%w: unsupported level %q", ErrInvalidRunCommand, c.Scope.Level)
	}
	if c.Scope.Level == ScopeSlide && strings.TrimSpace(c.Scope.SlideID) == "" {
		return fmt.Errorf("%w: slide_id is required for slide scope", ErrInvalidRunCommand)
	}
	if c.Scope.Level == ScopeDeck && c.Scope.SlideID != "" {
		return fmt.Errorf("%w: slide_id is forbidden for deck scope", ErrInvalidRunCommand)
	}
	if c.Scope.SlideID == "current" {
		return fmt.Errorf("%w: current must be resolved to a stable slide_id", ErrInvalidRunCommand)
	}
	switch c.Intent {
	case IntentTalk, IntentAsk, IntentPlan, IntentExecute:
	default:
		return fmt.Errorf("%w: unsupported intent %q", ErrInvalidRunCommand, c.Intent)
	}
	if strings.TrimSpace(c.Instruction) == "" {
		return fmt.Errorf("%w: instruction is required", ErrInvalidRunCommand)
	}
	switch c.Options.Language {
	case "", LanguageChinese, LanguageEnglish:
	default:
		return fmt.Errorf("%w: unsupported language %q", ErrInvalidRunCommand, c.Options.Language)
	}
	switch c.Options.Range {
	case "", SlideRangeFiveToEight, SlideRangeNineToFifteen, SlideRangeSixteenToTwentyFive, SlideRangeTwentySixPlus:
	default:
		return fmt.Errorf("%w: unsupported range %q", ErrInvalidRunCommand, c.Options.Range)
	}
	if c.Options.Range != "" && c.Scope.Level != ScopeDeck {
		return fmt.Errorf("%w: range is only valid for deck scope", ErrInvalidRunCommand)
	}
	return nil
}

func (r SlideRange) Contains(count int) bool {
	switch r {
	case SlideRangeFiveToEight:
		return count >= 5 && count <= 8
	case SlideRangeNineToFifteen:
		return count >= 9 && count <= 15
	case SlideRangeSixteenToTwentyFive:
		return count >= 16 && count <= 25
	case SlideRangeTwentySixPlus:
		return count >= 26
	default:
		return false
	}
}

type MaterializationState string

const (
	MaterializationNotMaterialized MaterializationState = "not_materialized"
	MaterializationFresh           MaterializationState = "fresh"
	MaterializationSpecStale       MaterializationState = "spec_stale"
	MaterializationDesignStale     MaterializationState = "design_stale"
	MaterializationUnknown         MaterializationState = "unknown"
)

type MaterializationRevisions struct {
	SlideHTML int `json:"slide_html"`
	Outline   int `json:"source_outline"`
	SlideSpec int `json:"source_spec"`
	Design    int `json:"source_design"`
}
