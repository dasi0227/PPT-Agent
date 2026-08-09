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

func TestWorkSpecValidationAcceptsPlanIntent(t *testing.T) {
	spec := WorkSpec{
		Target:      RunTarget{Artifact: ArtifactSpec, Level: TargetDeck},
		Interaction: RunInteraction{Intent: IntentPlan},
		Instruction: "plan the work",
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("plan intent rejected: %v", err)
	}
}
