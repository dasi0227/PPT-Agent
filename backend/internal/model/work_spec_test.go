package model

import (
	"testing"
)

func TestWorkSpecValidation(t *testing.T) {
	valid := WorkSpec{
		Target:      RunTarget{Artifact: ArtifactPresentation, Level: TargetSlide, SlideID: "stable"},
		Interaction: RunInteraction{Intent: IntentApply, Clarification: ClarifyWhenBlocked},
		Instruction: "revise",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	cases := []WorkSpec{
		{Target: RunTarget{Artifact: "generate", Level: TargetDeck}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactBlueprint, Level: "current"}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactBlueprint, Level: TargetSlide}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactBlueprint, Level: TargetDeck, SlideID: "current"}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactBlueprint, Level: TargetDeck, SlideID: "stable"}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactBlueprint, Level: TargetDeck}, Interaction: RunInteraction{Intent: "talk", Clarification: ClarifyNever}, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactBlueprint, Level: TargetDeck}, Interaction: RunInteraction{Intent: IntentApply, Clarification: "always"}, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactBlueprint, Level: TargetDeck}, Interaction: valid.Interaction, Instruction: "  "},
	}
	for i, spec := range cases {
		if err := spec.Validate(); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
}

func TestDeriveMaterializationState(t *testing.T) {
	if got := DeriveMaterializationState(false, 2, 2, 2, MaterializationRevisions{}); got != MaterializationNotMaterialized {
		t.Fatalf("not materialized: %s", got)
	}
	source := MaterializationRevisions{Presentation: 1, Deck: 2, SlideBlueprint: 2, Design: 2}
	if got := DeriveMaterializationState(true, 2, 3, 2, source); got != MaterializationBlueprintStale {
		t.Fatalf("blueprint stale: %s", got)
	}
	source.SlideBlueprint = 3
	if got := DeriveMaterializationState(true, 3, 3, 2, source); got != MaterializationBlueprintStale {
		t.Fatalf("deck stale: %s", got)
	}
	source.Deck = 3
	if got := DeriveMaterializationState(true, 3, 3, 3, source); got != MaterializationDesignStale {
		t.Fatalf("design stale: %s", got)
	}
	source.Design = 3
	if got := DeriveMaterializationState(true, 3, 3, 3, source); got != MaterializationFresh {
		t.Fatalf("fresh: %s", got)
	}
}
