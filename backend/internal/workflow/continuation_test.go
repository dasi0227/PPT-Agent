package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestManualContinuationRenewsBudgetAndKeepsCumulativeUsage(t *testing.T) {
	budget := runtimeBudgetWithDuration(time.Hour)
	budget.MaxTurns = 2
	priorDuration := (90 * time.Minute).Milliseconds()
	agent := &scriptedAgent{responses: []AgentResponse{finishCall("分析完成")}}
	checkpoints := &checkpointRecorder{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "continuation", ProjectDir: t.TempDir(),
		Context:     testPack(model.ModeChat, model.ScopeCurrentPage, false, "分析当前页"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec}, Budget: budget, Checkpoint: checkpoints,
		ResumeCheckpoint: &RuntimeCheckpoint{
			RunID: "continuation", LoopID: "same-loop", Mode: model.ModeChat, Phase: PhaseChat, ResumePhase: PhaseChat,
			Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_1"),
			Turns: 10, BudgetBaseTurns: 10, ActiveDurationMS: priorDuration, BudgetBaseDurationMS: priorDuration,
		},
	})
	if outcome.Status != StatusCompleted || len(agent.requests) != 1 {
		t.Fatalf("continuation immediately exhausted: %+v, requests=%d", outcome, len(agent.requests))
	}
	last := checkpoints.checkpoints[len(checkpoints.checkpoints)-1]
	if last.Turns != 11 || last.ActiveDurationMS < priorDuration || last.LoopID != "same-loop" {
		t.Fatalf("cumulative state lost: %+v", last)
	}
}

func TestTerminalCheckpointPreservesExecutablePhaseAndEligibility(t *testing.T) {
	for _, tc := range []struct {
		code    string
		allowed bool
	}{
		{CodeBudgetExceeded, true}, {CodeConsecutiveErrors, true}, {CodeGateRejectedRepeated, true},
		{CodeCanceled, true}, {"INTERNAL", false}, {CodeAgentFailed, false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			sink := &checkpointRecorder{}
			state := &RunState{ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}, runID: "run", mode: model.ModeExecute, phase: PhaseCompletionCheck,
				scope: model.NewRunScope(model.ScopeAllPages), pack: testPack(model.ModeExecute, model.ScopeAllPages, false, "继续"),
			}
			NewRuntime(nil).failAgentError(RuntimeInput{Checkpoint: sink}, state, model.NewAgentError(tc.code, "test", errors.New("test")))
			cp := sink.checkpoints[len(sink.checkpoints)-1]
			if cp.ContinuationAllowed != tc.allowed || cp.ResumePhase != PhaseExecuting || cp.Phase == PhaseTerminal {
				t.Fatalf("wrong terminal recovery point: %+v", cp)
			}
		})
	}
}

func TestContinuationDoesNotReplayCanceledCommandResult(t *testing.T) {
	transcript := &recordingTranscript{messages: []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "stopped-command", Name: "run_command", Args: map[string]any{"command": "pwd"}}}},
		{Role: llm.RoleTool, ToolCallID: "stopped-command", Content: llm.TextContent(`{"ok":false,"code":"CANCELED"}`)},
	}}
	agent := &scriptedAgent{err: context.Canceled}
	NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "continued-command", ProjectDir: testProject(t, ArtifactSlideSpec), Transcript: transcript,
		Context: testPack(model.ModeExecute, model.ScopeCurrentPage, false, "继续任务"),
		ResumeCheckpoint: &RuntimeCheckpoint{
			RunID: "continued-command", Mode: model.ModeExecute, Phase: PhaseWaitingInput, ResumePhase: PhaseExecuting,
			Scope:          model.NewRunScope(model.ScopeCurrentPage, "sli_1"),
			PendingCommand: &PendingCommandApproval{CallID: "stopped-command", Command: "pwd"},
		},
	})
	if len(agent.requests) != 1 {
		t.Fatalf("canceled command was replayed before reaching the model: requests=%d", len(agent.requests))
	}
}
