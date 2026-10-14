package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestNoopResourceCommitRechecksSemanticVersionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		approved   bool
	}{{"direct_format", "format", false}, {"direct_content", "content", false}, {"approved_format", "format", true}, {"approved_content", "content", true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir, _, pack := generationPackFixture(t)
			root, _ := NewRunSession(dir, "noop")
			defer root.Discard()
			operation := root.operationSession()
			tool := resourceEditTool{name: "edit_manifest"}
			title := "Deck"
			if tc.approved {
				title = "Proposed"
			}
			result := tool.Execute(context.Background(), DomainToolInput{
				ProjectDir: dir, Session: operation, Context: pack, Scope: pack.Command.Scope,
				Mode: model.ModeExecute, Args: map[string]any{"title": title},
				Messages: testResourceMessages(t, dir, pack),
			})
			before, err := os.ReadFile(filepath.Join(dir, ".manifest.json"))
			if err != nil || !result.OK {
				t.Fatalf("initial edit: %+v (%v)", result, err)
			}
			if tc.approved {
				// The user revises the proposal back to the original saved content.
				result, err = stageApprovedResource(dir, pack, operation, ResourceEditApproval{Resource: "manifest", Draft: before})
				if err != nil {
					t.Fatal(err)
				}
			}
			if !result.OK || len(result.ChangedTargets) != 0 || operation.ChangeSet().Count() != 0 {
				t.Fatalf("not a successful no-op: %+v", result)
			}
			assertSavedResourceObservation(t, tool.name, result, before)
			manifest := pack.PresentationManifest.Manifest
			if tc.kind == "content" {
				manifest.Title = "External"
			}
			external, _ := json.MarshalIndent(manifest, "", "    ")
			writeGenerationFile(t, dir, ".manifest.json", external)
			_, err = operation.CommitOperation(context.Background(), "noop", "", nil)
			if tc.kind == "format" && err != nil || tc.kind == "content" && !errors.Is(err, ErrArtifactHashMismatch) {
				t.Fatalf("no-op baseline: %v", err)
			}
			if current, _ := os.ReadFile(filepath.Join(dir, ".manifest.json")); string(current) != string(external) {
				t.Fatal("no-op overwrote current bytes")
			}
		})
	}
}

func TestSpecCommitDistinguishesMissingAndInvalidOutline(t *testing.T) {
	for _, core := range []string{"Message", "Changed"} {
		for _, kind := range []string{"missing", "invalid", "absent_page"} {
			t.Run(core+"/"+kind, func(t *testing.T) {
				dir, _, pack := generationPackFixture(t)
				session, err := NewRunSession(dir, "outline-recheck")
				if err != nil {
					t.Fatal(err)
				}
				defer session.Discard()
				before, _ := os.ReadFile(filepath.Join(dir, model.SlideSpecPath(generationSlide)))
				resource := Resource{Type: "slide", Part: "spec", SlideID: generationSlide}
				result := (resourceEditTool{name: "edit_spec"}).Execute(context.Background(), DomainToolInput{
					ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope,
					Args: map[string]any{"slide_id": generationSlide, "core": core}, Messages: testResourceMessages(t, dir, pack),
				})
				if !result.OK {
					t.Fatalf("initial edit: %+v", result)
				}
				outlinePath := filepath.Join(dir, ".outline.json")
				if kind == "missing" {
					if err := os.Remove(outlinePath); err != nil {
						t.Fatal(err)
					}
				} else if kind == "invalid" {
					writeGenerationFile(t, dir, ".outline.json", []byte(`null`))
				} else {
					writeGenerationFile(t, dir, ".outline.json", []byte(`{"sections":[]}`))
				}
				_, err = session.CommitOperation(context.Background(), "commit-spec", "", nil)
				if err == nil {
					t.Fatal("spec committed without its valid outline page")
				}
				failure := resourceMutationFailure(err, resource)
				want := model.CodeNotFound
				if kind == "missing" {
					want = model.CodeOutlineNotExist
				}
				if kind == "invalid" {
					want = "RESOURCE_INVALID"
				}
				if failure.Code != want || (strings.Contains(errorObservation(t, failure)["measure"].(string), "edit_outline(init:") != (kind == "missing")) {
					t.Fatalf("commit dependency error misclassified: %+v", failure)
				}
				if after, _ := os.ReadFile(filepath.Join(dir, model.SlideSpecPath(generationSlide))); string(after) != string(before) {
					t.Fatal("failed commit changed saved spec")
				}
			})
		}
	}
}

func TestOperationSessionPreservesReadBaselineAndRebasesOnlyStructuredFormatting(t *testing.T) {
	for _, kind := range []string{"structured_format", "text_format", "structured_content"} {
		t.Run(kind, func(t *testing.T) {
			dir, _, pack := generationPackFixture(t)
			root, _ := NewRunSession(dir, "baseline")
			defer root.Discard()
			operation := root.operationSession()
			ref := manifestRef(pack)
			before, err := operation.Read(ref)
			if err != nil {
				t.Fatal(err)
			}
			value := pack.PresentationManifest.Manifest
			if kind == "structured_content" {
				value.Title = "External"
			}
			external, _ := json.MarshalIndent(value, "", "    ")
			writeGenerationFile(t, dir, ref.Path, external)
			if kind != "structured_content" && spec.ResourceBytesHash(before) != spec.ResourceBytesHash(external) {
				t.Fatal("fixture changed semantic hash")
			}
			value.Title = "Agent"
			after, _ := json.Marshal(value)
			source := "edit_manifest"
			if kind == "text_format" {
				source = "run_command"
			}
			if _, err := operation.Write(ref, source, after); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("metadata unavailable")
			_, err = operation.CommitOperation(context.Background(), "call", "", func(context.Context, CommitContext) error { return failure })
			want := ErrArtifactHashMismatch
			if kind == "structured_format" {
				want = failure
			}
			if !errors.Is(err, want) {
				t.Fatalf("baseline check: %v wanted %v", err, want)
			}
			current, _ := os.ReadFile(filepath.Join(dir, ref.Path))
			if string(current) != string(external) {
				t.Fatal("rollback restored an obsolete physical snapshot")
			}
		})
	}
}

func TestRunSessionRollsBackFilesWhenCommitMetadataFails(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, ".manifest.json")
	if err := os.WriteFile(existingPath, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_metadata_failure")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err := session.Write(ArtifactRef{Kind: ArtifactManifest, ID: "project", Path: ".manifest.json"}, "test", []byte("after")); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Write(ArtifactRef{Kind: ArtifactDesign, ID: "project", Path: ".design.json"}, "test", []byte("created")); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("metadata commit failed")
	err = session.Commit(context.Background(), func(context.Context, CommitContext) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("commit error=%v", err)
	}
	raw, readErr := os.ReadFile(existingPath)
	if readErr != nil || string(raw) != "before" {
		t.Fatalf("existing artifact was not rolled back: raw=%q err=%v", raw, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".design.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("new artifact survived failed metadata commit: %v", statErr)
	}
}

func TestRunSessionCommitsEachOperationAndStaysOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_operation")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err := session.Write(projectFileRef("notes.txt"), "run_command", []byte("after\n")); err != nil {
		t.Fatal(err)
	}
	var metadata CommitContext
	changes, err := session.CommitOperation(context.Background(), "call_1", `{"ok":true}`, func(_ context.Context, input CommitContext) error {
		metadata = input
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if changes.Count() != 1 || metadata.OperationID != "call_1" || metadata.RequestHash == "" || metadata.ToolResultJSON != `{"ok":true}` {
		t.Fatalf("changes=%+v metadata=%+v", changes, metadata)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "after\n" {
		t.Fatalf("committed raw=%q err=%v", raw, err)
	}
	if snapshot := session.Snapshot(); snapshot == nil || len(snapshot.Artifacts) != 0 {
		t.Fatalf("session did not reset after operation: %+v", snapshot)
	}
	if _, err := session.Write(projectFileRef("notes.txt"), "run_command", []byte("second\n")); err != nil {
		t.Fatalf("session was not reusable: %v", err)
	}
}

func TestRunSessionIgnoresIdenticalHTMLWrites(t *testing.T) {
	dir := t.TempDir()
	ref := slideHTMLRef("sli_one")
	path := filepath.Join(dir, filepath.FromSlash(ref.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_noop")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err := session.Write(ref, "edit_spec", []byte("same")); err != nil {
		t.Fatal(err)
	}
	if session.ChangeSet().Count() != 0 {
		t.Fatal("identical HTML became a change")
	}
}

func TestRecoverMutationJournalRollsBackWithoutReceipt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact := RunSessionArtifactState{
		Ref: ArtifactRef{Kind: ArtifactProjectFile, ID: "notes.txt", Path: "notes.txt"}, Relative: "notes.txt",
		BeforeContent: []byte("before\n"), AfterContent: []byte("after\n"),
		BeforeHash: hashBytes([]byte("before\n")), AfterHash: hashBytes([]byte("after\n")), Existed: true,
	}
	journal := MutationJournal{RunID: "run_recovery", OperationID: "call_1", Artifacts: []RunSessionArtifactState{artifact}}
	requestHash, err := mutationRequestHash(journal.RunID, journal.OperationID, journal.Artifacts)
	if err != nil {
		t.Fatal(err)
	}
	journal.RequestHash = requestHash
	journalPath := mutationJournalPath(dir, journal.RunID, journal.OperationID)
	if err := writeMutationJournal(journalPath, journal); err != nil {
		t.Fatal(err)
	}
	if err := RecoverMutationJournals(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "before\n" {
		t.Fatalf("recovered raw=%q err=%v", raw, err)
	}
}

func TestRecoverMutationJournalRollsForwardWithReceipt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact := RunSessionArtifactState{
		Ref: ArtifactRef{Kind: ArtifactProjectFile, ID: "notes.txt", Path: "notes.txt"}, Relative: "notes.txt",
		BeforeContent: []byte("before\n"), AfterContent: []byte("after\n"),
		BeforeHash: hashBytes([]byte("before\n")), AfterHash: hashBytes([]byte("after\n")), Existed: true,
	}
	journal := MutationJournal{RunID: "run_recovery", OperationID: "call_1", Artifacts: []RunSessionArtifactState{artifact}}
	requestHash, err := mutationRequestHash(journal.RunID, journal.OperationID, journal.Artifacts)
	if err != nil {
		t.Fatal(err)
	}
	journal.RequestHash = requestHash
	journalPath := mutationJournalPath(dir, journal.RunID, journal.OperationID)
	if err := writeMutationJournal(journalPath, journal); err != nil {
		t.Fatal(err)
	}
	lookup := func(_ context.Context, runID, operationID string) (string, bool, error) {
		if runID != journal.RunID || operationID != journal.OperationID {
			t.Fatalf("lookup run=%q operation=%q", runID, operationID)
		}
		return journal.RequestHash, true, nil
	}
	if err := RecoverMutationJournals(context.Background(), dir, lookup); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "after\n" {
		t.Fatalf("recovered raw=%q err=%v", raw, err)
	}
}
