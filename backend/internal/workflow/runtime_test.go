package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestPostCommitCheckpointFailureDoesNotPublishUncommittedCompletion(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	checkpoints := &postCommitFailingCheckpoint{}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("write", "edit_spec", map[string]any{"slide_id": "sli_1", "content": "committed"}),
		finalReply(),
	}}
	outcome := newTestRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "post-commit-checkpoint", ProjectDir: dir,
		Context:        testPack(model.ModeCreate, model.ScopeCurrentPage, false, "修改当前页标题"),
		DomainTools:    fakeProvider{kind: ArtifactSlideSpec},
		Checkpoint:     checkpoints,
		CommitMetadata: func(context.Context, CommitContext) error { return nil },
	})
	if outcome.Status == StatusCompleted || checkpoints.failures == 0 {
		t.Fatalf("outcome=%+v checkpoint_failures=%d", outcome, checkpoints.failures)
	}
	raw, err := readTestSpecText(dir)
	if err != nil || string(raw) != "committed" {
		t.Fatalf("committed artifact missing: raw=%q err=%v", raw, err)
	}
}

func TestSuccessfulToolIsDurableBeforeNextModelAndCompletion(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	commits, turns := 0, 0
	agent := planTestAgentFunc(func(context.Context, AgentRequest) (AgentResponse, error) {
		turns++
		if turns == 1 {
			return toolCall("stage", "edit_spec", map[string]any{"slide_id": "sli_1", "content": "committed", "evidence": false}), nil
		}
		raw, err := readTestSpecText(dir)
		if err != nil || string(raw) != "committed" || commits != 1 {
			t.Fatalf("the next model call preceded the committed tool product: content=%q commits=%d err=%v", raw, commits, err)
		}
		return finalReply(), nil
	})
	outcome := newTestRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "commit", ProjectDir: dir,
		Context:        testPack(model.ModeCreate, model.ScopeCurrentPage, false, "修改当前页标题"),
		DomainTools:    fakeProvider{kind: ArtifactSlideSpec},
		CommitMetadata: func(context.Context, CommitContext) error { commits++; return nil },
	})
	if outcome.Status != StatusCompleted || commits != 1 || turns != 2 {
		t.Fatalf("outcome=%+v commits=%d turns=%d", outcome, commits, turns)
	}
}

func TestDeniedCommandWriteDoesNotSaveChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	events := &eventRecorder{}
	prompter := &commandPermissionPrompter{decision: "deny", feedback: "  保留原文\n改用公开信息  "}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("read-env", "run_command", map[string]any{"command": "cat .env; printf saved > notes.txt"}),
		finalReply(),
	}}
	outcome := newTestRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "command-write-deny", ProjectDir: dir,
		Context: testPack(model.ModeCreate, model.ScopeAllPages, false, "检查环境文件"),
		Emitter: events, Prompter: prompter,
	})
	if outcome.Status != StatusCompleted || len(prompter.requests) != 1 {
		t.Fatalf("outcome=%+v requests=%+v", outcome, prompter.requests)
	}
	if events.count(model.EventToolStarted) != 1 || events.count(model.EventToolCompleted) != 1 {
		t.Fatalf("events=%+v", events.events)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("denied candidate changed project files: %v", err)
	}
	for _, request := range agent.requests {
		raw, _ := json.Marshal(request)
		if strings.Contains(string(raw), "TOKEN=secret") {
			t.Fatalf("denied command output reached the model: %s", raw)
		}
	}
	var reply map[string]any
	for _, message := range agent.requests[1].Messages {
		if message.Role == llm.RoleTool && message.ToolCallID == "read-env" {
			_ = json.Unmarshal([]byte(message.Text()), &reply)
		}
	}
	if reply["status"] != "refuse" || reply["decision"] != nil || reply["code"] != nil || reply["feedback"] != prompter.feedback {
		t.Fatalf("denial feedback missing from model: %v", reply)
	}
	for _, event := range events.events {
		if event.kind == model.EventToolCompleted {
			payload := event.payload.(model.ToolCompletedPayload)
			if payload.Status != "refuse" || payload.Error != nil || payload.Command == nil || payload.Command.Status != "blocked" || payload.Command.ExitCode != nil || payload.Command.DurationMS != nil || payload.Command.StdoutPreview != "TOKEN=secret" {
				t.Fatalf("refusal was projected as an error or as an executed command: %+v", payload)
			}
		}
	}
}

func TestPendingCommandRecoveryReopensPendingApproval(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const runID = "command-recovery"

	args := map[string]any{"command": `printf '%s' "$RANDOM-$RANDOM" | tee notes.txt`}
	decision := (projectCommandTool{}).Preflight(context.Background(), DomainToolInput{
		Args: args, CallID: "read-env", ProjectDir: resumedProject(t, dir, runID),
		RunID: runID, Mode: model.ModeCreate, Phase: PhaseExecuting,
	})
	if decision.Outcome != "confirm" {
		t.Fatalf("decision=%+v", decision)
	}
	prepared := decision.Prepared.(preparedCommand)
	checkpoint := &RuntimeCheckpoint{
		Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_1"),
		RunID: runID, LoopID: "loop-recovery",
		Phase: PhaseWaitingInput, ResumePhase: PhaseExecuting, Mode: model.ModeCreate,
		PendingCommand: &PendingCommandApproval{
			InteractionID: "cmdperm_read-env",
			CallID:        "read-env",
			Args:          args, Prepared: &prepared,
			ResumePhase: PhaseExecuting,
		},
	}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	var restored RuntimeCheckpoint
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	checkpoint = &restored
	prompter := &commandPermissionPrompter{decision: "allow_once"}
	agent := &scriptedAgent{responses: []AgentResponse{finalReply()}}
	runtime := newTestRuntime(agent)
	runtime.HighRiskApprovalRequired = func() (bool, error) { return false, nil }
	outcome := runtime.Run(context.Background(), RuntimeInput{
		RunID: runID, ProjectDir: resumedProject(t, dir, runID),
		Context:          testPack(model.ModeCreate, model.ScopeCurrentPage, false, "继续运行"),
		Prompter:         prompter,
		ResumeCheckpoint: checkpoint,
	})
	if outcome.Status != StatusCompleted || len(prompter.requests) != 1 {
		t.Fatalf("outcome=%+v requests=%+v", outcome, prompter.requests)
	}
	saved, err := os.ReadFile(filepath.Join(dir, "notes.txt"))
	if err != nil || string(saved) != prepared.Execution.Stdout || prompter.requests[0].Stdout != prepared.Execution.Stdout || len(prompter.requests[0].Changes) != 1 {
		t.Fatalf("recovery reran the command or lost its candidate: %s %v", saved, err)
	}
}
