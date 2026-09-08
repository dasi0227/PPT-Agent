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

type DetailLevel string

const (
	DetailSummary   DetailLevel = "summary"
	DetailStructure DetailLevel = "structure"
	DetailFull      DetailLevel = "full"
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
	RefID           string              `json:"ref_id"`
	Kind            string              `json:"kind"`
	Source          string              `json:"source"`
	Target          Resource            `json:"target"`
	Revision        int                 `json:"revision"`
	Hash            string              `json:"hash"`
	Summary         string              `json:"summary"`
	Keywords        []string            `json:"keywords"`
	Embedding       []float32           `json:"embedding,omitempty"`
	TokenCost       map[DetailLevel]int `json:"token_cost"`
	AvailableLevels []DetailLevel       `json:"available_levels"`
	Scope           model.RunScope      `json:"scope"`
	Freshness       string              `json:"freshness"`
	UpdatedAt       int64               `json:"updated_at"`
}

type ContextIndexStore interface {
	SaveContextIndex(context.Context, ContextIndex) (string, error)
	GetContextIndex(context.Context, string) (ContextIndex, error)
	LatestContextIndex(context.Context, string) (ContextIndex, error)
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

type EmbeddingProvider interface {
	Embed(context.Context, []string) ([][]float32, error)
}

type NoopEmbeddingProvider struct{}

func (NoopEmbeddingProvider) Embed(_ context.Context, input []string) ([][]float32, error) {
	out := make([][]float32, len(input))
	for i := range out {
		out[i] = []float32{}
	}
	return out, nil
}

type HashEmbeddingProvider struct {
	Dimensions int
}

func (p HashEmbeddingProvider) Embed(_ context.Context, input []string) ([][]float32, error) {
	dim := p.Dimensions
	if dim <= 0 {
		dim = 16
	}
	out := make([][]float32, 0, len(input))
	for _, text := range input {
		vec := make([]float32, dim)
		for _, term := range strings.Fields(strings.ToLower(text)) {
			sum := hashBytes([]byte(term))
			for i := 0; i < dim; i++ {
				if sum[i%len(sum)]%2 == 0 {
					vec[i] += 1
				} else {
					vec[i] -= 1
				}
			}
		}
		normalizeVector(vec)
		out = append(out, vec)
	}
	return out, nil
}

type ContextRetriever interface {
	Retrieve(context.Context, RetrievalQuery) (RetrievalResult, error)
}

type RetrievalQuery struct {
	RunID             string
	Command           model.RunCommand
	RequirementLedger *RequirementLedger
	LatestIssues      []Issue
	Phase             RunPhase
	QueryText         string
	Kinds             []string
	Limit             int
	DetailBudget      int
}

type RetrievalResult struct {
	Query           string                 `json:"query"`
	Results         []RetrievedContextItem `json:"results"`
	EstimatedTokens int                    `json:"estimated_tokens"`
	RemainingBudget int                    `json:"remaining_budget"`
	IndexRef        string                 `json:"index_ref,omitempty"`
}

type RetrievedContextItem struct {
	RefID           string      `json:"ref_id"`
	Kind            string      `json:"kind"`
	Source          string      `json:"source"`
	Target          Resource    `json:"target,omitempty"`
	Revision        int         `json:"revision"`
	Hash            string      `json:"hash"`
	Score           float64     `json:"score"`
	SelectionReason string      `json:"selection_reason"`
	DetailAvailable bool        `json:"detail_available"`
	DetailLevel     DetailLevel `json:"detail_level"`
	Snippet         string      `json:"snippet"`
	Freshness       string      `json:"freshness"`
	EstimatedTokens int         `json:"estimated_tokens"`
}

type HybridContextRetriever struct {
	Index      ContextIndex
	Embedder   EmbeddingProvider
	Scope      model.RunScope
	PackBudget int
}

func NewContextIndexFromPack(pack contextengine.ContextPack, scope model.RunScope, embedder EmbeddingProvider) ContextIndex {
	index := ContextIndex{
		RunID: pack.Manifest.RunID, ThreadID: pack.Manifest.ThreadID, ProjectID: pack.Manifest.ProjectID,
		PackHash: pack.Manifest.PackHash, BuiltAt: time.Now().UnixNano(), Items: []ContextIndexItem{},
	}
	appendItem := func(item ContextIndexItem) {
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
		if len(item.AvailableLevels) == 0 {
			item.AvailableLevels = []DetailLevel{DetailSummary}
		}
		if item.TokenCost == nil {
			item.TokenCost = map[DetailLevel]int{DetailSummary: approximateTokens(item.Summary)}
		}
		item.Scope = scope
		item.Keywords = uniqueKeywords(item.Kind + " " + item.Source + " " + item.Summary)
		index.Items = append(index.Items, item)
	}
	for _, ref := range pack.Manifest.Refs {
		target := Resource{Type: "slide", SlideID: ref.TargetID, Part: "html"}
		levels := make([]DetailLevel, 0, len(ref.AvailableLevels))
		cost := map[DetailLevel]int{}
		for _, level := range ref.AvailableLevels {
			converted := DetailLevel(level)
			levels = append(levels, converted)
			cost[converted] = ref.EstimatedTokens[level]
		}
		appendItem(ContextIndexItem{
			RefID: ref.ID, Kind: string(ref.Kind), Source: "context_manifest",
			Target: target, Revision: ref.Revision, Hash: ref.ContentHash, Summary: ref.Summary,
			TokenCost: cost, AvailableLevels: levels,
		})
	}
	for _, segment := range pack.Manifest.Segments {
		if segment.Kind == contextengine.SegmentPolicy || segment.Kind == contextengine.SegmentRunCommand {
			continue
		}
		appendItem(ContextIndexItem{
			Kind: string(segment.Kind), Source: segment.SourceRef,
			Target: targetForSegment(segment), Revision: segment.Revision, Hash: segment.ContentHash,
			Summary: segment.SelectionReason, TokenCost: map[DetailLevel]int{DetailLevel(segment.DetailLevel): segment.EstimatedTokens},
			AvailableLevels: []DetailLevel{DetailLevel(segment.DetailLevel)},
		})
	}
	for _, item := range pack.Memory.UserPreferences {
		appendItem(memoryIndexItem("memory", "user_preference", pack, item))
	}
	for _, item := range pack.Memory.ConfirmedDecisions {
		appendItem(memoryIndexItem("memory", "confirmed_decision", pack, item))
	}
	if embedder != nil {
		texts := make([]string, len(index.Items))
		for i := range index.Items {
			texts[i] = index.Items[i].Kind + " " + index.Items[i].Source + " " + index.Items[i].Summary
		}
		if vectors, err := embedder.Embed(context.Background(), texts); err == nil && len(vectors) == len(index.Items) {
			for i := range index.Items {
				index.Items[i].Embedding = vectors[i]
			}
		}
	}
	index.ID = ContextIndexSnapshotID(index)
	return index
}

func (r HybridContextRetriever) Retrieve(ctx context.Context, query RetrievalQuery) (RetrievalResult, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	budget := query.DetailBudget
	if budget <= 0 {
		budget = 1200
	}
	queryText := strings.TrimSpace(query.QueryText)
	if queryText == "" {
		queryText = query.Command.Instruction
	}
	scope := r.Scope
	if scope.Object == "" {
		scope = query.Command.Scope
	}
	allowedKinds := map[string]bool{}
	for _, kind := range query.Kinds {
		allowedKinds[kind] = true
	}
	var queryVector []float32
	if r.Embedder != nil {
		vectors, err := r.Embedder.Embed(ctx, []string{queryText})
		if err == nil && len(vectors) == 1 {
			queryVector = vectors[0]
		}
	}
	candidates := make([]RetrievedContextItem, 0, len(r.Index.Items))
	for _, item := range r.Index.Items {
		if len(allowedKinds) > 0 && !allowedKinds[item.Kind] {
			continue
		}
		if !retrievalScopeAllows(scope, item.Target) {
			continue
		}
		if item.Freshness == "stale" {
			continue
		}
		keyword := normalizedKeywordScore(queryText, item.Kind+" "+item.Source+" "+item.Summary)
		semantic := cosine(queryVector, item.Embedding)
		scopeBoost := 0.0
		if item.Target.Key() == resourceForRunScope(query.Command.Scope).Key() {
			scopeBoost = 1
		}
		freshness := 0.5
		if item.Freshness == "current" {
			freshness = 1
		}
		issueBoost := issueRelevance(query.LatestIssues, item)
		score := semantic*0.45 + keyword*0.25 + scopeBoost*0.15 + freshness*0.10 + issueBoost*0.05
		if score <= 0 && strings.TrimSpace(queryText) != "" {
			continue
		}
		level := DetailSummary
		cost := item.TokenCost[level]
		if cost == 0 {
			cost = approximateTokens(item.Summary)
		}
		candidates = append(candidates, RetrievedContextItem{
			RefID: item.RefID, Kind: item.Kind, Source: item.Source, Target: item.Target,
			Revision: item.Revision, Hash: item.Hash, Score: math.Round(score*10000) / 10000,
			SelectionReason: selectionReason(queryText, item, keyword, semantic, scopeBoost, issueBoost),
			DetailAvailable: len(item.AvailableLevels) > 1, DetailLevel: level,
			Snippet: compactSnippet(item.Summary, 1200), Freshness: item.Freshness, EstimatedTokens: cost,
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

func targetForSegment(segment contextengine.ContextSegment) Resource {
	source := segment.SourceRef
	if strings.Contains(source, "/design") || strings.HasPrefix(source, "theme://") {
		return Resource{Type: "deck", Part: "design"}
	}
	if strings.Contains(source, "/outline") || strings.Contains(source, "related-slides") {
		return Resource{Type: "deck", Part: "outline"}
	}
	if strings.HasPrefix(source, "slide://") {
		rest := strings.TrimPrefix(source, "slide://")
		id := strings.Split(rest, "/")[0]
		part := "spec"
		if strings.Contains(source, "/html") {
			part = "html"
		}
		return Resource{Type: "slide", SlideID: id, Part: part}
	}
	return Resource{Type: "deck", Part: "outline"}
}

func memoryIndexItem(kind, source string, pack contextengine.ContextPack, item contextengine.MemoryItem) ContextIndexItem {
	return ContextIndexItem{
		Kind: kind, Source: source, Revision: pack.Revisions.ThreadMemory,
		Summary: item.Key + ": " + item.Value, Freshness: "current",
	}
}

func retrievalScopeAllows(scope model.RunScope, target Resource) bool {
	if target.Type == "" {
		return true
	}
	return AllowsRead(scope, target)
}

func resourceForRunScope(target model.RunScope) Resource {
	if target.IsSinglePage() {
		part := "spec"
		if target.AllowsHTML() {
			part = "html"
		}
		return Resource{Type: "slide", SlideID: target.SlideIDs[0], Part: part}
	}
	if target.AllowsHTML() {
		return Resource{Type: "deck", Part: "design"}
	}
	return Resource{Type: "deck", Part: "outline"}
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

func selectionReason(query string, item ContextIndexItem, keyword, semantic, scopeBoost, issueBoost float64) string {
	reasons := []string{}
	if keyword > 0 {
		reasons = append(reasons, "keyword match for query")
	}
	if semantic > 0 {
		reasons = append(reasons, "semantic similarity")
	}
	if scopeBoost > 0 {
		reasons = append(reasons, "matches current RunScope")
	}
	if issueBoost > 0 {
		reasons = append(reasons, "matches latest runtime issue")
	}
	if item.Freshness == "current" {
		reasons = append(reasons, "current revision")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "authorized context candidate")
	}
	return strings.Join(reasons, "; ")
}

func uniqueKeywords(text string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, term := range strings.Fields(strings.ToLower(text)) {
		term = strings.Trim(term, ".,;:!?()[]{}\"'")
		if len([]rune(term)) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
		if len(out) >= 24 {
			break
		}
	}
	return out
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

func normalizeVector(vec []float32) {
	sum := 0.0
	for _, value := range vec {
		sum += float64(value * value)
	}
	if sum == 0 {
		return
	}
	length := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] /= length
	}
}

func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	sum := float64(0)
	for i := range a {
		sum += float64(a[i] * b[i])
	}
	if sum < 0 {
		return 0
	}
	if sum > 1 {
		return 1
	}
	return sum
}
