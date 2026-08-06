package workflow

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

func BuildContextBriefing(pack contextengine.ContextPack, state *runtimeState) string {
	if state == nil {
		return ""
	}
	sections := []string{
		"Objective: " + strings.TrimSpace(pack.WorkSpec.Instruction),
		fmt.Sprintf("Mode: strategy=%s execute_mode=%s phase=%s", state.strategy, state.executeMode, state.phase),
		"Authority: use only disclosed tools and current target scope; ordinary assistant text never completes the run.",
	}
	if state.strategy == StrategyPlan {
		sections = append(sections, "Authority detail: this is read-only planning; do not call update_plan or write tools.")
	}
	if state.strategy == StrategyExecute {
		sections = append(sections, "Authority detail: writes are allowed only through the active run session and only inside target scope.")
	}
	if state.requirements != nil {
		sections = append(sections, "Requirement ledger:\n"+state.requirements.Brief())
	}
	sections = append(sections, "Working set:\n"+workingSetSummary(state))
	if focus := nextFocus(state); focus != "" {
		sections = append(sections, "Next focus: "+focus)
	}
	return strings.Join(sections, "\n")
}

func workingSetSummary(state *runtimeState) string {
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

func nextFocus(state *runtimeState) string {
	if state.strategy == StrategyExecute && state.executeMode == ExecuteModePlanned {
		if state.plan == nil {
			return "create a lightweight execution plan with update_plan before writing."
		}
		if state.plan.HasBlockingSteps() {
			return "complete the next pending plan step and keep the plan statuses current."
		}
	}
	if state.strategy == StrategyExecute && len(state.changeSet().All()) > 0 {
		return "ensure latest changed targets have fresh required evidence, then finish with complete message."
	}
	if state.strategy == StrategyPlan {
		return "deliver the full plan in finish(message), not in ordinary assistant text."
	}
	return "use the next disclosed tool or finish(message) when complete."
}
