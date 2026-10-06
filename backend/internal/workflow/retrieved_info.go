package workflow

import (
	"context"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

// RetrievedInfo is reference text, not a task, permission or generated summary.
type RetrievedInfo struct {
	Target  string `json:"target"`
	Source  string `json:"source"`
	Content string `json:"content"`
}

func buildRetrievedInfo(state *RunState) []RetrievedInfo {
	out := []RetrievedInfo{}
	if state == nil {
		return out
	}
	for _, item := range state.retrievedContext {
		if item.Source != "slide_html_summary" || item.Target.SlideID == "" || item.Snippet == "" {
			continue
		}
		current, exists := state.pack.SlideHTML.Summaries[item.Target.SlideID]
		if !exists || current.SourceHash != item.Hash {
			continue
		}
		out = append(out, RetrievedInfo{item.Target.SlideID, "html", item.Snippet})
	}
	return out
}

func (r *Runtime) retrieveTurnContext(ctx context.Context, input RuntimeInput, state *RunState) error {
	if state == nil {
		return nil
	}
	queryText := retrievalQueryText(state.pack, state)
	retrievalKey := hashBytes([]byte(state.contextIndex.ID + "\x00" + string(state.phase) + "\x00" + queryText))
	if retrievalKey == state.lastRetrievalKey {
		state.retrievedInfo = buildRetrievedInfo(state)
		return nil
	}
	retriever := KeywordContextRetriever{
		Index: state.contextIndex, Scope: state.scope,
	}
	result, err := retriever.Retrieve(ctx, RetrievalQuery{
		RunID: state.runID, Command: state.pack.Command,
		LatestIssues: state.issues, Phase: state.phase,
		QueryText: queryText, Limit: 5, TokenBudget: 1200,
	})
	if err != nil {
		return err
	}
	state.retrievedContext = result.Results
	state.lastRetrievalKey = retrievalKey
	state.contextIndexRef = result.IndexRef
	state.retrievedInfo = buildRetrievedInfo(state)
	recordTrace(input.Trace, state.runID, "context.retrieved", map[string]any{
		"loop_id": state.loopID, "query": result.Query, "count": len(result.Results),
		"estimated_tokens": result.EstimatedTokens, "index_ref": result.IndexRef,
	})
	return nil
}

func retrievalQueryText(pack contextengine.ContextPack, state *RunState) string {
	parts := []string{pack.Command.Instruction}
	for _, instruction := range state.reviewInstructions {
		if instruction.Text != pack.Command.Instruction {
			parts = append(parts, instruction.Text)
		}
	}
	if state.plan != nil {
		parts = append(parts, state.plan.Title, state.plan.Content)
	}
	for _, issue := range state.issues {
		parts = append(parts, issue.Summary)
	}
	return strings.Join(parts, " ")
}
