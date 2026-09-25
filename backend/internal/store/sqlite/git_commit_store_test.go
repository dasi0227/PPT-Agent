package sqlite

import (
	"context"
	"encoding/json"
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
		CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{
		ID: "t1", ProjectID: "p1", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	command, _, err := st.AcceptCommand(ctx, "t1", model.CommandRequest{RequestKey: "req_1", Kind: "commit", Input: json.RawMessage(`{}`)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	runModel := model.Run{
		ID: "r1", ThreadID: "t1", ProjectID: "p1", Status: model.RunPending,
		Command: model.RunCommand{
			Scope: model.NewRunScope(model.ScopeAllPages),
			Mode:  model.ModeExecute, Instruction: "build",
		},
		CreatedAt: 2, UpdatedAt: 2,
	}
	if err := st.CreateRun(ctx, runModel); !errors.Is(err, store.ErrGitCommitActive) {
		t.Fatalf("expected Git commit conflict, got %v", err)
	}
	command.Status = "completed"
	command.Result = json.RawMessage(`{"empty":true}`)
	if err := st.SaveCommandExecution(ctx, command); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(ctx, runModel); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.AcceptCommand(ctx, "t1", model.CommandRequest{RequestKey: "req_2", Kind: "commit", Input: json.RawMessage(`{}`)}, 0); !errors.Is(err, store.ErrRunActive) {
		t.Fatalf("expected active Run conflict: %v", err)
	}
}
