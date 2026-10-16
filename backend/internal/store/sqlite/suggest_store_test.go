package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func TestSuggestCommandsAreBoundToTheLatestIdleTurnAndGeneration(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	request := model.CommandRequest{Kind: "suggest", RequestKey: "first", Input: json.RawMessage(`{}`)}
	if _, _, err := s.AcceptCommand(ctx, "t", request, 0); !errors.Is(err, store.ErrRunActive) {
		t.Fatalf("active turn: %v", err)
	}
	if err := s.db.Model(&runPO{}).Where("id = ?", "r").Update("status", string(model.RunDone)).Error; err != nil {
		t.Fatal(err)
	}
	thread, err := s.UpdateThreadSuggestState(ctx, "t", false)
	if err != nil || thread.AutoSuggestEnabled {
		t.Fatalf("disable: %+v %v", thread, err)
	}
	first, _, err := s.AcceptCommand(ctx, "t", request, 0)
	if err != nil {
		t.Fatal(err)
	}
	thread, _ = s.GetThread(ctx, "t")
	if thread.AutoSuggestEnabled {
		t.Fatal("manual generation changed the automatic preference")
	}
	request.RequestKey = "second"
	second, _, err := s.AcceptCommand(ctx, "t", request, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.CheckSuggestCommand(ctx, first), context.Canceled) {
		t.Fatal("older generation is still current")
	}
	if err := s.CheckSuggestCommand(ctx, second); err != nil {
		t.Fatal(err)
	}
	first.Status = "completed"
	first.Result = json.RawMessage(`{"requests":["stale"]}`)
	if err := s.SaveCommandExecution(ctx, first); err != nil {
		t.Fatal(err)
	}
	saved, err := s.GetCommand(ctx, first.CommandID)
	if err != nil || saved.Status != "canceled" || len(saved.Result) != 0 {
		t.Fatalf("late result: %+v %v", saved, err)
	}
	if err := s.CreateRun(ctx, model.Run{ID: "next", ProjectID: "p", ThreadID: "t", Status: model.RunPending,
		Command: model.RunCommand{Mode: model.ModeChat, Scope: model.NewRunScope(model.ScopeAllPages), Instruction: "new request"}, CreatedAt: 2, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.CheckSuggestCommand(ctx, second), context.Canceled) {
		t.Fatal("new turn did not invalidate suggestions")
	}
}
