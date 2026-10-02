package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestNoopResourceCommitRechecksSemanticVersionWithoutWriting(t *testing.T) {
	for _, kind := range []string{"format", "content"} {
		t.Run(kind, func(t *testing.T) {
			dir, _, pack := generationPackFixture(t)
			root, _ := NewRunSession(dir, "noop")
			defer root.Discard()
			operation := root.operationSession()
			tool := resourceEditTool{name: "edit_manifest"}
			result := tool.Execute(context.Background(), DomainToolInput{
				ProjectDir: dir, Session: operation, Context: pack, Scope: pack.Command.Scope,
				Mode: model.ModeExecute, Args: map[string]any{"title": "Deck"},
				Messages: testResourceMessages(t, dir, pack),
			})
			if !result.OK || operation.ChangeSet().Count() != 0 {
				t.Fatalf("not a successful no-op: %+v", result)
			}
			manifest := pack.PresentationManifest.Manifest
			if kind == "content" {
				manifest.Title = "External"
			}
			external, _ := json.MarshalIndent(manifest, "", "    ")
			writeGenerationFile(t, dir, ".manifest.json", external)
			_, err := operation.CommitOperation(context.Background(), "noop", "", nil)
			if kind == "format" && err != nil || kind == "content" && !errors.Is(err, ErrArtifactHashMismatch) {
				t.Fatalf("no-op baseline: %v", err)
			}
			if current, _ := os.ReadFile(filepath.Join(dir, ".manifest.json")); string(current) != string(external) {
				t.Fatal("no-op overwrote current bytes")
			}
		})
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
	requestHash, err := mutationRequestHash(journal.RunID, journal.OperationID, journal.Artifacts, nil)
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
	requestHash, err := mutationRequestHash(journal.RunID, journal.OperationID, journal.Artifacts, nil)
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

func TestRunSessionSnapshotRestoresOverlayWithoutChangingBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_restore")
	if err != nil {
		t.Fatal(err)
	}
	ref := projectFileRef("notes.txt")
	if _, err := session.Write(ref, "run_command", []byte("after\n")); err != nil {
		t.Fatal(err)
	}
	snapshot := session.Snapshot()
	session.Discard()

	restored, err := RestoreRunSession(dir, "run_restore", *snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Discard()
	staged, err := restored.Read(ref)
	if err != nil || string(staged) != "after\n" {
		t.Fatalf("staged=%q err=%v", staged, err)
	}
	baseline, err := os.ReadFile(path)
	if err != nil || string(baseline) != "before\n" {
		t.Fatalf("baseline=%q err=%v", baseline, err)
	}
	if err := restored.Commit(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if restored.Snapshot() != nil {
		t.Fatal("committed session remained checkpointable")
	}
	committed, err := os.ReadFile(path)
	if err != nil || string(committed) != "after\n" {
		t.Fatalf("committed=%q err=%v", committed, err)
	}
}

func TestRunSessionSnapshotRejectsChangedBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_conflict")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Write(projectFileRef("notes.txt"), "run_command", []byte("after\n")); err != nil {
		t.Fatal(err)
	}
	snapshot := session.Snapshot()
	session.Discard()
	if err := os.WriteFile(path, []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if restored, err := RestoreRunSession(dir, "run_conflict", *snapshot); err == nil {
		restored.Discard()
		t.Fatal("restored an overlay after its baseline changed")
	}
	if ActiveRunSession(dir) != nil {
		t.Fatal("failed restore left an active session")
	}
}

func TestRunSessionSnapshotRejectsTampering(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_tampered")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Write(projectFileRef("notes.txt"), "run_command", []byte("after\n")); err != nil {
		t.Fatal(err)
	}
	snapshot := session.Snapshot()
	session.Discard()
	snapshot.Artifacts[0].AfterContent = []byte("tampered\n")

	if restored, err := RestoreRunSession(dir, "run_tampered", *snapshot); err == nil {
		restored.Discard()
		t.Fatal("restored a tampered overlay")
	}
}
