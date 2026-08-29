package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSessionRollsBackFilesWhenCommitMetadataFails(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(existingPath, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_metadata_failure")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err := session.Write(ArtifactRef{Kind: ArtifactManifest, ID: "project", Path: "manifest.json"}, "test", []byte("after")); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Write(ArtifactRef{Kind: ArtifactDesign, ID: "project", Path: "design.json"}, "test", []byte("created")); err != nil {
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
	if _, statErr := os.Stat(filepath.Join(dir, "design.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("new artifact survived failed metadata commit: %v", statErr)
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
