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

func TestSemanticReviewParseChecksContract(t *testing.T) {
	raw := `{"checks":[{"code":"REVIEW_INTENT_MISMATCH","summary":"用户要求更新第 3 页，但当前结果只显示第 2 页发生了变化，需要主 Agent 继续核对目标页。"}]}`
	result, err := ParseSemanticReviewResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Checks) != 1 || result.Checks[0].Code != "REVIEW_INTENT_MISMATCH" {
		t.Fatalf("parsed checks=%+v", result.Checks)
	}
	if _, err := ParseSemanticReviewResult(`{"checks":[]}`); err == nil {
		t.Fatal("expected empty checks rejection")
	}
	if _, err := ParseSemanticReviewResult(`{"checks":[{"code":"REVIEW_PASS","summary":"ok"}]}`); err == nil {
		t.Fatal("expected vague summary rejection")
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
