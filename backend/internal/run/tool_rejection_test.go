package run

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type rejectionAgent struct {
	calls          []llm.ToolCall
	requests       []workflow.AgentRequest
	correctPurpose bool
}

func (a *rejectionAgent) Next(_ context.Context, request workflow.AgentRequest) (workflow.AgentResponse, error) {
	a.requests = append(a.requests, request)
	if len(a.requests) == 1 {
		return workflow.AgentResponse{ToolCalls: a.calls}, nil
	}
	if len(a.requests) == 2 && a.correctPurpose {
		return workflow.AgentResponse{ToolCalls: []llm.ToolCall{{ID: "refresh-spec", Name: "read_resource", Args: map[string]any{"type": "spec", "slide_id": "sli_page"}}}}, nil
	}
	if len(a.requests) == 3 && a.correctPurpose {
		return workflow.AgentResponse{ToolCalls: []llm.ToolCall{{ID: "corrected", Name: "edit_spec", Args: map[string]any{"slide_id": "sli_page", "purpose": "cover"}}}}, nil
	}
	return workflow.AgentResponse{Text: "检查完成"}, nil
}

// Exercise the real Runtime -> emitter -> Bus path, not an event-only recorder.
func TestRejectedToolsReachAgentWithoutPausingRun(t *testing.T) {
	command := func(id string, args map[string]any) llm.ToolCall {
		return llm.ToolCall{ID: id, Name: "run_command", Args: args}
	}
	cases := []struct {
		name            string
		calls           []llm.ToolCall
		code            string
		correctPurpose  bool
		candidateWrites bool
	}{
		{"invalid purpose", []llm.ToolCall{{ID: "invalid", Name: "edit_spec", Args: map[string]any{"slide_id": "sli_page", "purpose": "invalid-purpose"}}}, workflow.CodeToolArgumentInvalid, true, false},
		{"invalid read", []llm.ToolCall{{ID: "invalid", Name: "read_resource", Args: map[string]any{"type": "spec"}}}, workflow.CodeToolArgumentInvalid, false, false},
		{"missing command", []llm.ToolCall{command("invalid", map[string]any{})}, workflow.CodeToolArgumentInvalid, false, false},
		{"wrong command type", []llm.ToolCall{command("invalid", map[string]any{"command": 123})}, workflow.CodeToolArgumentInvalid, false, false},
		{"policy denial", []llm.ToolCall{command("denied", map[string]any{"command": "git push"})}, "COMMAND_NOT_ALLOWED", false, false},
		{"multiple write approvals", []llm.ToolCall{
			command("confirm-0", map[string]any{"command": "cat .env; printf candidate > notes-0.txt"}),
			command("confirm-1", map[string]any{"command": "cat .env; printf candidate > notes-1.txt"}),
		}, workflow.CodeToolCallInvalid, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			outline := spec.Outline{Sections: []spec.Section{{ID: "sec_one", Title: "Section", Slides: []spec.SlideNode{{ID: "sli_page", Title: "Page"}}, Subsections: []spec.Subsection{}}}}
			for name, value := range map[string]any{
				".outline.json":                 outline,
				model.SlideSpecPath("sli_page"): spec.SlideSpec{Purpose: "content_explanation", Layout: "Title above content", Core: "Message", Elements: []spec.Element{}},
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
				Project: contextengine.ProjectContext{ID: "project", Title: "Deck"},
				Command: model.RunCommand{Mode: model.ModeCreate, Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_page"), Instruction: "检查当前页"},
				Outline: contextengine.OutlineContext{Outline: outline},
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := newMemStore()
			row := model.Run{ID: "rejected", Status: model.RunRunning, OwnerInstanceID: "owner"}
			if err := store.CreateRun(ctx, row); err != nil {
				t.Fatal(err)
			}
			bus := NewBus(row, store)
			a := &active{run: row, cancel: cancel}
			emitter := &workflowEmitter{ctx: ctx, bus: bus, cancel: cancel, active: a}
			emitter.Emit(model.EventRunStarted, model.RunStartedPayload{PublicEventBase: model.NewPublicEventBase(row.ID), Mode: pack.Command.Mode, Scope: pack.Command.Scope, UserInput: pack.Command.Instruction})
			agent := &rejectionAgent{calls: tc.calls, correctPurpose: tc.correctPurpose}
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
			completed, candidateStarts := 0, 0
			correctedStarted, correctedCompleted := 0, 0
			for _, event := range events {
				var payload model.ToolCompletedPayload
				_ = json.Unmarshal([]byte(event.Payload), &payload)
				if payload.CallID == "refresh-spec" {
					if event.Type == model.EventToolCompleted && payload.Error != nil {
						t.Fatalf("spec read failed: %+v", payload.Error)
					}
					continue
				}
				if payload.CallID == "corrected" {
					if event.Type == model.EventToolStarted {
						correctedStarted++
					}
					if event.Type == model.EventToolCompleted && payload.Error != nil {
						t.Fatalf("corrected tool failed: %+v", payload.Error)
					}
					if event.Type == model.EventToolCompleted && payload.Status == "completed" && payload.Error == nil {
						correctedCompleted++
					}
				}
				if event.Type == model.EventToolStarted && payload.CallID != "corrected" {
					if !tc.candidateWrites {
						t.Fatalf("rejected call started: %s", event.Payload)
					}
					candidateStarts++
				}
				if event.Type == model.EventToolCompleted && payload.CallID != "corrected" {
					completed++
					wantStatus := "blocked"
					if tc.candidateWrites {
						wantStatus = "failed"
					}
					if payload.Status != wantStatus || payload.Error == nil || payload.Error.Code != tc.code {
						t.Fatalf("bad rejected event: %+v", payload)
					}
					if payload.Command != nil {
						if payload.Command.Status != wantStatus || payload.Command.StdoutPreview != "" || payload.Command.StderrPreview != "" {
							t.Fatalf("rejected command exposed candidate output: %+v", payload.Command)
						}
						if !tc.candidateWrites && (payload.Command.ExitCode != nil || payload.Command.DurationMS != nil) {
							t.Fatalf("blocked command claims execution: %+v", payload.Command)
						}
					}
				}
			}
			if completed != len(tc.calls) {
				t.Fatalf("completed=%d calls=%d", completed, len(tc.calls))
			}
			if tc.candidateWrites {
				if candidateStarts != len(tc.calls) {
					t.Fatalf("candidate starts=%d calls=%d", candidateStarts, len(tc.calls))
				}
				for _, name := range []string{"notes-0.txt", "notes-1.txt"} {
					if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("unapproved batch changed project file %s: %v", name, err)
					}
				}
			}
			if tc.correctPurpose && (correctedStarted != 1 || correctedCompleted != 1) {
				t.Fatalf("corrected tool did not execute successfully: started=%d completed=%d", correctedStarted, correctedCompleted)
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
func (s *pauseCountingStore) ApplyRunUpdate(ctx context.Context, runID string, update sessionstate.RunUpdate) (sessionstate.Receipt, error) {
	if s.failAppend {
		return sessionstate.Receipt{}, errors.New("database commit failure")
	}
	return s.memStore.ApplyRunUpdate(ctx, runID, update)
}

func TestEmitterPausesOnceAndDistinguishesProtocolFromStorageFailure(t *testing.T) {
	for _, failPause := range []bool{false, true} {
		for _, failAppend := range []bool{false, true} {
			store := &pauseCountingStore{memStore: newMemStore(), failPauseOnce: failPause, failAppend: failAppend}
			row := model.Run{ID: "failure", Status: model.RunRunning, OwnerInstanceID: "owner"}
			_ = store.CreateRun(context.Background(), row)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			emitter := &workflowEmitter{ctx: ctx, cancel: cancel, bus: NewBus(row, store), active: &active{run: row, cancel: cancel}}
			if failAppend {
				emitter.Emit(model.EventRunStarted, model.RunStartedPayload{PublicEventBase: model.NewPublicEventBase(row.ID), Mode: model.ModeCreate, Scope: model.NewRunScope(model.ScopeAllPages), UserInput: "test"})
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
				wantReason = "session_commit_failed"
			}
			if store.pauses != wantPauses || got.Status != model.RunPaused || got.PauseReason != wantReason {
				t.Fatalf("pauses=%d run=%+v", store.pauses, got)
			}
		}
	}
}
