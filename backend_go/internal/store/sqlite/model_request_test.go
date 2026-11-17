package sqlite

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func preparedModelCheckpoint(t *testing.T, cp workflow.RuntimeCheckpoint) workflow.RuntimeCheckpoint {
	t.Helper()
	raw, err := sessionstate.EncodeModelRequest(llm.GenerateRequest{Messages: []llm.Message{{Role: llm.RoleSystem, Content: llm.TextContent("frozen prompt")}}})
	if err != nil {
		t.Fatal(err)
	}
	cp.PendingModelCall = &sessionstate.PendingModelCall{ID: "model-call", ContextRevision: cp.ContextRevision, Provider: "provider", Model: "model", Stage: "prepared"}
	cp.NewModelRequest = &sessionstate.ModelRequestSnapshot{ID: "model-call", SceneRevision: cp.SceneRevision, ContextRevision: cp.ContextRevision, Provider: "provider", Model: "model", RequestJSON: raw}
	cp.Boundary = "model_prepared"
	return cp
}

func TestModelResponseAndCheckpointCommitTogetherAndCannotRegress(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	cp = preparedModelCheckpoint(t, cp)
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	load := func() workflow.RuntimeCheckpoint {
		t.Helper()
		value, err := s.LatestCheckpoint(ctx, cp.RunID)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	dispatched := load()
	if dispatched.ModelRequestSnapshot == nil || dispatched.NewModelRequest != nil || dispatched.PendingModelCall.Stage != "prepared" {
		t.Fatal("checkpoint did not hydrate the immutable request separately from writes")
	}
	dispatched.PendingModelCall.Stage, dispatched.Boundary = "dispatched", "model_dispatched"
	if err := s.SaveCheckpoint(ctx, dispatched); err != nil {
		t.Fatal(err)
	}
	responded := load()
	responded.PendingModelCall.Stage, responded.Boundary = "responded", "model_responded"
	response, err := sessionstate.EncodeModelResponse(llm.GenerateResponse{Content: llm.TextContent("complete response"), Replay: &llm.AssistantReplay{Source: "provider", Blocks: []json.RawMessage{json.RawMessage(`{"signature":"signed"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	responded.ModelResponseJSON = response
	if err := s.db.Exec(`CREATE TRIGGER reject_model_checkpoint BEFORE UPDATE OF checkpoint_json ON runs BEGIN SELECT RAISE(ABORT,'checkpoint failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, responded); err == nil {
		t.Fatal("checkpoint failure was ignored")
	}
	before := load()
	if before.PendingModelCall.Stage != "dispatched" || len(before.ModelRequestSnapshot.ResponseJSON) != 0 {
		t.Fatal("response survived without its checkpoint transition")
	}
	if err := s.db.Exec(`DROP TRIGGER reject_model_checkpoint`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, responded); err != nil {
		t.Fatal(err)
	}
	after := load()
	decoded, err := sessionstate.DecodeModelResponse(after.ModelRequestSnapshot.ResponseJSON)
	if err != nil || after.PendingModelCall.Stage != "responded" || decoded.Text() != "complete response" || decoded.Replay == nil {
		t.Fatalf("saved response cannot be recovered: %+v, %v", decoded, err)
	}
	after.PendingModelCall.Stage = "dispatched"
	if err := s.SaveCheckpoint(ctx, after); err == nil {
		t.Fatal("completed model response regressed to dispatched")
	}
}

func TestModelPreparationRollsBackAndPauseKeepsUnknownRequest(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	cp = preparedModelCheckpoint(t, cp)
	if err := s.db.Exec(`CREATE TRIGGER reject_model_prepare BEFORE UPDATE OF checkpoint_json ON runs BEGIN SELECT RAISE(ABORT,'checkpoint failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, cp); err == nil {
		t.Fatal("expected preparation failure")
	}
	var count int64
	if err := s.db.Table("model_request_snapshots").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("request escaped its checkpoint transaction: %d %v", count, err)
	}
	if err := s.db.Exec(`DROP TRIGGER reject_model_prepare`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	dispatched, err := s.LatestCheckpoint(ctx, cp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	dispatched.PendingModelCall.Stage = "dispatched"
	if err := s.SaveCheckpoint(ctx, dispatched); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PauseRun(ctx, cp.RunID, cp.OwnerInstanceID, "restart", 1); err != nil {
		t.Fatal(err)
	}
	paused, err := s.LatestCheckpoint(ctx, cp.RunID)
	if err != nil || paused.PendingModelCall == nil || paused.PendingModelCall.Stage != "outcome_unknown" || paused.ModelRequestSnapshot == nil || paused.ModelRequestSnapshot.ID != cp.NewModelRequest.ID {
		t.Fatalf("pause lost the exact dispatched request: %+v %v", paused.PendingModelCall, err)
	}
}
