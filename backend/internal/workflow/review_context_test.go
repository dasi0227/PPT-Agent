package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestReviewMaterialUsesRunStartNetChangesAcrossResume(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("sli_1.html", "initial\n")
	write("removed.txt", "removed\n")
	write("unchanged.txt", "same\n")
	if err := ensureReviewBaseline(ctx, root, "run", false); err != nil {
		t.Fatal(err)
	}
	write("sli_1.html", "intermediate\n")
	if err := ensureReviewBaseline(ctx, root, "run", true); err != nil {
		t.Fatal(err)
	}
	write("sli_1.html", "final\n")
	write("added.txt", "added\n")
	if err := os.Remove(filepath.Join(root, "removed.txt")); err != nil {
		t.Fatal(err)
	}
	pack := testPack(model.ModeExecute, model.ScopeAllPages, false, "检查")
	session, err := NewRunSession(root, "run")
	if err != nil {
		t.Fatal(err)
	}
	state := &RunState{runID: "run", projectDir: root, pack: pack, tx: session, reviewInstructions: []ReviewInstruction{{Text: "original"}, {Text: "correction"}}}
	material, _, err := buildReviewMaterial(ctx, state, "检查本次成果")
	if err != nil {
		t.Fatal(err)
	}
	if len(material.Changes) != 3 || len(material.UserInstructions) != 2 {
		t.Fatalf("material=%+v", material)
	}
	for _, change := range material.Changes {
		if change.Path == "sli_1.html" && (!strings.Contains(change.Diff, "-initial") || !strings.Contains(change.Diff, "+final") || strings.Contains(change.Diff, "intermediate")) {
			t.Fatalf("not a cumulative diff: %s", change.Diff)
		}
	}
	write("sli_1.html", "initial\n")
	material, _, err = buildReviewMaterial(ctx, state, "再次检查")
	if err != nil || len(material.Changes) != 2 {
		t.Fatalf("reverted edit should disappear: %+v %v", material.Changes, err)
	}
	if err := ensureReviewBaseline(ctx, root, "missing-resumed-run", true); err == nil {
		t.Fatal("resume must not fabricate a new baseline")
	}
}

type reviewPNGRenderer struct{}

func (reviewPNGRenderer) Render(_ context.Context, request RenderRequest) (RenderDiagnostics, error) {
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		return RenderDiagnostics{}, err
	}
	if err := os.WriteFile(request.ScreenshotPath, pixels.Bytes(), 0o600); err != nil {
		return RenderDiagnostics{}, err
	}
	return RenderDiagnostics{ScreenshotBytes: pixels.Len(), FontStatus: "loaded"}, nil
}

func TestReviewMaterialPreparesCompleteFrozenEvidence(t *testing.T) {
	ctx := context.Background()
	root, css := renderThemeFixture(t)
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "保留全部正文")
	pack.Project.ThemeID = "clean"
	write := func(path string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".manifest.json", pack.PresentationManifest.Manifest)
	write(".outline.json", pack.Outline.Outline)
	write(".design.json", spec.Design{Requirements: []string{"保持克制"}, Decorations: spec.DefaultDecorations()})
	write(model.SpecCollectionPath, map[string]spec.SlideSpec{"sli_1": {KeyMessage: "关键观点", Elements: []spec.Element{}}})
	html := `<!doctype html><html><body><section class="slide-stage"><h1>Before</h1></section></body></html>`
	if err := os.WriteFile(filepath.Join(root, "sli_1.html"), []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}
	var pixels bytes.Buffer
	_ = png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	meta, err := attachment.Create(ctx, root, "p1", "att_reference", "reference.png", bytes.NewReader(pixels.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureReviewBaseline(ctx, root, "current_run", false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sli_1.html"), []byte(strings.ReplaceAll(html, "Before", "Current")), 0o600); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(root, "current_run")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	state := &RunState{runID: "current_run", projectDir: root, pack: pack, tx: session, scope: pack.Command.Scope, reviewInstructions: []ReviewInstruction{{Text: "保留全部正文", Attachments: []model.AttachmentReference{meta.Reference()}}, {Text: "改为中文标题"}}}
	renders := 0
	render := func(ctx context.Context, id string, scope model.RunScope) (ToolResult, error) {
		renders++
		return (slideRenderTool{renderer: reviewPNGRenderer{}, themes: staticThemeLoader{theme: model.Theme{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "clean", CSS: css}}}).Execute(ctx, DomainToolInput{Args: map[string]any{"slide_id": id}, Context: pack, ProjectDir: root, RunID: state.runID, Session: session, Scope: scope}), nil
	}
	material, parts, err := prepareReviewMaterial(ctx, state, "核对页面", render)
	if err != nil {
		t.Fatal(err)
	}
	if renders != 1 || len(material.UserInstructions) != 2 || len(material.Changes) != 1 || len(material.TaskChanges) != 1 || !strings.Contains(material.TaskChanges[0].Diff, "+<!doctype") || material.EvidenceVersion == "" {
		t.Fatalf("incomplete material: %+v renders=%d", material, renders)
	}
	sources := map[string]string{}
	for _, source := range material.Sources {
		sources[source.Path] = source.Content
	}
	for _, path := range []string{".manifest.json", ".outline.json", ".design.json", model.SpecCollectionPath, "sli_1.html"} {
		if sources[path] == "" {
			t.Fatalf("missing current source %s", path)
		}
	}
	if !strings.Contains(sources["sli_1.html"], "Current") {
		t.Fatal("current source is stale")
	}
	counts := requestImageCounts([]llm.Message{{Content: parts}})
	if len(counts) != 2 || len(material.ImageEvidence) != 2 || material.Pages[0].Render.Stale {
		t.Fatalf("missing image pixels: %v", counts)
	}
	if _, _, err := prepareReviewMaterial(ctx, state, "再次核对", render); err != nil || renders != 1 {
		t.Fatalf("valid screenshot rendered again: %d %v", renders, err)
	}
	first := material.ImageEvidence[0]
	frozen, err := reviewImageResolver(material.imageData).ResolveImage(ctx, first.ImageRef)
	if err != nil || hashBytes(frozen.Bytes) != first.Hash {
		t.Fatal("frozen pixels do not match evidence version")
	}
	// Runtime output is derived evidence, never a source change.
	for _, change := range material.Changes {
		if strings.HasPrefix(change.Path, ".runtime/") {
			t.Fatal("render polluted source changes")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "sli_1.html"), []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateReviewEvidence(ctx, state, material); err == nil {
		t.Fatal("stale result accepted after a source edit")
	}
	if _, _, err := buildReviewMaterial(ctx, state, "缺失截图"); err == nil {
		t.Fatal("stale required screenshot was silently omitted")
	}
	if _, _, err := prepareReviewMaterial(ctx, state, "渲染失败", func(context.Context, string, model.RunScope) (ToolResult, error) {
		return failedToolResult(CodeRenderFailed, "failed", false), nil
	}); err == nil {
		t.Fatal("render execution failure masqueraded as evidence")
	}
}
