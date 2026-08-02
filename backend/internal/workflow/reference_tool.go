package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

type referenceSearchTool struct{ pack contextengine.ContextPack }

func (referenceSearchTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "search_refs", Description: "Search only the current run's authorized context manifest, references, memory, history, and project material.",
		Parameters: objectSchema([]string{"query"}, map[string]any{
			"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 1000},
			"kinds": map[string]any{
				"type": "array", "uniqueItems": true,
				"items": map[string]any{"type": "string", "enum": []string{"reference", "memory", "history", "project"}},
			},
			"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 20},
		}),
	}
}

type refCandidate struct {
	kind, title, source, snippet, refID, hash string
	revision                                  int
	detail                                    bool
	score                                     int
}

func (t referenceSearchTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	query := strings.TrimSpace(stringValue(input.Args["query"]))
	if query == "" {
		return failedToolResult(CodeModelInvalid, "query is required", false)
	}
	manifest := t.pack.Manifest
	if manifest.RunID == "" || manifest.ProjectID != t.pack.Project.ID ||
		(input.RunID != "" && manifest.RunID != input.RunID) {
		return failedToolResult(ErrCapabilityDenied.Error(), "context manifest is not authorized for this run", false)
	}
	limit := intValue(input.Args["limit"], 5)
	if limit < 1 {
		limit = 1
	}
	if limit > 20 {
		limit = 20
	}
	kinds, err := searchKinds(input.Args["kinds"])
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	candidates := t.candidates(query, kinds)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].refID < candidates[j].refID
		}
		return candidates[i].score > candidates[j].score
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	results := make([]map[string]any, 0, len(candidates))
	estimated := 0
	for _, item := range candidates {
		estimated += len([]rune(item.snippet))/4 + 30
		results = append(results, map[string]any{
			"ref_id": item.refID, "kind": item.kind, "title": item.title,
			"source": item.source, "snippet": item.snippet, "revision": item.revision,
			"hash": item.hash, "detail_available": item.detail,
		})
	}
	remaining := manifest.BudgetTokens - manifest.EstimatedTokens
	if remaining < 256 {
		remaining = 256
	}
	if estimated > remaining {
		return failedToolResult(CodeContextBudget, "search results exceed the remaining context budget; narrow query or lower limit", true)
	}
	result := SuccessfulToolResult(fmt.Sprintf("found %d authorized references", len(results)))
	result.Data = map[string]any{
		"query": query, "results": results, "estimated_tokens": estimated,
		"remaining_budget": remaining,
	}
	return result
}

func searchKinds(value any) (map[string]bool, error) {
	out := map[string]bool{"reference": true, "memory": true, "history": true, "project": true}
	if value == nil {
		return out, nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("kinds must be an array")
	}
	out = map[string]bool{}
	for _, raw := range values {
		kind := stringValue(raw)
		switch kind {
		case "reference", "memory", "history", "project":
			out[kind] = true
		default:
			return nil, fmt.Errorf("unsupported reference kind %q", kind)
		}
	}
	return out, nil
}

func (t referenceSearchTool) candidates(query string, kinds map[string]bool) []refCandidate {
	out := []refCandidate{}
	if kinds["reference"] {
		for _, ref := range t.pack.Manifest.Refs {
			out = appendCandidate(out, refCandidate{
				kind: "reference", title: string(ref.Kind), source: "context_manifest",
				snippet: compactSnippet(ref.Summary, 1200), refID: ref.ID,
				hash: ref.ContentHash, revision: ref.Revision, detail: len(ref.AvailableLevels) > 1,
			}, query)
		}
	}
	if kinds["memory"] {
		appendMemory := func(title string, items []contextengine.MemoryItem) {
			for _, item := range items {
				hash := hashBytes([]byte(item.Key + "\x00" + item.Value))
				out = appendCandidate(out, refCandidate{
					kind: "memory", title: title, source: "thread_memory",
					snippet: compactSnippet(item.Value, 1200), refID: "ref_" + hash[:24],
					hash: hash, revision: t.pack.Memory.Revision, detail: false,
				}, query)
			}
		}
		appendMemory("User preference", t.pack.Memory.UserPreferences)
		appendMemory("Confirmed decision", t.pack.Memory.ConfirmedDecisions)
		appendMemory("Brand constraint", t.pack.Memory.BrandConstraints)
		appendMemory("Content fact", t.pack.Memory.ContentFacts)
		appendMemory("Open question", t.pack.Memory.OpenQuestions)
		appendMemory("Recent change", t.pack.Memory.RecentChanges)
	}
	if kinds["history"] {
		for _, turn := range t.pack.RecentTurns {
			hash := hashBytes([]byte(turn.RunID + "\x00" + turn.Text))
			out = appendCandidate(out, refCandidate{
				kind: "history", title: turn.Type, source: "current_thread_history",
				snippet: compactSnippet(turn.Text, 1200), refID: "ref_" + hash[:24],
				hash: hash, revision: t.pack.Revisions.ThreadMemory, detail: false,
			}, query)
		}
	}
	if kinds["project"] {
		for _, segment := range t.pack.Manifest.Segments {
			if segment.Kind == contextengine.SegmentPolicy || segment.Kind == contextengine.SegmentWorkSpec {
				continue
			}
			out = appendCandidate(out, refCandidate{
				kind: "project", title: string(segment.Kind), source: segment.SourceRef,
				snippet: segment.SelectionReason, refID: "ref_" + segment.ContentHash[:minInt(24, len(segment.ContentHash))],
				hash: segment.ContentHash, revision: segment.Revision, detail: segment.DetailLevel != contextengine.DetailFull,
			}, query)
		}
	}
	return out
}

func appendCandidate(values []refCandidate, candidate refCandidate, query string) []refCandidate {
	candidate.score = relevanceScore(query, candidate.title+" "+candidate.snippet+" "+candidate.source)
	if candidate.score == 0 {
		return values
	}
	return append(values, candidate)
}

func relevanceScore(query, text string) int {
	query, text = strings.ToLower(query), strings.ToLower(text)
	if strings.Contains(text, query) {
		return 100 + len([]rune(query))
	}
	score := 0
	for _, term := range strings.Fields(query) {
		if len([]rune(term)) > 1 && strings.Contains(text, term) {
			score += 10
		}
	}
	return score
}

func compactSnippet(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	return string([]rune(value)[:limit]) + "…"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
