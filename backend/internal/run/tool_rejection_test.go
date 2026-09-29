package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type rejectionAgent struct {
	calls       []llm.ToolCall
	requests    []workflow.AgentRequest
	correctRole bool
}

func (a *rejectionAgent) Next(_ context.Context, request workflow.AgentRequest) (workflow.AgentResponse, error) {
	a.requests = append(a.requests, request)
	if len(a.requests) == 1 {
		return workflow.AgentResponse{ToolCalls: a.calls}, nil
	}
	if len(a.requests) == 2 && a.correctRole {
		return workflow.AgentResponse{ToolCalls: []llm.ToolCall{{ID: "corrected", Name: "edit_spec", Args: map[string]any{"slide_id": "sli_page", "role": "cover"}}}}, nil
	}
	return workflow.AgentResponse{ToolCalls: []llm.ToolCall{{ID: "finish_task", Name: "finish_task", Args: map[string]any{"message": "检查完成"}}}}, nil
}

// Exercise the real Runtime -> emitter -> Bus path, not an event-only recorder.
func TestRejectedToolsReachAgentWithoutPausingRun(t *testing.T) {
	command := func(id string, args map[string]any) llm.ToolCall {
		return llm.ToolCall{ID: id, Name: "run_command", Args: args}
	}
	cases := []struct {
		name        string
		calls       []llm.ToolCall
		code        string
		correctRole bool
	}{
		{"invalid role", []llm.ToolCall{{ID: "invalid", Name: "edit_spec", Args: map[string]any{"slide_id": "sli_page", "role": "invalid-role"}}}, workflow.CodeToolArgumentInvalid, true},
		{"invalid read", []llm.ToolCall{{ID: "invalid", Name: "read_resource", Args: map[string]any{"resource": "spec"}}}, workflow.CodeToolArgumentInvalid, false},
		{"missing command", []llm.ToolCall{command("invalid", map[string]any{})}, workflow.CodeToolArgumentInvalid, false},
		{"wrong command type", []llm.ToolCall{command("invalid", map[string]any{"command": 123})}, workflow.CodeToolArgumentInvalid, false},
		{"policy denial", []llm.ToolCall{command("denied", map[string]any{"command": "curl https://example.com"})}, "COMMAND_NOT_ALLOWED", false},
	}
	for _, count := range []int{2, 3} {
		calls := []llm.ToolCall{}
		for i := 0; i < count; i++ {
			calls = append(calls, command(fmt.Sprintf("confirm-%d", i), map[string]any{"command": "cat .env"}))
		}
		cases = append(cases, struct {
			name        string
			calls       []llm.ToolCall
			code        string
			correctRole bool
		}{fmt.Sprintf("%d approvals in batch", count), calls, workflow.CodeInvalidControlCall, false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			outline := spec.Outline{Sections: []spec.Section{{ID: "sec_one", Title: "Section", Purpose: "Test", Slides: []spec.SlideNode{{SlideID: "sli_page", Title: "Page"}}, Subsections: []spec.Subsection{}}}}
			for name, value := range map[string]any{
				".outline.json":          outline,
				model.SpecCollectionPath: map[string]spec.SlideSpec{"sli_page": {Role: "cover", KeyMessage: "Message", Elements: []spec.Element{}}},
			} {
				raw, _ := json.Marshal(value)
				if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PRIVATE_VALUE=never_read"), 0o600); err != nil {
				t.Fatal(err)
			}
			pack := contextengine.ContextPack{
				SchemaVersion: contextengine.SchemaVersion,
				Project:       contextengine.ProjectContext{ID: "project", Title: "Deck"},
				Command:       model.RunCommand{Mode: model.ModeExecute, Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_page"), Instruction: "检查当前页"},
				Outline:       contextengine.OutlineContext{Outline: outline},
				Target:        contextengine.TargetContext{SlideIDs: []string{"sli_page"}},
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := newMemStore()
			row := model.Run{ID: "rejected", Status: model.RunRunning, OwnerInstanceID: "owner"}
			if err := store.CreateRun(ctx, row); err != nil {
				t.Fatal(err)
			}
			bus := NewBus(row.ID, "thread", store)
			a := &active{run: row, cancel: cancel}
			emitter := &workflowEmitter{ctx: ctx, bus: bus, cancel: cancel, active: a}
			emitter.Emit(model.EventRunStarted, model.RunStartedPayload{PublicEventBase: model.NewPublicEventBase(row.ID), Mode: pack.Command.Mode, Scope: pack.Command.Scope, UserInput: pack.Command.Instruction})
			agent := &rejectionAgent{calls: tc.calls, correctRole: tc.correctRole}
			outcome := workflow.NewRuntime(agent).Run(ctx, workflow.RuntimeInput{RunID: row.ID, ProjectDir: dir, Context: pack, Emitter: emitter})
			if emitter.Err() != nil || ctx.Err() != nil || a.pauseRequested || outcome.Status != workflow.StatusCompleted {
				t.Fatalf("rejection interrupted run: outcome=%+v emitter=%v ctx=%v paused=%v", outcome, emitter.Err(), ctx.Err(), a.pauseRequested)
			}
			if len(agent.requests) < 2 {
				t.Fatal("agent never received repair feedback")
			}
			feedback, _ := json.Marshal(agent.requests[1].Messages)
			if !strings.Contains(string(feedback), tc.code) || strings.Contains(string(feedback), "PRIVATE_VALUE=never_read") {
				t.Fatalf("missing error or unauthorized command executed: %s", feedback)
			}
			events, _ := store.EventsSince(ctx, row.ID, 0)
			blocked := 0
			correctedStarted, correctedCompleted := 0, 0
			for _, event := range events {
				var payload model.ToolCompletedPayload
				_ = json.Unmarshal([]byte(event.Payload), &payload)
				if payload.CallID == "corrected" {
					if event.Type == model.EventToolStarted {
						correctedStarted++
					}
					if event.Type == model.EventToolCompleted && payload.Status == "completed" && payload.Error == nil {
						correctedCompleted++
					}
				}
				if event.Type == model.EventToolStarted && payload.CallID != "corrected" {
					t.Fatalf("rejected call started: %s", event.Payload)
				}
				if event.Type == model.EventToolCompleted && payload.CallID != "corrected" {
					blocked++
					if payload.Status != "blocked" || payload.Error == nil || payload.Error.Code != tc.code {
						t.Fatalf("bad blocked event: %+v", payload)
					}
					if payload.Command != nil && (payload.Command.Status != "blocked" || payload.Command.ExitCode != nil || payload.Command.DurationMS != nil) {
						t.Fatalf("blocked command claims execution: %+v", payload.Command)
					}
				}
			}
			if blocked != len(tc.calls) {
				t.Fatalf("blocked=%d calls=%d", blocked, len(tc.calls))
			}
			if tc.correctRole && (correctedStarted != 1 || correctedCompleted != 1) {
				t.Fatalf("corrected tool did not execute successfully: started=%d completed=%d", correctedStarted, correctedCompleted)
			}
			if err := NewBus(row.ID, "thread", store).Restore(events); err != nil {
				t.Fatalf("history cannot replay: %v", err)
			}
		})
	}
}

type pauseCountingStore struct {
	*memStore
	pauses        int
	failPauseOnce bool
	failAppend    bool
}

func (s *pauseCountingStore) PauseRun(ctx context.Context, id, owner, reason string, at int64) (model.Run, error) {
	s.pauses++
	if s.failPauseOnce && s.pauses == 1 {
		return model.Run{}, errors.New("pause write failed")
	}
	return s.memStore.PauseRun(ctx, id, owner, reason, at)
}
func (s *pauseCountingStore) AppendEvent(ctx context.Context, event *model.Event) error {
	if s.failAppend {
		return errors.New("journal disk failure")
	}
	return s.memStore.AppendEvent(ctx, event)
}

func TestEmitterPausesOnceAndDistinguishesProtocolFromStorageFailure(t *testing.T) {
	for _, failPause := range []bool{false, true} {
		for _, failAppend := range []bool{false, true} {
			store := &pauseCountingStore{memStore: newMemStore(), failPauseOnce: failPause, failAppend: failAppend}
			row := model.Run{ID: "failure", Status: model.RunRunning, OwnerInstanceID: "owner"}
			_ = store.CreateRun(context.Background(), row)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			emitter := &workflowEmitter{ctx: ctx, cancel: cancel, bus: NewBus(row.ID, "thread", store), active: &active{run: row, cancel: cancel}}
			if failAppend {
				emitter.Emit(model.EventRunStarted, model.RunStartedPayload{PublicEventBase: model.NewPublicEventBase(row.ID), Mode: model.ModeExecute, Scope: model.NewRunScope(model.ScopeAllPages), UserInput: "test"})
			} else {
				emitter.Emit(model.EventToolCompleted, model.ToolCompletedPayload{})
			}
			if emitter.Err() == nil || ctx.Err() == nil {
				t.Fatal("delivery failure did not cancel execution")
			}
			for i := 0; i < 2; i++ {
				if err := emitter.pauseAfterFailure(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			wantPauses := 1
			if failPause {
				wantPauses = 2
			}
			got, _ := store.GetRun(context.Background(), row.ID)
			wantReason := "event_protocol_invalid"
			if failAppend {
				wantReason = "journal_write_failed"
			}
			if store.pauses != wantPauses || got.Status != model.RunPaused || got.PauseReason != wantReason {
				t.Fatalf("pauses=%d run=%+v", store.pauses, got)
			}
		}
	}
}
