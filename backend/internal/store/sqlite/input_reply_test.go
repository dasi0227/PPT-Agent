package sqlite

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestStandaloneChatReplyAppendsOneTurnToModelHistory(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedProject(t, s)
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p1"}); err != nil {
		t.Fatal(err)
	}
	current := model.Run{ID: "r", ThreadID: "t", ProjectID: "p1", ClientRequestID: "greet",
		Status: model.RunRunning, ExecutionRevision: 1,
		Command: model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeChat, Instruction: "  你能做什么？\n"}}
	if err := s.CreateRun(ctx, current); err != nil {
		t.Fatal(err)
	}
	transcript := contextengine.NewJournalTranscriptStore(s)
	prior := llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("上一轮的答复")}
	if err := transcript.Replace("", current.ThreadID, []llm.Message{prior}); err != nil {
		t.Fatal(err)
	}
	decision := model.InputDecision{InputID: "initial:greet", Disposition: model.InputChat, Fingerprint: "request",
		ExecutionRevision: current.ExecutionRevision, ReplyText: "我能帮你制作 HTML 演示文稿。"}
	if _, err := s.SaveInputDecision(ctx, current, decision); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(current.ID),
		MessageID: "input_greet", Text: decision.ReplyText, AffectedTargets: []model.PublicTarget{}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		event := model.Event{RunID: current.ID, Type: model.EventMessageFinal, Payload: string(raw)}
		if err := s.AppendInputReply(ctx, &event, decision.InputID); err != nil {
			t.Fatal(err)
		}
	}
	messages, err := transcript.Load("", current.ThreadID)
	if err != nil || len(messages) != 3 || messages[0].Text() != prior.Text() ||
		!llm.IsRunInput(messages[1], current.ID) || messages[1].Text() != current.Command.Instruction ||
		messages[2].Role != llm.RoleAssistant || messages[2].Text() != decision.ReplyText {
		t.Fatalf("unexpected model history after reply replay: %+v, %v", messages, err)
	}
}

func TestChatReplyPublicationSurvivesRestart(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	current, err := s.GetRun(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	input := model.InputDecision{InputID: "chat-input", Disposition: model.InputChat, Fingerprint: "request", ReplyText: "你好！", ExecutionRevision: current.ExecutionRevision}
	if _, err := s.SaveInputDecision(ctx, current, input); err != nil {
		t.Fatal(err)
	}
	payload := model.MessageReplyPayload{PublicEventBase: model.NewPublicEventBase(current.ID), MessageID: "input_chat-input", InputID: input.InputID, Text: input.ReplyText}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	event := model.Event{RunID: current.ID, Type: model.EventMessageReply, Payload: string(raw)}
	if err := s.AppendInputReply(ctx, &event, input.InputID); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(s.db)
	if err != nil {
		t.Fatal(err)
	}
	// A later decision and a repeated publication cannot replace the issued reply.
	saved, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: input.InputID, Disposition: model.InputTask, Fingerprint: "changed"})
	if err != nil || saved.Disposition != model.InputChat || saved.ReplyEventSeq != event.Seq || saved.ReplyText != input.ReplyText {
		t.Fatalf("published reply replaced: %+v %v", saved, err)
	}
	retry := model.Event{RunID: current.ID, Type: event.Type}
	var replayed model.MessageReplyPayload
	if err := s.AppendInputReply(ctx, &retry, input.InputID); err != nil || retry.Seq != event.Seq || retry.CreatedAt != event.CreatedAt ||
		json.Unmarshal([]byte(retry.Payload), &replayed) != nil || replayed != payload {
		t.Fatalf("retry changed reply: %+v %v", retry, err)
	}
	events, err := s.EventsSince(ctx, current.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range events {
		if item.Type == model.EventMessageReply {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("published %d chat replies", count)
	}
}
