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
		RunID: "continuation", ProjectDir: resumedProject(t, t.TempDir(), "continuation"),
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
		{CodeReadLoop, true},
		{CodeCanceled, true}, {"PROVIDER_UNAVAILABLE", true}, {"PROVIDER_BAD_REQUEST", false},
		{"IMAGE_REFERENCE_UNAVAILABLE", false}, {"INTERNAL", false}, {CodeAgentFailed, false},
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
		RunID: "continued-command", ProjectDir: resumedProject(t, testProject(t, ArtifactSlideSpec), "continued-command"), Transcript: transcript,
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

type fallbackBudgetAgent struct {
	deadlines     []time.Time
	waitForExpiry bool
}

func (agent *fallbackBudgetAgent) Next(ctx context.Context, _ AgentRequest) (AgentResponse, error) {
	deadline, _ := ctx.Deadline()
	agent.deadlines = append(agent.deadlines, deadline)
	if len(agent.deadlines) == 1 {
		if agent.waitForExpiry {
			<-ctx.Done()
		}
		return AgentResponse{}, llm.ErrFallbackActivated
	}
	return finishCall("done"), nil
}

func TestProviderFallbackRebuildKeepsOriginalRequestDeadline(t *testing.T) {
	agent := &fallbackBudgetAgent{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "fallback-budget", ProjectDir: t.TempDir(),
		Context:     testPack(model.ModeChat, model.ScopeCurrentPage, false, "分析"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted || len(agent.deadlines) != 2 || agent.deadlines[0].IsZero() || !agent.deadlines[0].Equal(agent.deadlines[1]) {
		t.Fatalf("fallback reset request budget: outcome=%+v deadlines=%v", outcome, agent.deadlines)
	}
}

func TestExpiredProviderBudgetStopsBeforeFallbackPreparation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	agent := &fallbackBudgetAgent{waitForExpiry: true}
	checkpoints := &checkpointRecorder{}
	outcome := NewRuntime(agent).Run(ctx, RuntimeInput{
		RunID: "fallback-expired", ProjectDir: t.TempDir(), Checkpoint: checkpoints,
		Context:     testPack(model.ModeChat, model.ScopeCurrentPage, false, "分析"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	last := checkpoints.checkpoints[len(checkpoints.checkpoints)-1]
	if outcome.Code != "PROVIDER_UNAVAILABLE" || len(agent.deadlines) != 1 || !last.ContinuationAllowed {
		t.Fatalf("expired budget sent fallback or lost recovery: outcome=%+v calls=%d checkpoint=%+v", outcome, len(agent.deadlines), last)
	}
}

func TestProviderOutageContinuationKeepsApprovedPlanAndSavedToolResult(t *testing.T) {
	const runID = "provider-continue"
	dir := resumedProject(t, testProject(t, ArtifactSlideSpec), runID)
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "修改当前页")
	plan := &Plan{ID: "approved", ApprovalID: "approval", Status: PlanActive, Title: "修改", Content: "已批准的正文"}
	plan.ApprovedContentHash = plan.ContentHash()
	transcript := &recordingTranscript{}
	pack.Manifest.ThreadID = "thread"
	checkpoints, events := &checkpointRecorder{}, &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{toolCall("saved-write", "edit_spec", map[string]any{"content": "saved content"})}, err: llm.ErrUnavailable}
	input := RuntimeInput{
		RunID: runID, ProjectDir: dir, Context: pack, Transcript: transcript, Checkpoint: checkpoints, Emitter: events,
		DomainTools: fakeProvider{kind: ArtifactSlideSpec},
		ResumeCheckpoint: &RuntimeCheckpoint{RunID: runID, LoopID: "original-loop", Mode: model.ModeExecute,
			Phase: PhaseExecuting, ResumePhase: PhaseExecuting, Scope: pack.Command.Scope, Plan: plan},
	}
	outcome := NewRuntime(agent).Run(context.Background(), input)
	checkpoint := checkpoints.checkpoints[len(checkpoints.checkpoints)-1]
	if outcome.Code != "PROVIDER_UNAVAILABLE" || !checkpoint.ContinuationAllowed || events.count(model.EventToolCompleted) != 1 {
		t.Fatalf("write/outage did not produce a recoverable checkpoint: outcome=%+v checkpoint=%+v", outcome, checkpoint)
	}
	resumed := &scriptedAgent{err: llm.ErrUnavailable}
	input.ResumeCheckpoint = &checkpoint
	NewRuntime(resumed).Run(context.Background(), input)
	if len(resumed.requests) != 1 || resumed.requests[0].Plan == nil || resumed.requests[0].Plan.ApprovedContentHash != plan.ApprovedContentHash ||
		resumed.requests[0].LoopID != "original-loop" || events.count(model.EventToolCompleted) != 1 {
		t.Fatalf("continuation lost approval or replayed the saved write: requests=%+v events=%+v", resumed.requests, events.events)
	}
	found := false
	for _, message := range resumed.requests[0].Messages {
		if message.Role == llm.RoleTool && message.ToolCallID == "saved-write" {
			found = true
		}
	}
	if !found {
		t.Fatal("continuation lost the saved tool observation")
	}
}
