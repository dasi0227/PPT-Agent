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

	interrupted := model.GitCommitOperation{
		ID: "gco_interrupted", ProjectID: "p1", ThreadID: "t1",
		ClientRequestID: "req-interrupted", ModelProfile: "Commit",
		Status: model.GitCommitRunning, Phase: model.GitCommitAnalyzing,
		CreatedAt: 3, UpdatedAt: 3,
	}
	if err := st.CreateGitCommitOperation(ctx, interrupted); err != nil {
		t.Fatal(err)
	}
	if err := svc.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := st.GetGitCommitOperation(ctx, interrupted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != model.GitCommitFailed {
		t.Fatalf("interrupted operation was not failed: %+v", recovered)
	}
	recoveredEvents, err := st.GitCommitEventsSince(ctx, interrupted.ID, 0)
	if err != nil || len(recoveredEvents) != 1 || recoveredEvents[0].Type != model.EventGitCommitFailed {
		t.Fatalf("unexpected recovery events: %+v err=%v", recoveredEvents, err)
	}
}

func TestGitCommitServiceReturnsEmptyAfterProjectInitialization(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, cleanupDB, err := sqlitestore.Open(&config.Config{
		DBPath:   filepath.Join(root, "commit.db"),
		WorkRoot: root,
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanupDB)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project, err := NewProjectService(st, WorkRoot(root)).CreateProject(ctx, CreateProjectParams{
		Topic: "Empty after initialization", Language: "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	thread := model.Thread{
		ID: "t-empty", ProjectID: project.ID, HistoryPath: "threads/t-empty.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.CreateThread(ctx, thread); err != nil {
		t.Fatal(err)
	}

	provider := &llmtest.FakeProvider{
		ProviderName: "fake",
		ModelName:    "commit-model",
		Caps:         llm.Capabilities{ToolCalls: true},
	}
	registry, err := llm.NewRegistryWithProfiles("Commit", []llm.Profile{
		llm.NewTestProfile("Commit", "https://example.invalid", provider),
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewGitCommitService(st, registry, run.NewLockManager())
	operation, err := svc.Start(ctx, project.ID, GitCommitParams{
		ThreadID: thread.ID, Model: "Commit", ClientRequestID: "req-empty",
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
				t.Fatalf("stream closed before empty event: %v", names)
			}
			names = append(names, event.Type)
			if event.Type.Terminal() {
				goto complete
			}
		case <-timer.C:
			t.Fatal("timed out waiting for empty Git commit")
		}
	}

complete:
	expected := []model.GitCommitEventType{model.EventGitCommitProgress, model.EventGitCommitEmpty}
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
	if stored.Status != model.GitCommitEmpty {
		t.Fatalf("unexpected operation: %+v", stored)
	}
	if requests := provider.Requests(); len(requests) != 0 {
		t.Fatalf("empty commit called the model: %+v", requests)
	}
}
