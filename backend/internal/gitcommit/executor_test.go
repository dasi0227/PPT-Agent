package gitcommit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutorStagesAndCommitsAllProjectChanges(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	executor := NewExecutor()
	if err := executor.Bootstrap(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "threads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "threads", "ignored.jsonl"), []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte("{\"title\":\"Deck\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, cleanup, err := executor.StageAll(ctx, root, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if changes.FilesChanged != 2 {
		t.Fatalf("expected manifest and .gitignore, got %+v", changes)
	}
	result, err := executor.Commit(ctx, root, changes, Message{
		Title: "feat: initialize presentation", Items: []string{"Track the initial presentation content"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Branch != "main" || len(result.Hash) != 7 || result.CommittedAt == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	next, nextCleanup, err := executor.StageAll(ctx, root, "empty")
	if err != nil {
		t.Fatal(err)
	}
	defer nextCleanup()
	if next.FilesChanged != 0 {
		t.Fatalf("ignored history left repository dirty: %+v", next)
	}
}
