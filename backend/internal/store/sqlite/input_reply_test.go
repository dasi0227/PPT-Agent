package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func acceptFixedReplyInput(t *testing.T, s *Store, current model.Run, id, disposition string) model.InputDecision {
	t.Helper()
	ctx := context.Background()
	if _, _, err := s.CreateSteering(ctx, model.SteeringMessage{RunID: current.ID, ThreadID: current.ThreadID, ClientMessageID: id, RequestHash: id, Content: "input", Scope: current.Command.Scope}); err != nil {
		t.Fatal(err)
	}
	decision, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: id, Disposition: disposition, Fingerprint: id, ExecutionRevision: current.ExecutionRevision})
	if err != nil || decision.Reply() != "" {
		t.Fatalf("reply was chosen before event persistence: %+v %v", decision, err)
	}
	return decision
}

func fixedReplyEvent(t *testing.T, current model.Run, inputID string) model.Event {
	t.Helper()
	payload := model.MessageReplyPayload{PublicEventBase: model.NewPublicEventBase(current.ID), MessageID: "input_" + inputID, InputID: inputID}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return model.Event{RunID: current.ID, Type: model.EventMessageReply, Payload: string(raw)}
}

func TestFixedRepliesKeepPerCategoryStateAcrossRunsAndStoreRestart(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	current, err := s.GetRun(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	last := map[string]string{}
	for round := 0; round < 6; round++ {
		for _, category := range []string{model.InputGreeting, model.InputIdentity, model.InputIrrelevant} {
			id := fmt.Sprintf("%s-%d", category, round)
			decision := acceptFixedReplyInput(t, s, current, id, category)
			event := fixedReplyEvent(t, current, id)
			if category == model.InputIrrelevant {
				terminal := model.NewRunTerminalPayload(current.ID, 0, nil, nil)
				terminal.Reason = model.RunCancelInputRejected
				terminal.Reply = &model.InputReply{InputID: id}
				raw, err := json.Marshal(terminal)
				if err != nil {
					t.Fatal(err)
				}
				event.Type, event.Payload = model.EventRunCanceled, string(raw)
			}
			if err := s.AppendInputReply(ctx, &event, id); err != nil {
				t.Fatal(err)
			}
			record, err := s.GetIdempotency(ctx, "input_decision", current.ID, id)
			var saved model.InputDecision
			if err != nil || json.Unmarshal([]byte(record.ResultJSON), &saved) != nil || !slices.Contains(decision.ReplyCandidates(), saved.Reply()) || saved.Reply() == last[category] || saved.ReplyEventSeq != event.Seq {
				t.Fatalf("repeated or unpersisted %s reply: %+v %v", category, saved, err)
			}
			if category == model.InputIrrelevant {
				var terminal model.RunTerminalPayload
				if err := json.Unmarshal([]byte(event.Payload), &terminal); err != nil || terminal.Reply == nil || terminal.Reply.Text != saved.Reply() {
					t.Fatalf("terminal did not use the exact candidate: %+v %v", terminal, err)
				}
			}
			last[category] = saved.Reply()
		}
		// A fresh Store has no in-memory selection state. The same thread's
		// category state must survive while the next Run gets new input IDs.
		s, err = NewStore(s.db)
		if err != nil {
			t.Fatal(err)
		}
		next := model.Run{ID: fmt.Sprintf("next-%d", round), ThreadID: "t", ProjectID: "p", ClientRequestID: fmt.Sprintf("next-request-%d", round), OwnerInstanceID: "owner", Status: model.RunRunning, Command: current.Command}
		if err := s.CreateRun(ctx, next); err != nil {
			t.Fatal(err)
		}
		current, err = s.GetRun(ctx, next.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	initialID := "initial:" + current.ClientRequestID
	if _, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: initialID, Disposition: model.InputGreeting, Fingerprint: initialID, ExecutionRevision: current.ExecutionRevision}); err != nil {
		t.Fatal(err)
	}
	final := model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(current.ID), MessageID: "input_" + current.ClientRequestID, AffectedTargets: []model.PublicTarget{}}
	raw, err := json.Marshal(final)
	if err != nil {
		t.Fatal(err)
	}
	initialEvent := model.Event{RunID: current.ID, Type: model.EventMessageFinal, Payload: string(raw)}
	if err := s.AppendInputReply(ctx, &initialEvent, initialID); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(initialEvent.Payload), &final); err != nil || final.Text == "" || final.Text == last[model.InputGreeting] {
		t.Fatalf("initial final reply did not share the category state: %+v %v", final, err)
	}
	last[model.InputGreeting] = final.Text
	if err := s.SetRunStatus(ctx, current.ID, model.RunDone); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "other", ProjectID: "p"}); err != nil {
		t.Fatal(err)
	}
	other := model.Run{ID: "other-run", ThreadID: "other", ProjectID: "p", ClientRequestID: "other-request", OwnerInstanceID: "owner", Status: model.RunRunning, Command: current.Command}
	if err := s.CreateRun(ctx, other); err != nil {
		t.Fatal(err)
	}
	other, err = s.GetRun(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	acceptFixedReplyInput(t, s, other, "other-input", model.InputGreeting)
	event := fixedReplyEvent(t, other, "other-input")
	if err := s.AppendInputReply(ctx, &event, "other-input"); err != nil {
		t.Fatal(err)
	}
	for category, text := range last {
		record, err := s.GetIdempotency(ctx, "input_reply_choice", "t", category)
		var state inputReplyChoice
		if err != nil || json.Unmarshal([]byte(record.ResultJSON), &state) != nil || state.Text != text {
			t.Fatalf("another thread changed %s state: %+v %v", category, state, err)
		}
	}
}

func TestConcurrentFixedRepliesFollowEventOrderAndReplayOnlyOnce(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	current, err := s.GetRun(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	const count = 16
	events := make([]model.Event, count)
	for i := range events {
		id := fmt.Sprintf("concurrent-%d", i)
		acceptFixedReplyInput(t, s, current, id, model.InputGreeting)
		events[i] = fixedReplyEvent(t, current, id)
	}
	errors := make(chan error, count)
	var workers sync.WaitGroup
	// Deliberately publish in a different order from acceptance/classification.
	for i := count - 1; i >= 0; i-- {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			errors <- s.AppendInputReply(ctx, &events[i], fmt.Sprintf("concurrent-%d", i))
		}(i)
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	history, err := s.EventsSince(ctx, current.ID, 0)
	if err != nil || len(history) != count {
		t.Fatalf("missing replies: %d %v", len(history), err)
	}
	last := ""
	for _, event := range history {
		var reply model.MessageReplyPayload
		if json.Unmarshal([]byte(event.Payload), &reply) != nil || reply.Text == last || !slices.Contains((model.InputDecision{Disposition: model.InputGreeting}).ReplyCandidates(), reply.Text) {
			t.Fatalf("concurrent replies repeated in actual event order: %+v", event)
		}
		last = reply.Text
	}
	before, err := s.GetIdempotency(ctx, "input_reply_choice", "t", model.InputGreeting)
	if err != nil {
		t.Fatal(err)
	}
	retry := fixedReplyEvent(t, current, "concurrent-0")
	if err := s.AppendInputReply(ctx, &retry, "concurrent-0"); err != nil || retry.Seq != events[0].Seq || retry.Payload != events[0].Payload {
		t.Fatalf("retry changed the committed reply: %+v %v", retry, err)
	}
	after, err := s.GetIdempotency(ctx, "input_reply_choice", "t", model.InputGreeting)
	if err != nil || before.ResultJSON != after.ResultJSON {
		t.Fatal("replay advanced the category state")
	}
	history, err = s.EventsSince(ctx, current.ID, 0)
	if err != nil || len(history) != count {
		t.Fatalf("retry appended another event: %d %v", len(history), err)
	}
	saved, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: "concurrent-0", Disposition: model.InputIdentity, Fingerprint: "changed", ExecutionRevision: current.ExecutionRevision})
	if err != nil || saved.Disposition != model.InputGreeting || saved.ReplyEventSeq != retry.Seq || saved.Reply() == "" {
		t.Fatalf("reclassification replaced an issued reply: %+v %v", saved, err)
	}
}
