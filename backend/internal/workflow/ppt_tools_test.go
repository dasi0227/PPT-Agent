package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type successfulScreenshotRenderer struct{}

func (successfulScreenshotRenderer) Render(_ context.Context, request RenderRequest) (RenderDiagnostics, error) {
	if err := os.WriteFile(request.ScreenshotPath, []byte("screenshot"), 0o600); err != nil {
		return RenderDiagnostics{}, err
	}
	return RenderDiagnostics{
		ScreenshotBytes: 10,
		ContentSize:     map[string]int{"width": request.ViewportWidth, "height": request.ViewportHeight},
		Overflow:        map[string]bool{"horizontal": false, "vertical": false},
		FontStatus:      "loaded",
	}, nil
}

type staticThemeLoader struct {
	theme model.Theme
}

func (l staticThemeLoader) Get(string) (model.Theme, error) {
	return l.theme, nil
}

func mutationPack(projectID string, outline spec.Outline) contextengine.ContextPack {
	return contextengine.ContextPack{Project: contextengine.ProjectContext{ID: projectID, ThemeID: "clean"}, PresentationManifest: contextengine.PresentationManifestContext{Manifest: spec.Manifest{}}, Outline: contextengine.OutlineContext{Outline: outline}}
}

func TestToolSchemasDoNotEmitNullRequired(t *testing.T) {
	empty := spec.Outline{Sections: []spec.Section{}}
	nonEmpty := spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover"}}}}}
	plan := &Plan{ApprovalID: "approval-test", ID: "plan_1", Status: PlanActive}

	for _, outline := range []spec.Outline{empty, nonEmpty} {
		pack := mutationPack("pro_aaaaaa", outline)
		registry := NewToolRegistry()
		if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name    string
			phase   RunPhase
			mode    model.RunMode
			scope   model.RunScope
			control []ToolSchema
		}{
			{
				name:    "plan deck",
				phase:   PhasePlanning,
				mode:    model.ModePlan,
				scope:   model.NewRunScope(model.ScopeAllPages),
				control: controlSchemas(PhasePlanning, model.ModePlan, nil),
			},
			{
				name:    "execute deck",
				phase:   PhaseExecuting,
				mode:    model.ModeExecute,
				scope:   model.NewRunScope(model.ScopeAllPages),
				control: controlSchemas(PhaseExecuting, model.ModeExecute, plan),
			},
			{
				name:    "execute slide",
				phase:   PhaseExecuting,
				mode:    model.ModeExecute,
				scope:   model.NewRunScope(model.ScopeCurrentPage, "sli_aaaaaa"),
				control: controlSchemas(PhaseExecuting, model.ModeExecute, plan),
			},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				schemas := append(registry.Disclose(test.phase, test.mode, test.scope), test.control...)
				for _, schema := range schemas {
					var wire any
					raw, err := json.Marshal(schema.Parameters)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(raw, &wire); err != nil {
						t.Fatal(err)
					}
					assertProviderToolSchema(t, schema.Name, wire)
				}
			})
		}
	}
}

func TestToolRegistryRejectsIncoherentCapabilityPolicy(t *testing.T) {
	registry := NewToolRegistry()
	tool := resourceEditTool{pack: mutationPack("pro_aaaaaa", spec.Outline{}), name: "edit_manifest"}
	if err := registry.Register(tool, false, CapabilityPPTRead, RiskMedium, PhaseExecuting); err == nil {
		t.Fatal("write tool accepted a read-only capability")
	}
	if err := registry.Register(tool, false, ToolCapability("ppt.unknown"), RiskMedium, PhaseExecuting); err == nil {
		t.Fatal("unknown capability was accepted")
	}
}

func TestToolRegistryRejectsScopeOrModeThatDriftsFromRunCommand(t *testing.T) {
	pack := mutationPack("pro_aaaaaa", spec.Outline{})
	pack.Command = model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeExecute, Instruction: "read deck"}
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	result := registry.Execute(context.Background(), map[string]bool{"read_resource": true}, "read_resource", map[string]any{}, DomainToolInput{
		Context: pack, Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_other"),
		Mode: model.ModeExecute, Phase: PhaseExecuting,
	})
	if result.Code != ErrCapabilityDenied.Error() {
		t.Fatalf("result=%+v", result)
	}
}

func assertProviderToolSchema(t *testing.T, name string, value any) {
	t.Helper()
	root, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s.parameters encoded as %T, want object", name, value)
	}
	if root["type"] != "object" {
		t.Fatalf("%s.parameters.type=%#v, want object", name, root["type"])
	}
	assertNoNullRequired(t, name, value)
}

func assertNoNullRequired(t *testing.T, path string, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if required, exists := typed["required"]; exists {
			if required == nil {
				t.Fatalf("%s.required encoded as null", path)
			}
			if _, ok := required.([]any); !ok {
				t.Fatalf("%s.required encoded as %T, want array", path, required)
			}
		}
		for key, child := range typed {
			assertNoNullRequired(t, path+"."+key, child)
		}
	case []any:
		for index, child := range typed {
			assertNoNullRequired(t, fmt.Sprintf("%s[%d]", path, index), child)
		}
	}
}

func TestRuntimeFrameForRenderUsesCurrentOutlineOrdinal(t *testing.T) {
	dir, _ := renderThemeFixture(t)
	projectID := "pro_aaaaaa"
	deck := spec.Manifest{Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Pages: "待明确", Requirements: []string{}, Prohibitions: []string{}}
	outline := spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover"}, {SlideID: "sli_bbbbbb", Title: "Body"}}, Subsections: []spec.Subsection{}}}}
	design := spec.Design{LayoutPreferences: []string{}, Direction: "minimal", Decorations: spec.Decorations{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "none", KeyMessage: "none"}}
	for path, value := range map[string]any{".manifest.json": deck, ".outline.json": outline, ".design.json": design, model.SpecCollectionPath: map[string]spec.SlideSpec{"sli_bbbbbb": {KeyMessage: "Message", Elements: []spec.Element{}}}} {
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(dir, path), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	frame, err := runtimeFrameForRender(mutationPack(projectID, outline), dir, nil, "sli_bbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if frame.Ordinal != 2 || frame.Total != 2 || frame.Canvas != spec.CanonicalCanvas() {
		t.Fatalf("frame=%#v", frame)
	}
}

func TestRenderSlideUsesHTMLArtifactHashWhenThemeCSSIsPresent(t *testing.T) {
	dir, themeCSS := renderThemeFixture(t)
	projectID := "pro_aaaaaa"
	slideID := "sli_attea2"
	deck := spec.Manifest{Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Pages: "待明确", Requirements: []string{}, Prohibitions: []string{}}
	outline := spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: slideID, Title: "Cover"}}, Subsections: []spec.Subsection{}}}}
	design := spec.Design{LayoutPreferences: []string{}, Direction: "minimal", Decorations: spec.DefaultDecorations()}
	slide := spec.SlideSpec{KeyMessage: "Hello", Elements: []spec.Element{}}
	html := []byte(`<!doctype html><html><body><section class="slide-stage"><h1>Hello</h1></section></body></html>`)
	for path, value := range map[string]any{
		".manifest.json":         deck,
		".outline.json":          outline,
		".design.json":           design,
		model.SpecCollectionPath: map[string]spec.SlideSpec{slideID: slide},
	} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	htmlPath := filepath.Join(dir, model.SlideHTMLPath(slideID))
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_1")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	pack := mutationPack(projectID, outline)
	result := (slideRenderTool{
		pack:     pack,
		renderer: successfulScreenshotRenderer{},
		themes:   staticThemeLoader{theme: model.Theme{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "clean", CSS: themeCSS}},
	}).Execute(context.Background(), DomainToolInput{
		Args: map[string]any{"slide_id": slideID}, RunID: "run_1", ProjectDir: dir, Session: session, Context: pack,
		Scope: model.NewRunScope(model.ScopeAllPages, slideID),
	})
	if !result.OK {
		t.Fatalf("render result=%+v", result)
	}
	wantHash := hashBytes(html)
	if result.Data["hash"] != wantHash || len(result.Evidence) != 2 || result.Evidence[0].SourceHash != wantHash || result.Evidence[1].Kind != "static" || result.Evidence[1].SourceHash != wantHash {
		t.Fatalf("render hash=%v evidence=%+v want=%s", result.Data["hash"], result.Evidence, wantHash)
	}
	if result.Evidence[0].Render == nil || result.Evidence[0].Render.ArtifactHash != wantHash {
		t.Fatalf("render proof=%+v want artifact hash %s", result.Evidence[0].Render, wantHash)
	}
	for _, part := range result.ObservationParts {
		if part.Type == "image" {
			t.Fatal("render automatically attached image pixels")
		}
	}
	images := latestRenderedImages(pack, dir, nil)
	if len(images) != 1 || images[0].ImagePath != result.Data["image_path"] || images[0].Stale {
		t.Fatalf("latest render index is missing or stale: %+v", images)
	}
	themePath := filepath.Join(dir, "..", "..", "..", "assets", "themes", "clean", "theme.css")
	if err := os.WriteFile(themePath, []byte(themeCSS+"\n.slide-title {color:red}"), 0644); err != nil {
		t.Fatal(err)
	}
	if changed := latestRenderedImages(pack, dir, nil); len(changed) != 1 || !changed[0].Stale {
		t.Fatal("CSS change under the same theme ID did not invalidate the screenshot")
	}
	if err := os.WriteFile(themePath, []byte(themeCSS), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlPath, append(html, []byte("\n<!-- changed -->")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if updated := latestRenderedImages(pack, dir, nil); len(updated) != 1 || !updated[0].Stale {
		t.Fatalf("changed HTML did not invalidate render freshness: %+v", updated)
	}
}

func renderThemeFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "pro_aaaaaa", "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "seed", "assets", "themes", "editorial-serif", "theme.css"))
	if err != nil {
		t.Fatal(err)
	}
	themePath := filepath.Join(root, "assets", "themes", "clean", "theme.css")
	if err := os.MkdirAll(filepath.Dir(themePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themePath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	return dir, string(raw)
}
