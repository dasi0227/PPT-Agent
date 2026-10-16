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

type fakeComponentLoader struct{ values []model.Component }

func (l fakeComponentLoader) LoadComponents(context.Context) ([]model.Component, error) {
	return l.values, nil
}

type fakeSkillLoader struct{ values []model.RepositorySkill }

func (l fakeSkillLoader) LoadSkills(context.Context) ([]model.RepositorySkill, error) {
	return l.values, nil
}

func fixture(t *testing.T) model.Project {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	deck := pptspec.Manifest{Title: "Deck", Goal: "goal", Audience: "leaders", Language: "zh-CN", Pages: "待明确", Requirements: []string{}, Prohibitions: []string{}}
	writeJSON(t, filepath.Join(dir, ".manifest.json"), deck)
	outline := pptspec.Outline{Sections: []pptspec.Section{{ID: "sec_aaaaaa", Title: "Section", Slides: []pptspec.SlideNode{}, Subsections: []pptspec.Subsection{{ID: "sub_aaaaaa", Title: "Sub", Slides: []pptspec.SlideNode{{ID: "sli_aaaaaa", Title: "One"}, {ID: "sli_bbbbbb", Title: "Two"}, {ID: "sli_cccccc", Title: "Three"}}}}}}}
	writeJSON(t, filepath.Join(dir, ".outline.json"), outline)
	design := pptspec.Design{
		Requirements: []string{"test direction", "Prefer open grids"},
		Prohibitions: []string{},
	}
	writeJSON(t, filepath.Join(dir, ".design.json"), design)
	for _, loc := range pptspec.FlattenOutline(outline) {
		id := loc.Slide.ID
		bp := pptspec.SlideSpec{
			Purpose: "content", Approach: "explanation",
			Core: "Message " + id,
			Elements: []pptspec.Element{
				{Type: "chart", Intent: "Show growth"},
				{Type: "asset", Intent: "growth chart"},
			},
			Layout: "two-column",
		}
		writeJSON(t, filepath.Join(dir, model.SlideSpecPath(id)), bp)
		html := `<!doctype html><html><head><title>` + id + `</title><style>:root{--color:red}</style></head><body><main id="slide" data-slide="` + id + `"><section class="hero token-accent"><h1>` + loc.Slide.Title + `</h1><img src="asset.png" alt="asset"></section></main></body></html>`
		if err := os.WriteFile(filepath.Join(dir, id+".html"), []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return model.Project{ID: "p1", Title: "Deck", Theme: "swiss-modern", WorkDir: dir}
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

func TestPageProfiles(t *testing.T) {
	project := fixture(t)
	assembler := NewContextAssembler()
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
				if summary.Purpose != "content" || summary.Approach != "explanation" {
					t.Fatalf("page summary did not read Spec purpose and approach: %+v", summary)
				}
			}
			if pack.Project.ThemeID != project.Theme {
				t.Fatal("runtime project theme was not retained")
			}

		})
	}
}

func TestPPTContextKeepsThemeOutOfModelInput(t *testing.T) {
	project := fixture(t)
	pack, err := NewContextAssembler().Assemble(context.Background(), ContextRequest{
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

}

func TestAssemblerLoadsEnabledRepositoryCatalogForEveryProfile(t *testing.T) {
	project := fixture(t)
	assembler := NewContextAssembler().
		WithComponentLoader(fakeComponentLoader{values: []model.Component{
			{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "feature-card", Name: "能力卡片", Tags: []model.ComponentTag{"card"}},
			{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "disabled-component", Name: "停用组件", Disabled: true},
		}}).
		WithSkillLoader(fakeSkillLoader{values: []model.RepositorySkill{
			{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "story-architect", Name: "演示叙事架构", Tags: []model.SkillTag{"methodology"}},
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
	project := fixture(t)
	bad := testScopeCommand(model.ScopeCurrentPage)
	bad.Scope.SlideIDs = []string{"sli_missing"}
	if _, err := NewContextAssembler().Assemble(context.Background(), ContextRequest{RunID: "r", ThreadID: "t", ProjectID: "p1", Command: bad}, project); err == nil {
		t.Fatal("missing target accepted")
	}
	if err := os.WriteFile(filepath.Join(project.WorkDir, model.SlideSpecPath("sli_aaaaaa")), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewContextAssembler().Assemble(context.Background(), ContextRequest{RunID: "r", ThreadID: "t", ProjectID: "p1", Command: testScopeCommand(model.ScopeCurrentPage)}, project); err != nil {
		t.Fatalf("another page's invalid Spec blocked context: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project.WorkDir, ".outline.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewContextAssembler().Assemble(context.Background(), ContextRequest{RunID: "r", ThreadID: "t", ProjectID: "p1", Command: testScopeCommand(model.ScopeAllPages)}, project); !errors.Is(err, ErrRequiredMissing) {
		t.Fatalf("err=%v", err)
	}
}

func TestMissingHTMLIsReportedInPageState(t *testing.T) {
	project := fixture(t)
	if err := os.Remove(filepath.Join(project.WorkDir, "sli_bbbbbb"+".html")); err != nil {
		t.Fatal(err)
	}
	pack, err := NewContextAssembler().Assemble(context.Background(), ContextRequest{
		RunID: "r", ThreadID: "t", ProjectID: "p1",
		Command: testScopeCommand(model.ScopeCurrentPage),
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range pack.Outline.Summaries {
		if summary.ID == "sli_bbbbbb" {
			if summary.State != string(model.HTMLMissing) {
				t.Fatalf("missing HTML state=%s", summary.State)
			}
			return
		}
	}
	t.Fatal("missing page state")
}
