package contextengine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type fakeStore struct {
	slides map[string]model.Slide
}

type fakeComponentLoader struct{ values []model.Component }

func (l fakeComponentLoader) LoadComponents(context.Context) ([]model.Component, error) {
	return l.values, nil
}

type fakeSkillLoader struct{ values []model.RepositorySkill }

func (l fakeSkillLoader) LoadSkills(context.Context) ([]model.RepositorySkill, error) {
	return l.values, nil
}

func (s *fakeStore) GetSlide(_ context.Context, id string) (model.Slide, error) {
	v, ok := s.slides[id]
	if !ok {
		return model.Slide{}, errors.New("missing")
	}
	return v, nil
}

func fixture(t *testing.T) (model.Project, *fakeStore) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	deck := pptspec.Manifest{Title: "Deck", Goal: "goal", Audience: "leaders", Language: "zh-CN", Pages: "待明确", Requirements: []string{}, Prohibitions: []string{}}
	writeJSON(t, filepath.Join(dir, ".manifest.json"), deck)
	outline := pptspec.Outline{Sections: []pptspec.Section{{ID: "sec_aaaaaa", Title: "Section", Purpose: "Test section", Slides: []pptspec.SlideNode{}, Subsections: []pptspec.Subsection{{ID: "sub_aaaaaa", Title: "Sub", Purpose: "Test subsection", Slides: []pptspec.SlideNode{{ID: "sli_aaaaaa", Title: "One"}, {ID: "sli_bbbbbb", Title: "Two"}, {ID: "sli_cccccc", Title: "Three"}}}}}}}
	writeJSON(t, filepath.Join(dir, ".outline.json"), outline)
	design := pptspec.Design{
		Demands:     []string{"test direction", "Prefer open grids"},
		Decorations: pptspec.Decorations{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "none", KeyMessage: "none"},
	}
	writeJSON(t, filepath.Join(dir, ".design.json"), design)
	slides := map[string]model.Slide{}
	specs := map[string]pptspec.SlideSpec{}
	for _, loc := range pptspec.FlattenOutline(outline) {
		id := loc.Slide.ID
		bp := pptspec.SlideSpec{
			Purpose: "content", ContentType: "explanation",
			Core: "Message " + id,
			Elements: []pptspec.Element{
				{Type: "chart", Intent: "Show growth"},
				{Type: "asset", Intent: "growth chart"},
			},
			Layout: "two-column",
		}
		specs[id] = bp
		html := `<!doctype html><html><head><title>` + id + `</title><style>:root{--color:red}</style></head><body><main id="slide" data-slide="` + id + `"><section class="hero token-accent"><h1>` + loc.Slide.Title + `</h1><img src="asset.png" alt="asset"></section></main></body></html>`
		if err := os.WriteFile(filepath.Join(dir, id+".html"), []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
		slides[id] = model.Slide{ID: id, ProjectID: "p1"}
	}
	writeJSON(t, filepath.Join(dir, ".spec.json"), specs)
	return model.Project{ID: "p1", Title: "Deck", Theme: "swiss-modern", WorkDir: dir}, &fakeStore{slides: slides}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func testScopeCommand(selection model.ScopeSelectionKind) model.RunCommand {
	s := model.RunCommand{
		Scope: model.NewRunScope(selection),
		Mode:  model.ModeExecute, Instruction: "improve target",
	}
	if selection == model.ScopeCurrentPage {
		s.Scope.SlideIDs = []string{"sli_bbbbbb"}
	}
	return s
}

func TestPageProfilesAndStableHash(t *testing.T) {
	project, store := fixture(t)
	assembler := NewContextAssembler(store)
	cases := []struct {
		level   model.ScopeSelectionKind
		profile ProfileID
	}{
		{model.ScopeAllPages, ProfilePPTDeck}, {model.ScopeCurrentPage, ProfilePPTSlide},
	}
	for _, tc := range cases {
		t.Run(string(tc.profile), func(t *testing.T) {
			req := ContextRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: testScopeCommand(tc.level)}
			pack, err := assembler.Assemble(context.Background(), req, project)
			if err != nil {
				t.Fatal(err)
			}
			if pack.Profile != tc.profile {
				t.Fatalf("profile=%s", pack.Profile)
			}
			for _, summary := range pack.Outline.Summaries {
				if summary.Purpose != "content" || summary.ContentType != "explanation" {
					t.Fatalf("page summary did not read Spec purpose and content_type: %+v", summary)
				}
			}
			if tc.level == model.ScopeCurrentPage {
				if pack.Target.SlideSpec == nil || len(pack.Target.SlideIDs) != 1 || pack.Target.SlideIDs[0] != "sli_bbbbbb" || pack.Target.SlideSpec.Core != "Message sli_bbbbbb" {
					t.Fatal("target spec missing")
				}
				for _, related := range pack.RelatedSlides {
					if related.ID == "sli_bbbbbb" {
						t.Fatal("target duplicated as related")
					}
				}
			}
			if pack.Project.ThemeID != project.Theme {
				t.Fatal("runtime project theme was not retained")
			}
			again, err := assembler.Assemble(context.Background(), req, project)
			if err != nil {
				t.Fatal(err)
			}
			if pack.Manifest.PackHash != again.Manifest.PackHash {
				t.Fatal("same input produced different hash")
			}
			req.RunID = "another-run"
			acrossRun, err := assembler.Assemble(context.Background(), req, project)
			if err != nil {
				t.Fatal(err)
			}
			if pack.Manifest.PackHash != acrossRun.Manifest.PackHash {
				t.Fatal("run identity changed semantic pack hash")
			}
		})
	}
}

func TestMentionedPagesRetainHTMLSummary(t *testing.T) {
	project, store := fixture(t)
	command := testScopeCommand(model.ScopeAllPages)
	command.MentionedPages = []model.MentionedPage{{
		Kind: "slide", SlideID: "sli_bbbbbb", Ordinal: 2, Title: "Two",
		SpecState: "ready", HTMLState: "available",
	}}
	pack, err := NewContextAssembler(store).Assemble(context.Background(), ContextRequest{
		RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: command,
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.RelatedSlides) != 1 || pack.RelatedSlides[0].ID != "sli_bbbbbb" {
		t.Fatalf("mentioned summary was not retained: %#v", pack.RelatedSlides)
	}
	if summary, ok := pack.SlideHTML.Summaries["sli_bbbbbb"]; !ok || len(summary.TextDigest) == 0 || summary.SourceHash == "" {
		t.Fatal("mentioned slide HTML summary is missing")
	}
}

func TestPPTContextKeepsThemeOutOfModelInput(t *testing.T) {
	project, store := fixture(t)
	pack, err := NewContextAssembler(store).Assemble(context.Background(), ContextRequest{
		RunID: "r1", ThreadID: "t1", ProjectID: "p1",
		Command: testScopeCommand(model.ScopeCurrentPage),
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Project.ThemeID != project.Theme {
		t.Fatal("runtime theme missing")
	}
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), project.Theme) || strings.Contains(string(raw), "theme_context") {
		t.Fatalf("selected theme leaked into serialized context: %s", raw)
	}
	projected, err := json.Marshal(ModelSections(pack))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(projected), project.Theme) || strings.Contains(string(projected), "theme_context") {
		t.Fatalf("selected theme leaked into model context: %s", projected)
	}
}

func TestContentChangeChangesPackHash(t *testing.T) {
	project, store := fixture(t)
	assembler := NewContextAssembler(store)
	req := ContextRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: testScopeCommand(model.ScopeCurrentPage)}
	before, err := assembler.Assemble(context.Background(), req, project)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project.WorkDir, ".spec.json")
	var entries map[string]pptspec.SlideSpec
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	slide := entries["sli_bbbbbb"]
	slide.Core += " changed"
	entries["sli_bbbbbb"] = slide
	writeJSON(t, path, entries)
	after, err := assembler.Assemble(context.Background(), req, project)
	if err != nil {
		t.Fatal(err)
	}
	if before.Manifest.PackHash == after.Manifest.PackHash {
		t.Fatal("content change did not change pack hash")
	}
}

func TestHTMLSummaryDeterministic(t *testing.T) {
	raw := []byte(`<html><head><title>X</title><style>.x{color:var(--ink)}</style></head><body><main id="m"><section data-kind="hero"><h1>Hello</h1><img src="x.png"></section></main><script>ok()</script></body></html>`)
	a, err := SummarizeHTML(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := SummarizeHTML(raw)
	if a.SourceHash != b.SourceHash || a.Title != "X" || len(a.Structure) < 2 || len(a.AssetRefs) != 1 || len(a.ScriptFeatures) != 2 {
		t.Fatalf("bad summary: %+v", a)
	}
}

func TestAssemblerLoadsEnabledRepositoryCatalogForEveryProfile(t *testing.T) {
	project, store := fixture(t)
	assembler := NewContextAssembler(store).
		WithComponentLoader(fakeComponentLoader{values: []model.Component{
			{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "feature-card", Name: "能力卡片", Tags: []model.ComponentTag{model.ComponentTagCard}},
			{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "disabled-component", Name: "停用组件", Disabled: true},
		}}).
		WithSkillLoader(fakeSkillLoader{values: []model.RepositorySkill{
			{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "story-architect", Name: "演示叙事架构", Tags: []model.SkillTag{model.SkillTagMethodology}},
			{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "disabled-skill", Name: "停用技能", Disabled: true},
		}})
	pack, err := assembler.Assemble(context.Background(), ContextRequest{
		RunID: "r1", ThreadID: "t1", ProjectID: project.ID,
		Command: testScopeCommand(model.ScopeAllPages),
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Components) != 1 || pack.Components[0].ID != "feature-card" {
		t.Fatalf("component catalog = %+v", pack.Components)
	}
	if len(pack.Skills) != 1 || pack.Skills[0].ID != "story-architect" {
		t.Fatalf("skill catalog = %+v", pack.Skills)
	}
}

func TestMissingTargetAndCorruptSourcesFail(t *testing.T) {
	project, store := fixture(t)
	bad := testScopeCommand(model.ScopeCurrentPage)
	bad.Scope.SlideIDs = []string{"sli_missing"}
	if _, err := NewContextAssembler(store).Assemble(context.Background(), ContextRequest{RunID: "r", ThreadID: "t", ProjectID: "p1", Command: bad}, project); err == nil {
		t.Fatal("missing target accepted")
	}
	if err := os.WriteFile(filepath.Join(project.WorkDir, ".outline.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewContextAssembler(store).Assemble(context.Background(), ContextRequest{RunID: "r", ThreadID: "t", ProjectID: "p1", Command: testScopeCommand(model.ScopeAllPages)}, project); !errors.Is(err, ErrRequiredMissing) {
		t.Fatalf("err=%v", err)
	}
}

func TestMissingHTMLIsDiagnosed(t *testing.T) {
	project, store := fixture(t)
	if err := os.Remove(filepath.Join(project.WorkDir, "sli_bbbbbb"+".html")); err != nil {
		t.Fatal(err)
	}
	pack, err := NewContextAssembler(store).Assemble(context.Background(), ContextRequest{
		RunID: "r", ThreadID: "t", ProjectID: "p1",
		Command: testScopeCommand(model.ScopeCurrentPage),
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Target.SlideHTMLSummary != nil {
		t.Fatal("missing HTML produced content")
	}
	found := false
	for _, warning := range pack.Manifest.Warnings {
		found = found || strings.Contains(warning, "HTML missing")
	}
	if !found {
		t.Fatalf("warnings=%v", pack.Manifest.Warnings)
	}
}
