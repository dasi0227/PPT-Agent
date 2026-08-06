package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestHybridRetrieverFiltersScopeFreshnessOrdersAndBudgets(t *testing.T) {
	scope := ScopeFromSpec(model.WorkSpec{
		Target: model.RunTarget{Artifact: model.ArtifactPresentation, Level: model.TargetSlide, SlideID: "s1"},
	})
	index := ContextIndex{RunID: "r", ID: "idx", Items: []ContextIndexItem{
		{
			RefID: "target", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "s1", Part: "html"},
			Summary: "pricing roadmap and launch story", Freshness: "current",
			Hash: "h1", TokenCost: map[DetailLevel]int{DetailSummary: 100},
		},
		{
			RefID: "other-slide", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "s2", Part: "html"},
			Summary: "pricing roadmap", Freshness: "current",
			Hash: "h2", TokenCost: map[DetailLevel]int{DetailSummary: 100},
		},
		{
			RefID: "stale", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "s1", Part: "html"},
			Summary: "pricing roadmap stale", Freshness: "stale",
			Hash: "h3", TokenCost: map[DetailLevel]int{DetailSummary: 100},
		},
		{
			RefID: "expensive", Kind: "slide_html", Source: "context_index",
			Target:  Resource{Type: "slide", SlideID: "s1", Part: "html"},
			Summary: "pricing roadmap appendix", Freshness: "current",
			Hash: "h4", TokenCost: map[DetailLevel]int{DetailSummary: 2000},
		},
	}}
	result, err := (HybridContextRetriever{
		Index: index, Scope: scope, Embedder: HashEmbeddingProvider{},
	}).Retrieve(context.Background(), RetrievalQuery{
		WorkSpec: model.WorkSpec{
			Target: model.RunTarget{Artifact: model.ArtifactPresentation, Level: model.TargetSlide, SlideID: "s1"},
		},
		QueryText: "pricing roadmap", Limit: 10, DetailBudget: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].RefID != "target" {
		t.Fatalf("unexpected retrieval result=%+v", result.Results)
	}
	if result.Results[0].Score <= 0 || !strings.Contains(result.Results[0].SelectionReason, "current revision") {
		t.Fatalf("missing score/reason: %+v", result.Results[0])
	}
}

func TestSemanticReviewParseAndIssueMapping(t *testing.T) {
	raw := `{"accepted":false,"confidence":0.7,"summary":"missing","coverage":[],"issues":[{"code":"FINAL_ANSWER_INCOMPLETE","severity":"error","summary":"final answer omits deliverable","required_action":{"tool":"finish","target":{"type":"deck","part":"outline"}}}]}`
	result, err := ParseSemanticReviewResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	issues := semanticIssuesToCompletion(result.Issues)
	if len(issues) != 1 || issues[0].Code != "FINAL_ANSWER_INCOMPLETE" ||
		issues[0].RequiredActions[0].Tool != "finish" {
		t.Fatalf("mapped issues=%+v", issues)
	}
	if _, err := ParseSemanticReviewResult(`{"accepted":false,"confidence":2,"issues":[]}`); err == nil {
		t.Fatal("expected invalid confidence rejection")
	}
}

func TestReconcileDirectWritesClassifiesArtifactState(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "slides", "s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, value string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		return hashBytes([]byte(value))
	}
	after := write(model.SlideSpecPath("s1"), "after")
	external := write(model.SlideHTMLPath("s1"), "external")
	checkpoint := RuntimeCheckpoint{RunID: "r", Changes: ChangeSet{Updated: []ArtifactChange{
		{Artifact: ArtifactRef{Kind: ArtifactSlideSpec, ID: "s1"}, AfterHash: after, Tentative: true},
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
	if statuses[ArtifactSlideSpec] != ReconcileDirtySameRun ||
		statuses[ArtifactSlideHTML] != ReconcileExternalModified ||
		statuses[ArtifactDesign] != ReconcileMissingArtifact {
		t.Fatalf("statuses=%+v", statuses)
	}
}
