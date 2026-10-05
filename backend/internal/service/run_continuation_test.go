package service

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func TestContinuationRejectsDivergentArtifactsButAcceptsCompletedDeletion(t *testing.T) {
	ref := workflow.ArtifactRef{Kind: workflow.ArtifactSlideHTML, ID: "sli_1"}
	for _, tc := range []struct {
		name, status, current string
		deleted, want         bool
	}{
		{"committed", workflow.ReconcileClean, "after", false, false},
		{"reverted", workflow.ReconcileClean, "before", false, true},
		{"external", workflow.ReconcileExternalModified, "external", false, true},
		{"unknown_write", "unknown", "after", false, true},
		{"missing", workflow.ReconcileMissingArtifact, "", false, true},
		{"completed_deletion", workflow.ReconcileMissingArtifact, "", true, false},
		{"recreated", workflow.ReconcileClean, "after", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := workflow.RuntimeCheckpoint{}
			if tc.deleted {
				cp.Changes.Deleted = []workflow.ArtifactChange{{Artifact: ref}}
			}
			snapshot := workflow.RecoverySnapshot{Results: []workflow.ArtifactReconciliation{{Artifact: ref, Status: tc.status, CurrentHash: tc.current, ExpectedHash: "after"}}}
			if got := continuationHasArtifactConflict(cp, snapshot); got != tc.want {
				t.Fatalf("conflict=%v want %v", got, tc.want)
			}
		})
	}
}
