package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestResumePreservesValidationEvidenceAndRejectsStaleProofs(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	state := &RunState{pack: pack, projectDir: dir}
	refs := []ArtifactRef{manifestRef(pack), outlineRef(pack), designRef(pack), specSlideRef(generationSlide), slideHTMLRef(generationSlide)}
	for _, ref := range refs {
		raw, _, err := readArtifact(dir, nil, ref)
		if err != nil {
			t.Fatal(err)
		}
		kind := "schema"
		if ref.Kind == ArtifactSlideHTML {
			kind = "static"
		}
		evidence := newEvidence(kind, ref.Resource(), hashBytes(raw))
		evidence.Fresh = true
		if !restoredEvidenceFresh(state, evidence) {
			t.Fatalf("unchanged %s evidence was lost", ref.Kind)
		}
		evidence.SourceHash = "old-version"
		if restoredEvidenceFresh(state, evidence) {
			t.Fatalf("stale %s evidence survived", ref.Kind)
		}
	}
	html, _, _ := readArtifact(dir, nil, slideHTMLRef(generationSlide))
	proof, err := currentRenderProof(pack, dir, nil, generationSlide, hashBytes(html))
	if err != nil {
		t.Fatal(err)
	}
	evidence := newEvidence("render", slideHTMLRef(generationSlide).Resource(), hashBytes(html))
	evidence.Fresh, evidence.Render = true, &proof
	if !restoredEvidenceFresh(state, evidence) {
		t.Fatal("unchanged render proof was lost")
	}
	proof.SpecHash = "old-spec"
	if restoredEvidenceFresh(state, evidence) {
		t.Fatal("changed render dependency was ignored")
	}
	evidence.Kind, evidence.Render, evidence.Fresh = "static", nil, false
	if restoredEvidenceFresh(state, evidence) {
		t.Fatal("previously invalidated evidence was revived")
	}
}

func TestResumedRunFinishesWithoutRewritingValidSemanticResources(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	changes := EmptyChangeSet()
	checkpoint := &RuntimeCheckpoint{RunID: "resume-validation", LoopID: "retained-loop", Mode: model.ModeExecute, Phase: PhaseExecuting, Scope: pack.Command.Scope}
	if err := ensureReviewBaseline(context.Background(), dir, checkpoint.RunID, false); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, ref := range []ArtifactRef{manifestRef(pack), outlineRef(pack), designRef(pack), specSlideRef(generationSlide)} {
		raw, _, err := readArtifact(dir, nil, ref)
		if err != nil {
			t.Fatal(err)
		}
		before[ref.Resource().Key()] = raw
		changes.Updated = append(changes.Updated, ArtifactChange{Artifact: ref, Source: "edit_spec", AfterHash: hashBytes(raw)})
		evidence := newEvidence("schema", ref.Resource(), hashBytes(raw))
		// Missing/stale receipts are deliberately repaired by read-only checks.
		evidence.Fresh = false
		checkpoint.Evidence = append(checkpoint.Evidence, evidence)
	}
	checkpoint.Changes = changes
	agent := &scriptedAgent{responses: []AgentResponse{finishCall("finish")}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{RunID: checkpoint.RunID, ProjectDir: dir, Context: pack, ResumeCheckpoint: checkpoint})
	if outcome.Status != StatusCompleted || len(agent.requests) != 1 {
		t.Fatalf("completion required extra model work: %+v", outcome)
	}
	for _, change := range changes.Updated {
		raw, _, err := readArtifact(dir, nil, change.Artifact)
		if err != nil || !bytes.Equal(raw, before[change.Artifact.Resource().Key()]) {
			t.Fatalf("completion modified %s: %v", change.Artifact.Kind, err)
		}
	}
}

func TestCompletionRefreshSeparatesInvalidContentReadFailureAndMissingRender(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "validation-defects")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	state := &RunState{pack: pack, projectDir: dir, mode: model.ModeExecute, scope: pack.Command.Scope, tx: session, ledger: NewEvidenceLedger()}
	input := RuntimeInput{ProjectDir: dir}
	manifest := manifestRef(pack)
	raw, _, _ := readArtifact(dir, nil, manifest)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	delete(fields, "goal")
	invalid, _ := json.Marshal(fields)
	writeGenerationFile(t, dir, manifest.Path, invalid)
	changes := EmptyChangeSet()
	changes.Updated = []ArtifactChange{{Artifact: manifest, Source: "edit_manifest", AfterHash: hashBytes(invalid)}}
	issues, err := NewRuntime(nil).refreshCompletionEvidence(context.Background(), input, state, changes)
	if err != nil || issues[manifest.Resource().Key()].Code != "RESOURCE_CONTENT_INVALID" || len(issues[manifest.Resource().Key()].ValidationErrors) == 0 {
		t.Fatalf("actual schema defect not reported: %+v, %v", issues, err)
	}
	if issue := issues[manifest.Resource().Key()]; len(issue.RequiredActions) != 0 || strings.Contains(issue.NextAction, "Call read_resource") {
		t.Fatalf("damaged semantic source requires an impossible read/edit loop: %+v", issue)
	}
	if state.ledger.HasFresh(manifest.Resource(), hashBytes(invalid), "schema") {
		t.Fatal("invalid source acquired valid evidence")
	}
	writeGenerationFile(t, dir, manifest.Path, raw)
	html := slideHTMLRef(generationSlide)
	htmlRaw, _, _ := readArtifact(dir, nil, html)
	invalidHTML := []byte("<!doctype html><html><body>missing stage</body></html>")
	writeGenerationFile(t, dir, html.Path, invalidHTML)
	changes.Updated = []ArtifactChange{{Artifact: html, Source: "edit_html", AfterHash: hashBytes(invalidHTML)}}
	issues, err = NewRuntime(nil).refreshCompletionEvidence(context.Background(), input, state, changes)
	if issue := issues[html.Resource().Key()]; err != nil || issue.Code != "RESOURCE_CONTENT_INVALID" || len(issue.RequiredActions) != 1 || issue.RequiredActions[0].Tool != "edit_html" {
		t.Fatalf("repairable HTML defect lacks focused feedback: %+v, %v", issues, err)
	}
	writeGenerationFile(t, dir, html.Path, htmlRaw)
	changes.Updated = []ArtifactChange{{Artifact: html, Source: "edit_html", AfterHash: hashBytes(htmlRaw)}}
	issues, err = NewRuntime(nil).refreshCompletionEvidence(context.Background(), input, state, changes)
	if err != nil || len(issues) != 0 || !state.ledger.HasFresh(html.Resource(), hashBytes(htmlRaw), "static") {
		t.Fatalf("static evidence was not refreshed: %v %v", issues, err)
	}
	blocked := (EvidenceCompletionPolicy{}).Check(CompletionContext{Mode: model.ModeExecute, Session: session, Context: pack, Changes: changes, Evidence: state.ledger})
	if len(blocked) != 1 || blocked[0].Code != "EVIDENCE_HTML_MISSING" || blocked[0].RequiredActions[0].Tool != "render_slide" {
		t.Fatalf("static validation bypassed rendering: %+v", blocked)
	}
	if err := os.Remove(filepath.Join(dir, html.Path)); err != nil {
		t.Fatal(err)
	}
	if _, err = NewRuntime(nil).refreshCompletionEvidence(context.Background(), input, state, changes); err == nil {
		t.Fatal("missing source was treated as an editable schema defect")
	}
}
