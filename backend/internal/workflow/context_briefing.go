package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

func BuildContextBriefing(pack contextengine.ContextPack, state *RunState) string {
	if state == nil {
		return ""
	}
	if len(state.retrievedContext) == 0 {
		return ""
	}
	changed := map[string]bool{}
	for _, change := range state.changeSet().All() {
		changed[change.Artifact.Resource().SlideID] = true
	}
	additional := []RetrievedContextItem{}
	for _, item := range state.retrievedContext {
		// Segment selection reasons describe bookkeeping, not retrieved content.
		if item.Source != "context_manifest" || item.Snippet == "" || changed[item.Target.SlideID] {
			continue
		}
		if pack.Target.SlideHTMLSummary != nil && len(pack.Target.SlideIDs) == 1 && item.Target.SlideID == pack.Target.SlideIDs[0] {
			continue
		}
		additional = append(additional, item)
	}
	return retrievedContextBrief(additional)
}

func (r *Runtime) retrieveTurnContext(ctx context.Context, input RuntimeInput, state *RunState) error {
	if state == nil {
		return nil
	}
	queryText := retrievalQueryText(state.pack, state)
	retrievalKey := hashBytes([]byte(state.contextIndex.ID + "\x00" + string(state.phase) + "\x00" + queryText))
	if retrievalKey == state.lastRetrievalKey {
		state.contextBriefing = BuildContextBriefing(state.pack, state)
		return nil
	}
	retriever := HybridContextRetriever{
		Index: state.contextIndex, Embedder: r.Embedder, Scope: state.scope,
	}
	result, err := retriever.Retrieve(ctx, RetrievalQuery{
		RunID: state.runID, Command: state.pack.Command, RequirementLedger: state.requirements,
		LatestIssues: state.issues, Phase: state.phase,
		QueryText: queryText, Limit: 5, DetailBudget: 1200,
	})
	if err != nil {
		return err
	}
	state.retrievedContext = result.Results
	state.lastRetrievalKey = retrievalKey
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
		line := fmt.Sprintf("- %s", target)
		if item.Snippet != "" {
			line += "\n  summary: " + item.Snippet
		}
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
