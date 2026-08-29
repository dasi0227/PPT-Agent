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
