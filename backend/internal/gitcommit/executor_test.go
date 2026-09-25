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
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "source.jsonl"), []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".manifest.json"), []byte("{\"title\":\"Deck\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, cleanup, err := executor.StageAll(ctx, root, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if changes.FilesChanged != 3 {
		t.Fatalf("expected all artifact files and .gitignore, got %+v", changes)
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
		t.Fatalf("committed artifact repository remains dirty: %+v", next)
	}
}

func TestReconcileCompletedAttemptWithoutRepeatingCommit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	executor := NewExecutor()
	if err := executor.Bootstrap(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deck.html"), []byte("<main>deck</main>"), 0600); err != nil {
		t.Fatal(err)
	}
	changes, cleanup, err := executor.StageAll(ctx, root, "recover")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	message := Message{Title: "feat: save presentation", Items: []string{"Save initial deck"}}
	intent, err := executor.PrepareIntent(ctx, root, "recover", changes, message)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := executor.Commit(ctx, root, changes, message)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate process death after Git finished and before the command terminal event.
	recovered, confirmed, err := executor.Reconcile(ctx, root, intent)
	if err != nil || !confirmed || recovered.Hash != committed.Hash {
		t.Fatalf("reconcile: %+v %v %v", recovered, confirmed, err)
	}
	intent.Tree = "different"
	if _, _, err := executor.Reconcile(ctx, root, intent); err == nil {
		t.Fatal("mismatched intent accepted")
	}
	intent.AttemptID = "unknown"
	if _, confirmed, err := executor.Reconcile(ctx, root, intent); err != nil || confirmed {
		t.Fatalf("unknown attempt confirmed: %v %v", confirmed, err)
	}
}
