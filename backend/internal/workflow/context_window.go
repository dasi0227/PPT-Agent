package workflow

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// EstimateContextWindow uses the same request projection as CognitiveAgent.Next.
func EstimateContextWindow(request AgentRequest, max int) contextengine.WindowSnapshot {
	request = prepareAgentRequest(request)
	snapshot := (contextengine.PromptEstimator{}).Estimate(contextengine.PromptEstimateInput{
		System: runtimeSystemPromptForRequest(request), Messages: requestContextMessages(request), Tools: request.Tools, Max: max,
	})
	snapshot.CompactableTokens = contextcompact.CompactableTokens(request.Messages)
	snapshot.CompactThresholdTokens = contextcompact.MinimumCompactableTokens
	return snapshot
}

func windowPhase(mode model.RunMode, phase RunPhase) RunPhase {
	switch phase {
	case PhaseChat, PhasePlanning, PhaseExecuting, PhaseWaitingInput:
		return phase
	}
	switch mode {
	case model.ModeCreate:
		return PhaseExecuting
	case model.ModePlan:
		return PhasePlanning
	default:
		return PhaseChat
	}
}

func windowTools(state *RunState) []ToolSchema {
	phase := windowPhase(state.mode, state.phase)
	var schemas []ToolSchema
	if state.tools != nil {
		schemas = state.tools.Disclose(phase, state.mode, state.scope)
	}
	schemas = append(schemas, controlSchemas(phase, state.mode, state.plan)...)
	sort.Slice(schemas, func(i, j int) bool { return schemas[i].Name < schemas[j].Name })
	return schemas
}

// RebuildContextWindow is read-only: no model calls, tool execution, checkpoints
// or transcript writes. It rebuilds complete accounting after restart/compaction.
func RebuildContextWindow(ctx context.Context, input RuntimeInput, messages []llm.Message, max int) (contextengine.WindowSnapshot, error) {
	state := &RunState{
		runID: input.RunID, projectDir: input.ProjectDir, pack: input.Context,
		mode: input.Context.Command.Mode, scope: input.Context.Command.Scope,
		messages: messages, ledger: NewEvidenceLedger(), committedChanges: EmptyChangeSet(),
		activeSkills: &ActiveSkillSet{Skills: input.Context.Command.Skills, Components: input.Context.Command.Components},
	}
	if cp := input.ResumeCheckpoint; cp != nil {
		state.phase, state.plan = cp.Phase, cp.Plan
		state.readImages = append([]RunReadImage(nil), cp.ReadImages...)
		state.activeSkills = &ActiveSkillSet{Skills: cp.ActiveSkills, Components: cp.ActiveComponents}
		state.committedChanges = cp.Changes
	}
	state.phase = windowPhase(state.mode, state.phase)
	if input.RunID == "" || input.ResumeCheckpoint == nil {
		baseline, err := readProjectSources(func(name string) ([]byte, error) {
			return os.ReadFile(filepath.Join(input.ProjectDir, name))
		})
		if err != nil {
			return contextengine.WindowSnapshot{}, err
		}
		state.projectBaseline = baseline
	}
	if err := state.refreshProjectState(); err != nil {
		return contextengine.WindowSnapshot{}, err
	}
	registry, err := buildDomainToolRegistry(input)
	if err != nil {
		return contextengine.WindowSnapshot{}, err
	}
	state.tools = registry
	if cp := input.ResumeCheckpoint; cp != nil && cp.ToolDecision != nil {
		state.tools.Limit(cp.ToolDecision.AllowedTools)
	}
	request := agentRequestForState(input, state, windowTools(state))
	// A compacted original request is represented by the summary. Do not append
	// the persisted Run instruction again during a read-only reconstruction.
	request.InstructionInMessages = true
	return EstimateContextWindow(request, max), nil
}

func (r *Runtime) measureTerminalContextWindow(input RuntimeInput, state *RunState) {
	if err := state.refreshProjectState(); err != nil {
		// Still account for the final messages if a failed Run has broken sources.
		recordTrace(input.Trace, state.runID, "context.window_refresh_failed", map[string]any{"error": err.Error()})
	}
	r.measureContextWindow(input, state, windowTools(state))
}
