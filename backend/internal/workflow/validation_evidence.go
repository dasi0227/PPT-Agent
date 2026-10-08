package workflow

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// Authored source checks are the same checks used by resource writes. They do
// not render, change source, or establish visual approval.
func validateEvidenceSource(raw []byte, kind string) error {
	if kind == "html" {
		return spec.ValidateSlideHTML(raw)
	}
	_, err := spec.ParseStrictSourceJSON(raw, kind)
	return err
}

func restoredEvidenceFresh(state *RunState, evidence Evidence) bool {
	if !evidence.Fresh {
		return false
	}
	switch evidence.Kind {
	case "schema", "static":
		ref, err := refForResource(state.pack, evidence.Target)
		if err != nil {
			return false
		}
		raw, _, err := readArtifact(state.projectDir, nil, ref)
		return err == nil && hashBytes(raw) == evidence.SourceHash && validateEvidenceSource(raw, evidence.Target.Part) == nil
	case "render":
		if evidence.Render == nil {
			return false
		}
		proof, err := currentRenderProof(state.pack, state.projectDir, nil, evidence.Render.SlideID, evidence.SourceHash)
		return err == nil && proof == *evidence.Render
	default:
		return false
	}
}

// Refresh missing checks from the current saved bytes rather than asking the
// model to make a cosmetic write. Read failures are runtime blockers; actual
// validation defects are returned separately for focused source repair.
func (r *Runtime) refreshCompletionEvidence(ctx context.Context, input RuntimeInput, state *RunState, changes ChangeSet) (map[string]CompletionIssue, error) {
	issues := map[string]CompletionIssue{}
	if state.mode != model.ModeExecute {
		return issues, nil
	}
	for _, change := range append(append([]ArtifactChange{}, changes.Created...), changes.Updated...) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		target := resourceForArtifact(change.Artifact)
		kind := "schema"
		switch change.Artifact.Kind {
		case ArtifactManifest, ArtifactOutline, ArtifactDesign, ArtifactSlideSpec:
		case ArtifactSlideHTML:
			kind = "static"
		default:
			continue
		}
		if !AllowsArtifact(state.scope, change.Artifact) {
			continue
		}
		raw, _, err := readArtifact(input.ProjectDir, state.tx, change.Artifact)
		if err != nil {
			return nil, fmt.Errorf("read completion source %s: %w", target.Key(), err)
		}
		if hashBytes(raw) != change.AfterHash {
			return nil, fmt.Errorf("completion source %s changed after the recorded write", target.Key())
		}
		if state.ledger.HasFresh(target, change.AfterHash, kind) {
			continue
		}
		if err := validateEvidenceSource(raw, target.Part); err != nil {
			details := resourceValidationDetails(err)
			field, _ := details["field"].(string)
			validationErrors, _ := details["validation_errors"].([]map[string]any)
			issue := CompletionIssue{
				Code: "RESOURCE_CONTENT_INVALID", Summary: target.Key() + ": " + err.Error(),
				Field: field, ValidationErrors: validationErrors,
				NextAction: model.ErrorDefinitionFor("RESOURCE_CONTENT_INVALID").ModelMessage,
			}
			// Semantic reads reject invalid saved source. Do not send the model
			// into a read/edit loop that cannot obtain an editable source stamp.
			// HTML reads expose raw source, so structural defects can be repaired.
			if target.Part == "html" {
				issue.RequiredActions = []RequiredAction{{Tool: "edit_html", Target: target}}
				issue.NextAction = resourceReadAction(target) + " Repair only the reported HTML validation defect; unchanged content needs no rewrite."
			}
			issues[target.Key()] = issue
			continue
		}
		evidence := state.ledger.Record(newEvidence(kind, target, change.AfterHash, map[string]any{"valid": true}))
		recordTrace(input.Trace, state.runID, "evidence.refreshed", map[string]any{"evidence": evidence})
	}
	return issues, ctx.Err()
}
