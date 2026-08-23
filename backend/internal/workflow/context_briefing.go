package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func BuildContextBriefing(_ contextengine.ContextPack, state *RunState) string {
	if state == nil {
		return ""
	}
	sections := []string{}
	if len(state.retrievedContext) > 0 {
		sections = append(sections, "Retrieved context:\n"+retrievedContextBrief(state.retrievedContext))
	}
	sections = append(sections, "Working set:\n"+workingSetSummary(state))
	if focus := nextFocus(state, state.mode); focus != "" {
		sections = append(sections, "Next focus: "+focus)
	}
	return strings.Join(sections, "\n")
}

func (r *Runtime) retrieveTurnContext(ctx context.Context, input RuntimeInput, state *RunState) error {
	if state == nil {
		return nil
	}
	retriever := HybridContextRetriever{
		Index: state.contextIndex, Embedder: r.Embedder, Scope: state.scope,
	}
	result, err := retriever.Retrieve(ctx, RetrievalQuery{
		RunID: state.runID, Command: state.pack.Command, RequirementLedger: state.requirements,
		LatestIssues: state.issues, Phase: state.phase,
		QueryText: retrievalQueryText(state.pack, state), Limit: 5, DetailBudget: 1200,
	})
	if err != nil {
		return err
	}
	state.retrievedContext = result.Results
	state.contextIndexRef = result.IndexRef
	state.contextBriefing = BuildContextBriefing(state.pack, state)
	recordTrace(input.Trace, state.runID, "context.retrieved", map[string]any{
		"loop_id": state.loopID, "query": result.Query, "count": len(result.Results),
		"estimated_tokens": result.EstimatedTokens, "index_ref": result.IndexRef,
	})
	return nil
}

func retrievalQueryText(pack contextengine.ContextPack, state *RunState) string {
	parts := []string{pack.Command.Instruction}
	if state.plan != nil {
		parts = append(parts, state.plan.Brief())
	}
	for _, issue := range state.issues {
		parts = append(parts, issue.Code, issue.Summary)
	}
	if state.requirements != nil {
		for _, item := range state.requirements.BlockingItems() {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, " ")
}

func retrievedContextBrief(items []RetrievedContextItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		target := item.Target.Key()
		if item.Target.Type == "" {
			target = "global"
		}
		lines = append(lines, fmt.Sprintf(
			"- %s kind=%s target=%s rev=%d hash=%s score=%.2f reason=%s",
			item.RefID, item.Kind, target, item.Revision, shortHash(item.Hash), item.Score, item.SelectionReason,
		))
	}
	return strings.Join(lines, "\n")
}

func shortHash(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func workingSetSummary(state *RunState) string {
	lines := []string{}
	if state.plan != nil {
		lines = append(lines, "- Plan: "+state.plan.Brief())
	} else {
		lines = append(lines, "- Plan: none")
	}
	changes := state.changeSet().All()
	if len(changes) == 0 {
		lines = append(lines, "- Changes: none")
	} else {
		values := make([]string, 0, len(changes))
		for _, change := range changes {
			target := resourceForArtifact(change.Artifact)
			values = append(values, fmt.Sprintf("%s via %s", target.Key(), change.Source))
		}
		lines = append(lines, "- Changes: "+strings.Join(values, "; "))
	}
	evidence := state.ledger.Entries(state.changeSet())
	if len(evidence) == 0 {
		lines = append(lines, "- Evidence: none")
	} else {
		start := len(evidence) - 4
		if start < 0 {
			start = 0
		}
		values := make([]string, 0, len(evidence)-start)
		for _, entry := range evidence[start:] {
			fresh := "stale"
			if entry.Fresh {
				fresh = "fresh"
			}
			values = append(values, fmt.Sprintf("%s:%s:%s", entry.Kind, entry.Target.Key(), fresh))
		}
		lines = append(lines, "- Evidence: "+strings.Join(values, "; "))
	}
	if len(state.issues) == 0 {
		lines = append(lines, "- Issues: none")
	} else {
		latest := state.issues[len(state.issues)-1]
		lines = append(lines, "- Latest issue: "+latest.Code+" - "+latest.Summary)
	}
	return strings.Join(lines, "\n")
}

func nextFocus(state *RunState, mode model.RunMode) string {
	if mode == model.ModeExecute && state.plan != nil && state.plan.HasBlockingSteps() {
		return "complete the next pending plan step and keep the plan statuses current."
	}
	if mode == model.ModeExecute && len(state.changeSet().All()) > 0 {
		return "ensure latest changed targets have fresh required evidence, then finish with complete message."
	}
	if mode == model.ModePlan {
		if state.plan == nil {
			return "complete the full proposal with create_plan, then wait for explicit approval without calling finish."
		}
		return "revise the complete proposal with update_plan when feedback is present, then wait for approval."
	}
	return "use the next disclosed tool or finish(message) when complete."
}
