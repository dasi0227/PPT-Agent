package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestGitCommitServiceEmitsRealPhasesAndPersistsResult(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, cleanupDB, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "commit.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanupDB)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	project := model.Project{
		ID: "p1", Title: "Growth deck", WorkDir: projectDir, Theme: "default",
		Status: "draft", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{
		ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "manifest.json"), []byte("{\"title\":\"Growth\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := &llmtest.FakeProvider{
		Caps: llm.Capabilities{ToolCalls: true},
		Script: []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{
			ID: "call-1", Name: "git_commit",
			Args: map[string]any{
				"title": "feat: initialize growth deck",
				"items": []any{"Track the initial deck manifest"},
			},
		}}}},
	}
	registry, err := llm.NewRegistryWithProfiles("Commit", []llm.Profile{
		llm.NewTestProfile("Commit", "https://example.invalid", provider),
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewGitCommitService(st, registry, run.NewLockManager())
	operation, err := svc.Start(ctx, "p1", GitCommitParams{
		ThreadID: "t1", Model: "Commit", ClientRequestID: "req-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	events, stop, err := svc.Subscribe(ctx, operation.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	var names []model.GitCommitEventType
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("stream closed before terminal event: %v", names)
			}
			names = append(names, event.Type)
			if event.Type.Terminal() {
				goto complete
			}
		case <-timer.C:
			t.Fatal("timed out waiting for Git commit")
		}
	}

complete:
	expected := []model.GitCommitEventType{
		model.EventGitCommitProgress,
		model.EventGitCommitProgress,
		model.EventGitCommitProgress,
		model.EventGitCommitCompleted,
	}
	if len(names) != len(expected) {
		t.Fatalf("unexpected events: %v", names)
	}
	for i := range names {
		if names[i] != expected[i] {
			t.Fatalf("unexpected events: %v", names)
		}
	}
	stored, err := st.GetGitCommitOperation(ctx, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.GitCommitCompleted {
		t.Fatalf("unexpected operation: %+v", stored)
	}
	var result model.GitCommitResult
	if err := json.Unmarshal([]byte(stored.ResultJSON), &result); err != nil {
		t.Fatal(err)
	}
	if result.Title != "feat: initialize growth deck" || result.Hash == "" || result.FilesChanged != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	requests := provider.Requests()
	if len(requests) != 1 || len(requests[0].Tools) != 1 || requests[0].Tools[0].Name != "git_commit" {
		t.Fatalf("unexpected model request: %+v", requests)
	}
}
