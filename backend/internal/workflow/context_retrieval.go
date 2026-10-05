package workflow

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ContextIndex struct {
	ID        string             `json:"id,omitempty"`
	RunID     string             `json:"run_id"`
	ThreadID  string             `json:"thread_id"`
	ProjectID string             `json:"project_id"`
	PackHash  string             `json:"pack_hash,omitempty"`
	BuiltAt   int64              `json:"built_at"`
	Items     []ContextIndexItem `json:"items"`
}

type ContextIndexItem struct {
	RefID     string   `json:"ref_id"`
	Kind      string   `json:"kind"`
	Source    string   `json:"source"`
	Target    Resource `json:"target"`
	Hash      string   `json:"hash"`
	Summary   string   `json:"summary"`
	TokenCost int      `json:"token_cost"`
	Freshness string   `json:"freshness"`
	UpdatedAt int64    `json:"updated_at"`
}

// ContextIndexContentHash excludes snapshot identity and timestamps so retries
// of the same logical index resolve to one durable snapshot.
func ContextIndexContentHash(index ContextIndex) string {
	index.ID = ""
	index.BuiltAt = 0
	index.Items = append([]ContextIndexItem(nil), index.Items...)
	for i := range index.Items {
		index.Items[i].UpdatedAt = 0
	}
	raw, _ := json.Marshal(index)
	return hashBytes(raw)
}

func ContextIndexSnapshotID(index ContextIndex) string {
	return "ctxidx_" + hashBytes([]byte(index.RunID + "\x00" + index.PackHash + "\x00" + ContextIndexContentHash(index)))[:24]
}

type RetrievalQuery struct {
	RunID        string
	Command      model.RunCommand
	LatestIssues []Issue
	Phase        RunPhase
	QueryText    string
	Kinds        []string
	Limit        int
	TokenBudget  int
}

type RetrievalResult struct {
	Query           string                 `json:"query"`
	Results         []RetrievedContextItem `json:"results"`
	EstimatedTokens int                    `json:"estimated_tokens"`
	RemainingBudget int                    `json:"remaining_budget"`
	IndexRef        string                 `json:"index_ref,omitempty"`
}

type RetrievedContextItem struct {
	RefID           string   `json:"ref_id"`
	Kind            string   `json:"kind"`
	Source          string   `json:"source"`
	Target          Resource `json:"target,omitempty"`
	Hash            string   `json:"hash"`
	Score           float64  `json:"score"`
	SelectionReason string   `json:"selection_reason"`
	Snippet         string   `json:"snippet"`
	Freshness       string   `json:"freshness"`
	EstimatedTokens int      `json:"estimated_tokens"`
}

type KeywordContextRetriever struct {
	Index ContextIndex
	Scope model.RunScope
}

func NewContextIndexFromPack(pack contextengine.ContextPack) ContextIndex {
	index := ContextIndex{
		RunID: pack.Manifest.RunID, ThreadID: pack.Manifest.ThreadID, ProjectID: pack.Manifest.ProjectID,
		PackHash: pack.Manifest.PackHash, BuiltAt: time.Now().UnixNano(), Items: []ContextIndexItem{},
	}
	appendItem := func(item ContextIndexItem) {
		if strings.TrimSpace(item.Summary) == "" {
			return
		}
		if item.RefID == "" {
			item.RefID = "ref_" + hashBytes([]byte(item.Kind + "\x00" + item.Source + "\x00" + item.Summary))[:24]
		}
		if item.Hash == "" {
			item.Hash = hashBytes([]byte(item.Summary))
		}
		if item.UpdatedAt == 0 {
			item.UpdatedAt = index.BuiltAt
		}
		if item.Freshness == "" {
			item.Freshness = "current"
		}
		if item.TokenCost <= 0 {
			item.TokenCost = approximateTokens(item.Summary)
		}
		index.Items = append(index.Items, item)
	}
	ids := make([]string, 0, len(pack.SlideHTML.Summaries))
	for id := range pack.SlideHTML.Summaries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		summary := pack.SlideHTML.Summaries[id]
		appendItem(ContextIndexItem{
			Kind: "slide_html", Source: "slide_html_summary",
			Target: Resource{Type: "slide", SlideID: id, Part: "html"},
			Hash:   summary.SourceHash, Summary: strings.Join(summary.TextDigest, " "),
		})
	}
	index.ID = ContextIndexSnapshotID(index)
	return index
}

func (r KeywordContextRetriever) Retrieve(ctx context.Context, query RetrievalQuery) (RetrievalResult, error) {
	if err := ctx.Err(); err != nil {
		return RetrievalResult{}, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	budget := query.TokenBudget
	if budget <= 0 {
		budget = 1200
	}
	queryText := strings.TrimSpace(query.QueryText)
	if queryText == "" {
		queryText = query.Command.Instruction
	}
	scope := r.Scope
	if scope.Source.Kind == "" {
		scope = query.Command.Scope
	}
	allowedKinds := map[string]bool{}
	for _, kind := range query.Kinds {
		allowedKinds[kind] = true
	}
	candidates := make([]RetrievedContextItem, 0, len(r.Index.Items))
	for _, item := range r.Index.Items {
		if len(allowedKinds) > 0 && !allowedKinds[item.Kind] {
			continue
		}
		if !retrievalScopeAllows(scope, item.Target) {
			continue
		}
		if item.Freshness == "stale" || strings.TrimSpace(item.Summary) == "" {
			continue
		}
		keyword := normalizedKeywordScore(queryText, item.Summary)
		scopeBoost := 0.0
		if item.Target.Key() == resourceForRunScope(scope).Key() {
			scopeBoost = 1
		}
		freshness := 0.5
		if item.Freshness == "current" {
			freshness = 1
		}
		issueBoost := issueRelevance(query.LatestIssues, item)
		if keyword == 0 && scopeBoost == 0 && issueBoost == 0 {
			continue
		}
		score := keyword*0.25 + scopeBoost*0.15 + freshness*0.10 + issueBoost*0.05
		cost := item.TokenCost
		if cost == 0 {
			cost = approximateTokens(item.Summary)
		}
		candidates = append(candidates, RetrievedContextItem{
			RefID: item.RefID, Kind: item.Kind, Source: item.Source, Target: item.Target,
			Hash: item.Hash, Score: math.Round(score*10000) / 10000,
			SelectionReason: selectionReason(item, keyword, scopeBoost, issueBoost),
			Snippet:         compactSnippet(item.Summary, 1200), Freshness: item.Freshness, EstimatedTokens: cost,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].RefID < candidates[j].RefID
		}
		return candidates[i].Score > candidates[j].Score
	})
	out := RetrievalResult{Query: queryText, Results: []RetrievedContextItem{}, RemainingBudget: budget, IndexRef: r.Index.ID}
	for _, item := range candidates {
		if len(out.Results) >= limit {
			break
		}
		if out.EstimatedTokens+item.EstimatedTokens > budget {
			continue
		}
		out.EstimatedTokens += item.EstimatedTokens
		out.Results = append(out.Results, item)
	}
	out.RemainingBudget = budget - out.EstimatedTokens
	return out, nil
}

func retrievalScopeAllows(scope model.RunScope, target Resource) bool {
	if target.Type == "" {
		return true
	}
	return AllowsRead(scope, target)
}

func resourceForRunScope(target model.RunScope) Resource {
	if target.IsSinglePage() {
		return Resource{Type: "slide", SlideID: target.SlideIDs[0], Part: "html"}
	}
	return Resource{Type: "deck", Part: "design"}
}

func normalizedKeywordScore(query, text string) float64 {
	raw := relevanceScore(query, text)
	if raw <= 0 {
		return 0
	}
	if raw >= 100 {
		return 1
	}
	return math.Min(float64(raw)/50.0, 1)
}

func issueRelevance(issues []Issue, item ContextIndexItem) float64 {
	for _, issue := range issues {
		if issue.Resource.Key() == item.Target.Key() {
			return 1
		}
	}
	return 0
}

func selectionReason(item ContextIndexItem, keyword, scopeBoost, issueBoost float64) string {
	reasons := []string{}
	if keyword > 0 {
		reasons = append(reasons, "keyword match for query")
	}
	if scopeBoost > 0 {
		reasons = append(reasons, "matches current RunScope")
	}
	if issueBoost > 0 {
		reasons = append(reasons, "matches latest runtime issue")
	}
	if item.Freshness == "current" {
		reasons = append(reasons, "current content")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "authorized context candidate")
	}
	return strings.Join(reasons, "; ")
}

func relevanceScore(query, text string) int {
	query, text = strings.ToLower(query), strings.ToLower(text)
	if strings.TrimSpace(query) == "" {
		return 0
	}
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
