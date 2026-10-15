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
	material, err := prepareReviewMaterial(ctx, state, "检查本次成果", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(material.Changes) != 3 {
		t.Fatalf("material=%+v", material)
	}
	parts, err := material.content()
	if err != nil {
		t.Fatal(err)
	}
	text := (llm.Message{Content: parts}).Text()
	for _, excluded := range []string{"original", "correction", "evidence_version", "task_scope", "user_instructions", "task_changes", "resource_availability", "image_evidence", "before_hash", "after_hash"} {
		if strings.Contains(text, excluded) {
			t.Fatalf("unexpected review background: %s", excluded)
		}
	}
	if !strings.Contains(text, "manifest (.manifest.json)\n[missing]") || !strings.Contains(text, "outline (.outline.json)\n[missing]") {
		t.Fatal("missing resources must be marked in place")
	}
	for _, change := range material.Changes {
		if change.Path == "sli_1.html" && (!strings.Contains(change.Diff, "-initial") || !strings.Contains(change.Diff, "+final") || strings.Contains(change.Diff, "intermediate")) {
			t.Fatalf("not a cumulative diff: %s", change.Diff)
		}
	}
	write("sli_1.html", "initial\n")
	material, err = prepareReviewMaterial(ctx, state, "再次检查", nil)
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
	pack.Outline.Outline.Sections[0].Slides = []spec.SlideNode{{ID: "sli_2", Title: "Second"}, {ID: "sli_1", Title: "First"}, {ID: "sli_missing", Title: "Missing"}}
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
	write(".design.json", spec.Design{Requirements: []string{"保持克制"}, Prohibitions: []string{}})
	write(model.SlideSpecPath("sli_1"), spec.SlideSpec{Core: "关键观点", Elements: []spec.Element{}})
	write(model.SlideSpecPath("sli_2"), spec.SlideSpec{Core: "第二观点", Elements: []spec.Element{}})
	html := `<!doctype html><html><body><section class="slide-stage"><h1>Before</h1></section></body></html>`
	if err := os.WriteFile(filepath.Join(root, "sli_1.html"), []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sli_2.html"), []byte(strings.ReplaceAll(html, "Before", "Second")), 0o600); err != nil {
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
		return (slideRenderTool{renderer: reviewPNGRenderer{}, themes: staticThemeLoader{theme: model.Theme{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "clean", CSS: css}}}).Execute(ctx, DomainToolInput{Args: map[string]any{"slide_id": id, "need_preview": false}, Context: pack, ProjectDir: root, RunID: state.runID, Session: session, Scope: scope}), nil
	}
	material, err := prepareReviewMaterial(ctx, state, "核对页面", render)
	if err != nil {
		t.Fatal(err)
	}
	if renders != 2 || len(material.Changes) != 1 || !strings.Contains(material.Changes[0].Diff, "+<!doctype") || material.sourceHash == "" {
		t.Fatalf("incomplete material: %+v renders=%d", material, renders)
	}
	parts, err := material.content()
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 20 || !strings.HasPrefix(parts[3].Text, "manifest (") || !strings.HasPrefix(parts[4].Text, "design (") || !strings.HasPrefix(parts[5].Text, "outline (") {
		t.Fatalf("global material order: %+v", parts)
	}
	for _, page := range []struct {
		offset         int
		id, core, html string
	}{{6, "sli_2", "第二观点", "Second"}, {11, "sli_1", "关键观点", "Current"}} {
		if !strings.Contains(parts[page.offset].Text, page.id) || !strings.Contains(parts[page.offset+1].Text, page.core) || !strings.Contains(parts[page.offset+2].Text, page.html) || !strings.Contains(parts[page.offset+3].Text, page.id) || parts[page.offset+4].Type != "image" {
			t.Fatalf("source/screenshot not adjacent for %s", page.id)
		}
		if !strings.Contains(parts[page.offset+2].Text, "<!doctype html>") || !strings.HasSuffix(parts[page.offset+2].Text, "</html>") {
			t.Fatal("HTML must remain complete")
		}
	}
	if !strings.Contains(parts[16].Text, "sli_missing") || !strings.Contains(parts[17].Text, "[missing]") || !strings.Contains(parts[18].Text, "[missing]") || !strings.Contains(parts[19].Text, "HTML source is missing") {
		t.Fatal("missing page resources must be marked in place")
	}
	counts := requestImageCounts([]llm.Message{{Content: parts}})
	if len(counts) != 2 || len(material.imageData) != 2 || counts[meta.ImageRef(pack.Project.ID, "original")] != 0 {
		t.Fatalf("screenshots missing or uploaded reference disclosed: %v", counts)
	}
	if _, err := prepareReviewMaterial(ctx, state, "再次核对", render); err != nil || renders != 2 {
		t.Fatalf("valid screenshot rendered again: %d %v", renders, err)
	}
	first := parts[10].ImageRef
	frozen, err := reviewImageResolver(material.imageData).ResolveImage(ctx, first)
	if err != nil || !bytes.Equal(frozen.Bytes, material.imageData[first].Bytes) || frozen.MIMEType != "image/png" {
		t.Fatal("frozen screenshot pixels missing")
	}
	frozen.Bytes[0] ^= 1
	if bytes.Equal(frozen.Bytes, material.imageData[first].Bytes) {
		t.Fatal("resolver must not expose mutable snapshot pixels")
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
	if _, err := prepareReviewMaterial(ctx, state, "缺失截图", nil); err == nil {
		t.Fatal("stale required screenshot was silently omitted")
	}
	if _, err := prepareReviewMaterial(ctx, state, "渲染失败", func(context.Context, string, model.RunScope) (ToolResult, error) {
		return failedToolResult(model.CodeInternal, "renderer unavailable"), nil
	}); err == nil || reviewFailure(err).Code != model.CodeInternal {
		t.Fatalf("render infrastructure failure changed during review: %v", err)
	}
}
