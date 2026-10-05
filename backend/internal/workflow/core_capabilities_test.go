package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestKeywordRetrieverFiltersScopeFreshnessOrdersAndBudgets(t *testing.T) {
	scope := model.NewRunScope(model.ScopeCurrentPage, "sli_1")
	index := ContextIndex{RunID: "r", ID: "idx", Items: []ContextIndexItem{
		{
			RefID: "target", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "sli_1", Part: "html"},
			Summary: "pricing roadmap and launch story", Freshness: "current",
			Hash: "h1", TokenCost: 100,
		},
		{
			RefID: "other-slide", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "sli_2", Part: "html"},
			Summary: "pricing roadmap", Freshness: "current",
			Hash: "h2", TokenCost: 100,
		},
		{
			RefID: "stale", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "sli_1", Part: "html"},
			Summary: "pricing roadmap stale", Freshness: "stale",
			Hash: "h3", TokenCost: 100,
		},
		{
			RefID: "expensive", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "sli_1", Part: "html"},
			Summary: "pricing roadmap appendix", Freshness: "current",
			Hash: "h4", TokenCost: 2000,
		},
		{
			RefID: "metadata-only", Kind: "pricing roadmap", Source: "pricing roadmap",
			Target:  Resource{Type: "slide", SlideID: "sli_3", Part: "html"},
			Summary: "unrelated budget notes", Freshness: "current",
			Hash: "h5", TokenCost: 20,
		},
		{
			RefID: "empty-content", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "sli_1", Part: "html"},
			Summary: " ", Freshness: "current",
			Hash: "h6", TokenCost: 20,
		},
	}}
	result, err := (KeywordContextRetriever{
		Index: index, Scope: scope,
	}).Retrieve(context.Background(), RetrievalQuery{
		Command: model.RunCommand{
			Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_1"),
		},
		QueryText: "pricing roadmap", Limit: 10, TokenBudget: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 2 || result.Results[0].RefID != "target" || result.Results[1].RefID != "other-slide" {
		t.Fatalf("unexpected retrieval result=%+v", result.Results)
	}
	if result.Results[0].Score <= 0 || !strings.Contains(result.Results[0].SelectionReason, "current content") {
		t.Fatalf("missing score/reason: %+v", result.Results[0])
	}
	if result.EstimatedTokens != 200 || result.RemainingBudget != 100 {
		t.Fatalf("non-content candidates consumed the retrieval budget: %+v", result)
	}
}

func TestTurnContextRetrievalReusesStableQueryAndInjectsSummary(t *testing.T) {
	trace := &traceRecorder{}
	input := RuntimeInput{Trace: trace}
	runtime := NewRuntime(nil)
	state := &RunState{
		runID: "r", loopID: "loop", phase: PhaseChat,
		scope: model.NewRunScope(model.ScopeAllPages),
		pack: contextengine.ContextPack{
			Command: model.RunCommand{Instruction: "pricing roadmap"},
		},
		plan: &Plan{Title: "Pricing plan", Content: "launch story", Steps: []PlanStep{{ID: "step_noise", Status: PlanStepProcessing}}},
		contextIndex: ContextIndex{ID: "idx", Items: []ContextIndexItem{{
			RefID: "ref", Kind: "slide_html", Source: "context_manifest",
			Summary: "pricing roadmap and launch story", Freshness: "current",
			Hash: "hash", TokenCost: 20,
		}}},
		ledger: NewEvidenceLedger(),
	}

	if err := runtime.retrieveTurnContext(context.Background(), input, state); err != nil {
		t.Fatal(err)
	}
	if len(trace.events) != 1 || !strings.Contains(state.contextBriefing, "summary: pricing roadmap and launch story") {
		t.Fatalf("retrievals=%d briefing=%q", len(trace.events), state.contextBriefing)
	}
	query := trace.events[0].Payload["query"].(string)
	if !strings.Contains(query, "launch story") || strings.Contains(query, "step_noise") || strings.Contains(query, "processing") {
		t.Fatalf("retrieval query must contain plan content instead of status metadata: %q", query)
	}
	if err := runtime.retrieveTurnContext(context.Background(), input, state); err != nil {
		t.Fatal(err)
	}
	if len(trace.events) != 1 {
		t.Fatalf("stable retrieval query executed again: retrievals=%d", len(trace.events))
	}

	state.issues = append(state.issues, Issue{Code: "NEEDS_REVIEW", Summary: "verify launch story"})
	if err := runtime.retrieveTurnContext(context.Background(), input, state); err != nil {
		t.Fatal(err)
	}
	if len(trace.events) != 2 {
		t.Fatalf("changed retrieval query was not executed: retrievals=%d", len(trace.events))
	}
	query = trace.events[1].Payload["query"].(string)
	if !strings.Contains(query, "verify launch story") || strings.Contains(query, "NEEDS_REVIEW") {
		t.Fatalf("retrieval query must contain issue content instead of code metadata: %q", query)
	}
}

func TestReconcileDirectWritesClassifiesArtifactState(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, value string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		return hashBytes([]byte(value))
	}
	write(model.SpecCollectionPath, `{"sli_1":{"core":"after","elements":[]}}`)
	after := hashBytes([]byte(`{"core":"after","elements":[]}`))
	external := write(model.SlideHTMLPath("s1"), "external")
	checkpoint := RuntimeCheckpoint{RunID: "r", Changes: ChangeSet{Updated: []ArtifactChange{
		{Artifact: ArtifactRef{Kind: ArtifactSlideSpec, ID: "sli_1"}, AfterHash: after},
		{Artifact: ArtifactRef{Kind: ArtifactSlideHTML, ID: "s1"}, BeforeHash: "before", AfterHash: "expected"},
		{Artifact: ArtifactRef{Kind: ArtifactDesign, ID: "deck"}, AfterHash: "missing"},
	}}}
	if external == "expected" {
		t.Fatal("test setup invalid")
	}
	snapshot, err := ReconcileDirectWrites(context.Background(), dir, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[ArtifactKind]string{}
	for _, result := range snapshot.Results {
		statuses[result.Artifact.Kind] = result.Status
	}
	if statuses[ArtifactSlideSpec] != ReconcileClean ||
		statuses[ArtifactSlideHTML] != ReconcileExternalModified ||
		statuses[ArtifactDesign] != ReconcileMissingArtifact {
		t.Fatalf("statuses=%+v", statuses)
	}
}
