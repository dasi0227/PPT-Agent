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
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func TestRejectedToolsReachAgentWithoutPausingRun(t *testing.T) {
	command := func(id string, args map[string]any) llm.ToolCall {
		return llm.ToolCall{ID: id, Name: "run_command", Args: args}
	}
	cases := []struct {
		name            string
		calls           []llm.ToolCall
		code            string
		candidateWrites bool
	}{
		{"policy denial", []llm.ToolCall{command("denied", map[string]any{"command": "git push"})}, "COMMAND_NOT_ALLOWED", false},
		{"multiple write approvals", []llm.ToolCall{
			command("confirm-0", map[string]any{"command": "cat .env; printf candidate > notes-0.txt"}),
			command("confirm-1", map[string]any{"command": "cat .env; printf candidate > notes-1.txt"}),
		}, workflow.CodeToolCallInvalid, true},
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
			agent := &rejectionAgent{calls: tc.calls}
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
			for _, event := range events {
				var payload model.ToolCompletedPayload
				_ = json.Unmarshal([]byte(event.Payload), &payload)
				if event.Type == model.EventToolStarted {
					if !tc.candidateWrites {
						t.Fatalf("rejected call started: %s", event.Payload)
					}
					candidateStarts++
				}
				if event.Type == model.EventToolCompleted {
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

		})
	}
}
