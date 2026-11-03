package sqlite

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestInitialReplyOnlyEntersPublicHistory(t *testing.T) {
	for _, tc := range []struct {
		input    string
		decision model.InputDecision
		kind     model.EventType
	}{
		{"  你能做什么？\n", model.InputDecision{Disposition: model.InputChat, ReplyText: "我能帮你制作 HTML 演示文稿。"}, model.EventMessageFinal},
		{"告诉我 NBA 最新消息", model.InputDecision{Disposition: model.InputIrrelevant}, model.EventMessageFinal},
		{"告诉我 NBA 最新消息", model.InputDecision{Disposition: model.InputIrrelevant}, model.EventMessageReply},
	} {
		t.Run(tc.decision.Disposition+"/"+string(tc.kind), func(t *testing.T) {
			s := newTestStore(t)
			seedProject(t, s)
			ctx := sessionstate.WithRequestScene(context.Background(), 1)
			if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p1"}); err != nil {
				t.Fatal(err)
			}
			prior := llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("上一轮的答复")}
			_, err := s.commitSession(ctx, sessionstate.Fence{ThreadID: "t", ProjectID: "p1", SceneRevision: 1}, sessionstate.CommitIdentity{Key: "prior", RequestHash: "prior"}, func(w *sessionWrite) error {
				return w.replaceContext(sessionstate.ContextUpdate{ExpectedRevision: 0, Reason: "append", MessageIDs: []string{"prior"}, NewMessages: []sessionstate.Message{{ID: "prior", RoundID: "context:t", Kind: "assistant", Native: prior}}})
			})
			if err != nil {
				t.Fatal(err)
			}
			current := model.Run{ID: "r", ThreadID: "t", ProjectID: "p1", SceneRevision: 1, ClientRequestID: "greet", Status: model.RunRunning, OwnerInstanceID: "owner", ExecutionRevision: 1, Command: model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeChat, Instruction: tc.input}}
			if err := s.CreateRun(ctx, current); err != nil {
				t.Fatal(err)
			}
			current, err = s.GetRun(ctx, "r")
			if err != nil {
				t.Fatal(err)
			}
			decision := tc.decision
			decision.InputID, decision.Fingerprint, decision.ExecutionRevision = "initial:greet", "request", current.ExecutionRevision
			if _, err := s.SaveInputDecision(ctx, current, decision); err != nil {
				t.Fatal(err)
			}
			var payload any = model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase("r"), MessageID: "input_greet", Text: decision.Reply(), AffectedTargets: []model.PublicTarget{}}
			if tc.kind == model.EventMessageReply {
				payload = model.MessageReplyPayload{PublicEventBase: model.NewPublicEventBase("r"), InputID: decision.InputID, MessageID: "input_" + decision.InputID, Text: decision.Reply()}
			}
			fact, err := sessionstate.NewRunUpdate(tc.kind, payload)
			if err != nil {
				t.Fatal(err)
			}
			first, err := s.CommitInputReply(ctx, "r", decision.InputID, fact)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := s.CommitInputReply(ctx, "r", decision.InputID, fact)
			if err != nil || repeated.CommitID != first.CommitID {
				t.Fatalf("repeated reply changed receipt: %+v %v", repeated, err)
			}
			snapshot, err := s.LoadContext(ctx, "t")
			if err != nil || snapshot.Revision != 1 || len(snapshot.Messages) != 1 || snapshot.Messages[0].Native.Text() != prior.Text() {
				t.Fatalf("direct reply entered task context: %+v %v", snapshot, err)
			}
			public, err := s.LoadThreadConversationContext(ctx, "t")
			if err != nil || len(public.Rounds) != 1 || len(public.Rounds[0].Messages) != 2 || public.Rounds[0].Messages[0].Text != tc.input || public.Rounds[0].Messages[1].Text != decision.Reply() {
				t.Fatalf("direct exchange lost: %+v %v", public, err)
			}
		})
	}
}

func TestChatReplyPublicationSurvivesRestart(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	current, err := s.GetRun(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	input := model.InputDecision{InputID: "initial:" + current.ClientRequestID, Disposition: model.InputChat, Fingerprint: "request", ReplyText: "你好！", ExecutionRevision: current.ExecutionRevision}
	if _, err := s.SaveInputDecision(ctx, current, input); err != nil {
		t.Fatal(err)
	}
	fact, err := sessionstate.NewRunUpdate(model.EventMessageReply, model.MessageReplyPayload{PublicEventBase: model.NewPublicEventBase(current.ID), MessageID: "input_" + input.InputID, InputID: input.InputID, Text: input.ReplyText})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.CommitInputReply(ctx, current.ID, input.InputID, fact)
	if err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(s.db)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: input.InputID, Disposition: model.InputTask, Fingerprint: "changed", ExecutionRevision: current.ExecutionRevision})
	if err != nil || saved.Disposition != model.InputChat || saved.ReplyMessageID == "" || saved.ReplyText != input.ReplyText {
		t.Fatalf("published reply replaced: %+v %v", saved, err)
	}
	repeated, err := s.CommitInputReply(ctx, current.ID, input.InputID, fact)
	if err != nil || repeated.CommitID != receipt.CommitID {
		t.Fatalf("restart retry changed receipt: %+v %v", repeated, err)
	}
	var count int64
	if err := s.reader.Table("conversation_messages").Where("run_id=? AND kind='direct_reply'", current.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("published %d direct replies: %v", count, err)
	}
}
