package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func applyPublicRunUpdate(t *testing.T, s *sqlitestore.Store, id string, kind model.EventType, value any) {
	t.Helper()
	r, err := s.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	ctx := workflow.WithCheckpointLease(sessionstate.WithRequestScene(context.Background(), r.SceneRevision), r.OwnerInstanceID, r.ExecutionRevision, r.CheckpointRevision)
	update, err := sessionstate.NewRunUpdate(kind, value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyRunUpdate(ctx, id, update); err != nil {
		t.Fatal(err)
	}
}

func seedPublicRunForNaming(t *testing.T, f briefingFixture, id, input, reply string) {
	t.Helper()
	command := model.RunCommand{Instruction: input, Mode: model.ModeCreate, Scope: model.NewRunScope(model.ScopeAllPages)}
	if err := f.store.CreateRun(context.Background(), model.Run{ID: id, ProjectID: f.project.ID, ThreadID: f.thread.ID, SceneRevision: 1, ClientRequestID: id, OwnerInstanceID: "test", Status: model.RunRunning, Command: command}); err != nil {
		t.Fatal(err)
	}
	if reply != "" {
		applyPublicRunUpdate(t, f.store, id, model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(id), MessageID: "final-" + id, Text: reply, AffectedTargets: []model.PublicTarget{}})
		applyPublicRunUpdate(t, f.store, id, model.EventRunCompleted, model.NewRunTerminalPayload(id, 0, nil, nil))
	} else if err := f.store.SetRunStatus(context.Background(), id, model.RunFailed); err != nil {
		t.Fatal(err)
	}
}

func commitRenameResult(t *testing.T, f briefingFixture, result RenameResult) {
	t.Helper()
	accepted, _, err := f.store.AcceptCommand(context.Background(), f.thread.ID, model.CommandRequest{RequestKey: model.MustShortID("test"), Kind: "rename", Input: json.RawMessage(`{"mode":"automatic"}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	command := accepted.Command
	command.Status = "completed"
	command.Result, err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := f.store.CompleteCommand(context.Background(), command, sessionstate.CommandEffects{Rename: result.PendingRename})
	if err != nil || completed.Status != "completed" {
		t.Fatalf("rename completion: %+v %v", completed, err)
	}
	if result.OnCommitted != nil {
		result.OnCommitted()
	}
}

func commitContextForTest(t *testing.T, f briefingFixture, update sessionstate.ContextUpdate) {
	t.Helper()
	ctx := context.Background()
	scene, err := f.store.ThreadSceneRevision(ctx, f.thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	compaction := model.ContextCompaction{ID: model.MustShortID("cmp"), ProjectID: f.project.ID, ThreadID: f.thread.ID, Trigger: model.ContextCompactionManual, CreatedAt: 1}
	accepted, _, err := f.store.AcceptCommand(ctx, f.thread.ID, model.CommandRequest{RequestKey: compaction.ID, Kind: "compact", Input: json.RawMessage(`{}`)}, scene)
	if err != nil {
		t.Fatal(err)
	}
	command := accepted.Command
	command.Status = "completed"
	command.Result, err = json.Marshal(compaction)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CompleteCommand(ctx, command, sessionstate.CommandEffects{Context: &update, Compaction: &compaction}); err != nil {
		t.Fatal(err)
	}
}
