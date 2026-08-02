package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTransactionMetadataFailureRestoresFormalArtifact(t *testing.T) {
	dir := t.TempDir()
	ref := ArtifactRef{Kind: ArtifactPresentation, ID: "s1", Path: "slides/s1/index.html"}
	formal := filepath.Join(dir, ref.Path)
	if err := os.MkdirAll(filepath.Dir(formal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(formal, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	tx, err := NewTransaction(dir, "metadata-rollback")
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Cleanup()
	if _, err := tx.Stage(ref, "step", []byte("after")); err != nil {
		t.Fatal(err)
	}
	metadataErr := errors.New("metadata commit failed")
	if err := tx.Commit(context.Background(), func(context.Context, ChangeSet) error {
		return metadataErr
	}); !errors.Is(err, metadataErr) {
		t.Fatalf("commit error=%v", err)
	}
	raw, err := os.ReadFile(formal)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "before" {
		t.Fatalf("formal artifact was not restored: %q", raw)
	}
}

func TestTransactionRejectsSourceRevisionConflict(t *testing.T) {
	dir := t.TempDir()
	ref := ArtifactRef{Kind: ArtifactSlide, ID: "s1", Path: "slides/s1/slide.json"}
	formal := filepath.Join(dir, ref.Path)
	if err := os.MkdirAll(filepath.Dir(formal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(formal, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	tx, err := NewTransaction(dir, "revision-conflict")
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Cleanup()
	if _, err := tx.Stage(ref, "step", []byte("v2")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(formal, []byte("concurrent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background(), nil); !errors.Is(err, ErrStagedHashMismatch) {
		t.Fatalf("commit error=%v", err)
	}
	raw, err := os.ReadFile(formal)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "concurrent" {
		t.Fatalf("conflict commit overwrote formal artifact: %q", raw)
	}
}

func TestTransactionRejectsConflictBeforeRestagingSameArtifact(t *testing.T) {
	dir := t.TempDir()
	ref := ArtifactRef{Kind: ArtifactSlide, ID: "s1", Path: "slides/s1/slide.json"}
	formal := filepath.Join(dir, ref.Path)
	if err := os.MkdirAll(filepath.Dir(formal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(formal, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	tx, err := NewTransaction(dir, "restage-conflict")
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Cleanup()
	if _, err := tx.Stage(ref, "write_ppt", []byte("staged-v2")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(formal, []byte("concurrent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Stage(ref, "edit_ppt", []byte("staged-v3")); !errors.Is(err, ErrStagedHashMismatch) {
		t.Fatalf("restage error=%v", err)
	}
	staged, err := tx.Read(ref)
	if err != nil {
		t.Fatal(err)
	}
	if string(staged) != "staged-v2" {
		t.Fatalf("conflicting restage changed staged content: %q", staged)
	}
}
