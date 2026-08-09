package model

import (
	"errors"
	"fmt"
	"strings"
)

type Artifact string

const (
	ArtifactSpec         Artifact = "spec"
	ArtifactPresentation Artifact = "presentation"
)

type TargetLevel string

const (
	TargetSlide TargetLevel = "slide"
	TargetDeck  TargetLevel = "deck"
)

type InteractionIntent string

const (
	IntentTalk    InteractionIntent = "talk"
	IntentAsk     InteractionIntent = "ask"
	IntentPlan    InteractionIntent = "plan"
	IntentExecute InteractionIntent = "execute"
)

type RunTarget struct {
	Artifact Artifact    `json:"artifact"`
	Level    TargetLevel `json:"level"`
	SlideID  string      `json:"slide_id,omitempty"`
}

type RunInteraction struct {
	Intent InteractionIntent `json:"intent"`
}

type RunOptions struct {
	Language          string `json:"language,omitempty"`
	ThemeID           string `json:"theme_id,omitempty"`
	DesiredSlideCount int    `json:"desired_slide_count,omitempty"`
}

type WorkSpec struct {
	Target      RunTarget      `json:"target"`
	Interaction RunInteraction `json:"interaction"`
	Instruction string         `json:"instruction"`
	Options     RunOptions     `json:"options,omitempty"`
}

var ErrInvalidWorkSpec = errors.New("invalid work spec")

func (s WorkSpec) Validate() error {
	if s.Target.Artifact != ArtifactSpec && s.Target.Artifact != ArtifactPresentation {
		return fmt.Errorf("%w: unsupported artifact %q", ErrInvalidWorkSpec, s.Target.Artifact)
	}
	if s.Target.Level != TargetSlide && s.Target.Level != TargetDeck {
		return fmt.Errorf("%w: unsupported level %q", ErrInvalidWorkSpec, s.Target.Level)
	}
	if s.Target.Level == TargetSlide && strings.TrimSpace(s.Target.SlideID) == "" {
		return fmt.Errorf("%w: slide_id is required for slide target", ErrInvalidWorkSpec)
	}
	if s.Target.Level == TargetDeck && s.Target.SlideID != "" {
		return fmt.Errorf("%w: slide_id is forbidden for deck target", ErrInvalidWorkSpec)
	}
	if s.Target.SlideID == "current" {
		return fmt.Errorf("%w: current must be resolved to a stable slide_id", ErrInvalidWorkSpec)
	}
	switch s.Interaction.Intent {
	case IntentTalk, IntentAsk, IntentPlan, IntentExecute:
	default:
		return fmt.Errorf("%w: unsupported intent %q", ErrInvalidWorkSpec, s.Interaction.Intent)
	}
	if strings.TrimSpace(s.Instruction) == "" {
		return fmt.Errorf("%w: instruction is required", ErrInvalidWorkSpec)
	}
	return nil
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
