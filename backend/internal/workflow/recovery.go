package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	ReconcileClean            = "clean"
	ReconcileDirtySameRun     = "dirty_same_run"
	ReconcileExternalModified = "external_modified"
	ReconcileMissingArtifact  = "missing_artifact"
)

type ArtifactReconciliation struct {
	Artifact     ArtifactRef `json:"artifact"`
	Status       string      `json:"status"`
	ExpectedHash string      `json:"expected_hash,omitempty"`
	CurrentHash  string      `json:"current_hash,omitempty"`
}

type RecoverySnapshot struct {
	CheckpointID string                   `json:"checkpoint_id,omitempty"`
	RunID        string                   `json:"run_id"`
	Results      []ArtifactReconciliation `json:"results"`
}

func ReconcileDirectWrites(_ context.Context, projectDir string, checkpoint RuntimeCheckpoint) (RecoverySnapshot, error) {
	if projectDir == "" {
		return RecoverySnapshot{}, errors.New("project dir is required")
	}
	out := RecoverySnapshot{RunID: checkpoint.RunID, Results: []ArtifactReconciliation{}}
	for _, change := range checkpoint.Changes.All() {
		path := artifactRelativePath(change.Artifact)
		result := ArtifactReconciliation{
			Artifact: change.Artifact, ExpectedHash: change.AfterHash,
		}
		raw, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(path)))
		if err != nil {
			if os.IsNotExist(err) {
				result.Status = ReconcileMissingArtifact
				out.Results = append(out.Results, result)
				continue
			}
			return out, err
		}
		result.CurrentHash = hashBytes(raw)
		switch {
		case result.CurrentHash == change.AfterHash:
			if change.Tentative {
				result.Status = ReconcileDirtySameRun
			} else {
				result.Status = ReconcileClean
			}
		case change.BeforeHash != "" && result.CurrentHash == change.BeforeHash:
			result.Status = ReconcileClean
		default:
			result.Status = ReconcileExternalModified
		}
		out.Results = append(out.Results, result)
	}
	return out, nil
}

func artifactRelativePath(ref ArtifactRef) string {
	if ref.Path != "" {
		return ref.Path
	}
	switch ref.Kind {
	case ArtifactDesign:
		return "design.json"
	case ArtifactSlideSpec:
		return model.SlideSpecPath(ref.ID)
	case ArtifactSlideHTML:
		return model.SlideHTMLPath(ref.ID)
	default:
		return "outline.json"
	}
}
