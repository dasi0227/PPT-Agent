package gitcommit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutorStagesOnlyProjectSources(t *testing.T) {
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
	if changes.FilesChanged != 2 {
		t.Fatalf("expected manifest and .gitignore only, got %+v", changes)
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
	if err := os.WriteFile(filepath.Join(root, "sli_deck.html"), []byte("<main>deck</main>"), 0600); err != nil {
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

func TestWhitelistRemovesTrackedCacheWithoutDeletingWorkFiles(t *testing.T) {
	ctx, root := context.Background(), t.TempDir()
	executor := NewExecutor()
	if err := executor.Bootstrap(ctx, root); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		".runtime/exports/deck.html": "export", "attachments/att_one.webp": "thumbnail",
		"attachments/att_one.png": "original", "sli_one.html": "slide", "notes.txt": "notes",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate a repository created before the whitelist was introduced.
	if _, err := executor.run(ctx, root, nil, "add", "-f", "--", "."); err != nil {
		t.Fatal(err)
	}
	env := []string{"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@local", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@local"}
	if _, err := executor.run(ctx, root, env, "commit", "-m", "old snapshot"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "sli_one.html")); err != nil {
		t.Fatal(err)
	}
	// Staging must enforce the whitelist independently of ignore configuration.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	changes, cleanup, err := executor.StageAll(ctx, root, "whitelist")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, path := range []string{".runtime/exports/deck.html", "attachments/att_one.webp", "notes.txt", "sli_one.html"} {
		if !strings.Contains(changes.NameStatus, "D\t"+path+"\n") {
			t.Fatalf("not removed from snapshot: %s: %s", path, changes.NameStatus)
		}
	}
	tracked, err := executor.run(ctx, root, []string{"GIT_INDEX_FILE=" + changes.IndexPath}, "ls-files")
	if err != nil {
		t.Fatal(err)
	}
	if tracked != ".gitignore\nattachments/att_one.png\n" {
		t.Fatalf("unexpected tracked files: %s", tracked)
	}
	for _, path := range []string{".runtime/exports/deck.html", "attachments/att_one.webp", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatal("work file deleted", path, err)
		}
	}
}
