package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
)

type cacheCountingRenderer struct {
	calls     int
	resources []string
}

func (r *cacheCountingRenderer) Render(ctx context.Context, request RenderRequest) (RenderDiagnostics, error) {
	r.calls++
	diagnostics, err := (reviewPNGRenderer{}).Render(ctx, request)
	if err != nil {
		return diagnostics, err
	}
	diagnostics.ResourceHashes = map[string]string{}
	for _, name := range r.resources {
		raw, err := os.ReadFile(filepath.Join(request.ProjectDir, name))
		if os.IsNotExist(err) {
			diagnostics.ResourceHashes[name] = "missing"
		} else if err != nil {
			return diagnostics, err
		} else {
			diagnostics.ResourceHashes[name] = runtimeassets.Hash(raw)
		}
	}
	for _, selector := range request.InspectSelectors {
		diagnostics.StyleInspections = append(diagnostics.StyleInspections, map[string]any{"selector": selector, "match_count": 0, "elements": []any{}})
	}
	return diagnostics, nil
}

func TestRenderCacheReusesSnapshotsAndChecksAllDependencies(t *testing.T) {
	ctx := context.Background()
	dir, css, pack := generationPackFixture(t)
	writeGenerationFile(t, dir, "image.svg", []byte("first image"))
	renderer := &cacheCountingRenderer{resources: []string{"image.svg", "missing.svg"}}
	tool := slideRenderTool{renderer: renderer, themes: staticThemeLoader{theme: model.Theme{ID: "clean", CSS: css, ResourceContentState: model.ResourceContentState{ContentState: "ready"}}}}
	input := DomainToolInput{RunID: "original_run", ProjectDir: dir, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"slide_id": generationSlide}}
	render := func(wantCalls int, wantCached bool) ToolResult {
		t.Helper()
		result := tool.Execute(ctx, input)
		if !result.OK || renderer.calls != wantCalls || result.Data["cached"] != wantCached {
			t.Fatalf("render calls=%d, want %d; result=%+v", renderer.calls, wantCalls, result)
		}
		if images, err := latestRenderedImages(pack, dir, nil); err != nil || len(images) != 1 || images[0].Stale {
			t.Fatalf("render is already stale: %+v %v", images, err)
		}
		return result
	}
	first := render(1, false)
	input.RunID = "later_run"
	reused := render(1, true)
	if first.Data["screenshot_ref"] != reused.Data["screenshot_ref"] || first.Data["screenshot_url"] != reused.Data["screenshot_url"] {
		t.Fatal("cache changed the immutable screenshot identity")
	}
	if preview := publicRenderPreview(input.Args, reused); preview == nil || preview.ImageURL != first.Data["screenshot_url"] {
		t.Fatal("cross-run cached screenshot lost its history preview")
	}
	input.Args["inspect_selectors"] = []any{"h1", ".quote"}
	render(2, false)
	input.Args["inspect_selectors"] = []any{"h1"}
	inspected := render(2, true)
	if got := inspected.Data["model_diagnostics"].(map[string]any)["style_inspections"].([]map[string]any); len(got) != 1 || got[0]["selector"] != "h1" {
		t.Fatal("cache did not return exactly the requested inspections")
	}
	delete(input.Args, "inspect_selectors")
	writeGenerationFile(t, dir, "image.svg", []byte("changed image"))
	render(3, false)
	writeGenerationFile(t, dir, "missing.svg", []byte("now available"))
	render(4, false)
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(generationHTML+"\n<!-- edit -->"))
	render(5, false)
	themePath := filepath.Join(dir, "..", "..", "..", "assets", "themes", "clean", "theme.css")
	css += "\nh1{color:red}"
	if err := os.WriteFile(themePath, []byte(css), 0644); err != nil {
		t.Fatal(err)
	}
	tool.themes = staticThemeLoader{theme: model.Theme{ID: "clean", CSS: css, ResourceContentState: model.ResourceContentState{ContentState: "ready"}}}
	render(6, false)
	design := *pack.Design.Design
	design.Decorations.PageNumber = "none"
	raw, _ := json.Marshal(design)
	writeGenerationFile(t, dir, ".design.json", raw)
	last := render(7, false)
	if _, _, err := renderimage.Read(ctx, dir, pack.Project.ID, first.Data["screenshot_ref"].(string)); err != nil {
		t.Fatalf("historical screenshot was overwritten: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, last.Data["image_path"].(string))); err != nil {
		t.Fatal(err)
	}
	render(8, false)
}

func TestLatestRunScreenshotReplacesPixelsInHistoryAndRecovery(t *testing.T) {
	oldRef, newRef := "project:p1/render:sli_1/shot_old", "project:p1/render:sli_1/shot_new"
	attachment := "project:p1/attachment:att_one/original"
	state := &RunState{readImages: []RunReadImage{{SlideID: "sli_1", ImagePath: "old.png", ImageRef: oldRef, MIMEType: "image/png"}, {AttachmentID: "att_one", ImageRef: attachment, MIMEType: "image/png"}}}
	result := SuccessfulToolResult("new screenshot")
	result.Data = map[string]any{"image_path": "new.png"}
	result.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: newRef, MIMEType: "image/png"}}
	if !state.rememberReadImages([]llm.ToolCall{{ID: "new", Name: "render_slide", Args: map[string]any{"slide_id": "sli_1"}}}, []ToolResult{result}) || len(state.readImages) != 2 {
		t.Fatal("new screenshot did not replace the old retained record")
	}
	raw, _ := json.Marshal(state.readImages)
	var restored []RunReadImage
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	request := AgentRequest{RunID: "run", Context: testPack(model.ModeChat, model.ScopeCurrentPage, false, "inspect"), ReadImages: restored,
		RenderedImages: []RenderedImageContext{{SlideID: "sli_1", ImagePath: "new.png"}},
		Messages: []llm.Message{
			{Role: llm.RoleTool, ToolCallID: "old", Content: []llm.ContentPart{{Type: "text", Text: "historical diagnostics"}, {Type: "image", ImageRef: oldRef}}},
			{Role: llm.RoleTool, ToolCallID: "new", Content: result.ObservationParts},
			{Role: llm.RoleUser, Content: []llm.ContentPart{{Type: "image", ImageRef: attachment}}},
		}}
	prepared := prepareAgentRequest(request)
	if got := requestImageCounts(prepared.Messages); !reflect.DeepEqual(got, map[string]int{newRef: 1, attachment: 1}) {
		t.Fatalf("incorrect provider pixels: %v", got)
	}
	if prepared.Messages[0].ToolCallID != "old" || prepared.Messages[0].Text() != "historical diagnostics" {
		t.Fatal("removing historical pixels changed tool pairing or diagnostics")
	}
	request.RenderedImages[0].Stale = true
	if got := requestImageCounts(prepareAgentRequest(request).Messages); !reflect.DeepEqual(got, map[string]int{attachment: 1}) {
		t.Fatalf("stale pixels survived in request history: %v", got)
	}
}
