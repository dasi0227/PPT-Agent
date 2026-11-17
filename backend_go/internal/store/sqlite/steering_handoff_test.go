package sqlite

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func completedSteeringFixture(t *testing.T) (*Store, model.Run, []model.SteeringMessage) {
	t.Helper()
	s := newTestStore(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	if err := s.CreateProject(ctx, model.Project{ID: "p", WorkDir: t.TempDir(), CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	previous := model.Run{ID: "old", ThreadID: "t", ProjectID: "p", SceneRevision: 1, ClientRequestID: "old-input", OwnerInstanceID: "owner", ExecutionRevision: 1, Status: model.RunRunning,
		Command: model.RunCommand{Instruction: "original", Mode: model.ModeChat, Scope: model.NewRunScope(model.ScopeAllPages)}}
	if err := s.CreateRun(ctx, previous); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "three"} {
		if _, _, err := s.CreateSteering(ctx, model.SteeringMessage{RunID: previous.ID, ThreadID: previous.ThreadID,
			ClientMessageID: id, RequestHash: id, Content: id, Scope: model.NewRunScope(model.ScopeAllPages)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetRunStatus(ctx, previous.ID, model.RunDone); err != nil {
		t.Fatal(err)
	}
	previous.Status = model.RunDone
	messages, err := s.ListPendingSteering(ctx, previous.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, previous, messages
}

func TestSteeringHandoffAcceptsNewRunAndMovesInboxAtomically(t *testing.T) {
	s, previous, messages := completedSteeringFixture(t)
	ctx := run.WithSteeringHandoff(context.Background(), previous.ID, messages)
	var accepted inputPO
	if err := s.reader.Where("thread_id = ? AND client_message_id = ?", "t", "one").Take(&accepted).Error; err != nil {
		t.Fatal(err)
	}
	var original struct {
		RunID, InputID, RoundID, NativeJSON, PublicJSON string
		CreatedSeq, CreatedAt                           int64
	}
	if err := s.reader.Table("conversation_messages").Where("id = ?", accepted.MessageID).Take(&original).Error; err != nil {
		t.Fatal(err)
	}
	next := previous
	next.ID, next.Status, next.ClientRequestID, next.RequestHash = "new", model.RunPending, "queued_old", "hash"
	next.Command.Instruction = messages[0].Content
	if err := s.CreateRun(ctx, next); err != nil {
		t.Fatal(err)
	}
	oldPending, err := s.ListPendingSteering(ctx, previous.ID)
	if err != nil || len(oldPending) != 0 {
		t.Fatalf("old inbox not consumed: %+v %v", oldPending, err)
	}
	newPending, err := s.ListPendingSteering(ctx, next.ID)
	if err != nil || len(newPending) != 2 || newPending[0].Content != "two" || newPending[1].Content != "three" {
		t.Fatalf("new inbox lost ordering: %+v %v", newPending, err)
	}
	if !newPending[0].Scope.Equal(next.Command.Scope) || !newPending[1].Scope.Equal(next.Command.Scope) {
		t.Fatalf("new initial scope authority: %+v", newPending)
	}
	current, err := s.GetRun(ctx, next.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: "initial:" + next.ClientRequestID, Fingerprint: "primary", Disposition: model.InputTask, ExecutionRevision: current.ExecutionRevision}); err != nil {
		t.Fatal(err)
	}
	session, err := contextengine.NewContextStore(s).Open(ctx, "t", nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := session.Prepare([]llm.Message{{Role: llm.RoleUser, Content: llm.TextContent(next.Command.Instruction), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: next.ID}}}, next.ID, "append")
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := workflow.RuntimeCheckpoint{RunID: next.ID, SceneRevision: 1, LoopID: "new-loop", OwnerInstanceID: "owner", ExecutionRevision: 1, Scope: next.Command.Scope, Mode: next.Command.Mode, Phase: workflow.PhaseChat, ContextRevision: session.Snapshot.Revision, ContextUpdate: &prepared.Update}
	if err := s.SaveCheckpoint(context.Background(), checkpoint); err != nil {
		t.Fatalf("initial checkpoint failed after handoff: %v", err)
	}
	checkpoint, err = s.LatestCheckpoint(context.Background(), next.ID)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint.Scope = newPending[1].Scope
	if err := s.SaveCheckpoint(context.Background(), checkpoint); err != nil {
		t.Fatalf("consuming initial queued scope caused revision conflict: %v", err)
	}
	var first steeringPO
	if err := s.db.First(&first, "thread_id = ? AND client_message_id = ?", "t", "one").Error; err != nil {
		t.Fatal(err)
	}
	if first.RunID != next.ID || first.Status != string(model.SteeringInjected) {
		t.Fatalf("primary message not owned by new Run: %+v", first)
	}
	if value, err := s.GetRun(context.Background(), previous.ID); err != nil || value.Status != model.RunDone {
		t.Fatalf("original Run changed outcome: %+v %v", value, err)
	}
	if recoverable, err := s.CompletedRunsWithPendingSteering(context.Background()); err != nil || len(recoverable) != 0 {
		t.Fatalf("handoff can be repeated after restart: %+v %v", recoverable, err)
	}
	var transferred inputPO
	if err := s.reader.Where("id = ?", accepted.ID).Take(&transferred).Error; err != nil {
		t.Fatal(err)
	}
	if textOrEmpty(transferred.RunID) != next.ID || textOrEmpty(transferred.SourceRunID) != previous.ID || transferred.MessageID != accepted.MessageID || transferred.AcceptedSeq != accepted.AcceptedSeq || transferred.HistoryBeforeSeq != accepted.HistoryBeforeSeq {
		t.Fatalf("handoff changed input identity or cutoff: before=%+v after=%+v", accepted, transferred)
	}
	after := original
	if err := s.reader.Table("conversation_messages").Where("id = ?", accepted.MessageID).Take(&after).Error; err != nil || after != original {
		t.Fatalf("handoff rewrote the accepted message: before=%+v after=%+v err=%v", original, after, err)
	}
	runUpdateForTest(t, s, next.ID, model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(next.ID), MessageID: "handoff-final", Text: "handoff reply", AffectedTargets: []model.PublicTarget{}})
	steeringForTest(t, s, next.ID, "after", "next task")
	if _, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: "after", Fingerprint: "after", Disposition: model.InputTask, ExecutionRevision: current.ExecutionRevision}); err != nil {
		t.Fatal(err)
	}
	history, err := s.LoadTaskConversationBeforeInput(ctx, "t", next.ID, "after")
	if err != nil {
		t.Fatal(err)
	}
	var taskMessages []model.ConversationMessage
	for _, round := range history.Rounds {
		taskMessages = append(taskMessages, round.Messages...)
	}
	if len(taskMessages) != 2 || taskMessages[0].Role != "user" || taskMessages[0].Text != "one" || taskMessages[1].Role != "assistant" || taskMessages[1].Text != "handoff reply" {
		t.Fatalf("transferred task history duplicated input or lost its reply: %+v", taskMessages)
	}
}
