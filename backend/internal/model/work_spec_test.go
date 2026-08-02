package model

import (
	"testing"
)

func TestWorkSpecValidation(t *testing.T) {
	valid := WorkSpec{
		Target:      RunTarget{Artifact: ArtifactPresentation, Level: TargetSlide, SlideID: "stable"},
		Interaction: RunInteraction{Intent: IntentExecute},
		Instruction: "revise",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	cases := []WorkSpec{
		{Target: RunTarget{Artifact: "generate", Level: TargetDeck}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactSpec, Level: "current"}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactSpec, Level: TargetSlide}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactSpec, Level: TargetDeck, SlideID: "current"}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactSpec, Level: TargetDeck, SlideID: "stable"}, Interaction: valid.Interaction, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactSpec, Level: TargetDeck}, Interaction: RunInteraction{Intent: "consult"}, Instruction: "x"},
		{Target: RunTarget{Artifact: ArtifactSpec, Level: TargetDeck}, Interaction: valid.Interaction, Instruction: "  "},
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
	source := MaterializationRevisions{SlideHTML: 1, Outline: 2, SlideSpec: 2, Design: 2}
	if got := DeriveMaterializationState(true, 2, 3, 2, source); got != MaterializationSpecStale {
		t.Fatalf("spec stale: %s", got)
	}
	source.SlideSpec = 3
	if got := DeriveMaterializationState(true, 3, 3, 2, source); got != MaterializationSpecStale {
		t.Fatalf("deck stale: %s", got)
	}
	source.Outline = 3
	if got := DeriveMaterializationState(true, 3, 3, 3, source); got != MaterializationDesignStale {
		t.Fatalf("design stale: %s", got)
	}
	source.Design = 3
	if got := DeriveMaterializationState(true, 3, 3, 3, source); got != MaterializationFresh {
		t.Fatalf("fresh: %s", got)
	}
}
