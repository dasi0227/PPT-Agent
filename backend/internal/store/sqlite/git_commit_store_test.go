package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func TestProjectWriteTriggersSerializeRunsAndGitCommits(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.CreateProject(ctx, model.Project{
		ID: "p1", Title: "Deck", WorkDir: t.TempDir(), Theme: "default",
		Status: "draft", LayoutVersion: currentProjectLayoutVersion, CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{
		ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	operation := model.GitCommitOperation{
		ID: "gco_1", ProjectID: "p1", ThreadID: "t1", ClientRequestID: "req_1",
		ModelProfile: "test", Status: model.GitCommitAccepted, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.CreateGitCommitOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	runModel := model.Run{
		ID: "r1", ThreadID: "t1", ProjectID: "p1", Status: model.RunPending,
		Command: model.RunCommand{
			Scope: model.NewRunScope(model.ScopeObjectPresentation, model.ScopeAllPages),
			Mode:  model.ModeExecute, Instruction: "build",
		},
		CreatedAt: 2, UpdatedAt: 2,
	}
	if err := st.CreateRun(ctx, runModel); !errors.Is(err, store.ErrGitCommitActive) {
		t.Fatalf("expected Git commit conflict, got %v", err)
	}
	operation.Status, operation.UpdatedAt = model.GitCommitCompleted, 2
	if err := st.UpdateGitCommitOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(ctx, runModel); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGitCommitOperation(ctx, model.GitCommitOperation{
		ID: "gco_2", ProjectID: "p1", ThreadID: "t1", ClientRequestID: "req_2",
		ModelProfile: "test", Status: model.GitCommitAccepted, CreatedAt: 3, UpdatedAt: 3,
	}); !errors.Is(err, store.ErrRunActive) {
		t.Fatalf("expected active Run conflict, got %v", err)
	}
}
