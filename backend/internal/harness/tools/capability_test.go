package tools

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestConsultCapabilitiesAreReadOnly(t *testing.T) {
	spec := model.WorkSpec{
		Target:      model.RunTarget{Artifact: model.ArtifactPresentation, Level: model.TargetDeck},
		Interaction: model.RunInteraction{Intent: model.IntentConsult, Clarification: model.ClarifyWhenBlocked},
		Instruction: "advise",
	}
	allowed := AllowedCapabilities(spec)
	for _, capability := range []Capability{CapabilityWriteBlueprint, CapabilityWritePresentation, CapabilityWriteDesignSpec, CapabilityMountAssets} {
		if allowed[capability] {
			t.Errorf("consult unexpectedly allows %s", capability)
		}
	}
	if !allowed[CapabilityReadPresentation] || !allowed[CapabilityReadBlueprint] {
		t.Fatal("consult should retain read access")
	}
}
