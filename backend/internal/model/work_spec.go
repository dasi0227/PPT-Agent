package model

import (
	"errors"
	"fmt"
	"strings"
)

type Artifact string

const (
	ArtifactBlueprint    Artifact = "blueprint"
	ArtifactPresentation Artifact = "presentation"
)

type TargetLevel string

const (
	TargetSlide TargetLevel = "slide"
	TargetDeck  TargetLevel = "deck"
)

type InteractionIntent string

const (
	IntentApply   InteractionIntent = "apply"
	IntentConsult InteractionIntent = "consult"
)

type ClarificationPolicy string

const (
	ClarifyWhenBlocked ClarificationPolicy = "when_blocked"
	ClarifyBeforeApply ClarificationPolicy = "before_apply"
	ClarifyNever       ClarificationPolicy = "never"
)

type RunTarget struct {
	Artifact Artifact    `json:"artifact"`
	Level    TargetLevel `json:"level"`
	SlideID  string      `json:"slide_id,omitempty"`
}

type RunInteraction struct {
	Intent        InteractionIntent   `json:"intent"`
	Clarification ClarificationPolicy `json:"clarification"`
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
	if s.Target.Artifact != ArtifactBlueprint && s.Target.Artifact != ArtifactPresentation {
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
	if s.Interaction.Intent != IntentApply && s.Interaction.Intent != IntentConsult {
		return fmt.Errorf("%w: unsupported intent %q", ErrInvalidWorkSpec, s.Interaction.Intent)
	}
	if s.Interaction.Clarification == "" {
		s.Interaction.Clarification = ClarifyWhenBlocked
	}
	switch s.Interaction.Clarification {
	case ClarifyWhenBlocked, ClarifyBeforeApply, ClarifyNever:
	default:
		return fmt.Errorf("%w: unsupported clarification %q", ErrInvalidWorkSpec, s.Interaction.Clarification)
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
	MaterializationBlueprintStale  MaterializationState = "blueprint_stale"
	MaterializationDesignStale     MaterializationState = "design_stale"
	MaterializationUnknown         MaterializationState = "unknown"
)

type MaterializationRevisions struct {
	Presentation   int `json:"presentation"`
	Deck           int `json:"source_deck"`
	SlideBlueprint int `json:"source_blueprint"`
	Design         int `json:"source_design"`
}

func DeriveMaterializationState(hasHTML bool, currentDeck, currentBlueprint, currentDesign int, source MaterializationRevisions) MaterializationState {
	if !hasHTML {
		return MaterializationNotMaterialized
	}
	if source.Presentation == 0 || source.SlideBlueprint == 0 || source.Design == 0 {
		return MaterializationUnknown
	}
	if source.Deck < currentDeck || source.SlideBlueprint < currentBlueprint {
		return MaterializationBlueprintStale
	}
	if source.Design < currentDesign {
		return MaterializationDesignStale
	}
	return MaterializationFresh
}
