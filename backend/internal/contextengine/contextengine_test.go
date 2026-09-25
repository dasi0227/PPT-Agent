package contextengine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
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

func testAssembler(store ContextStore, registry *RefRegistry) *ContextAssembler {
	return NewContextAssembler(store, registry)
}

func fixture(t *testing.T) (model.Project, *fakeStore) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	deck := pptspec.Manifest{Title: "Deck", Goal: "goal", Audience: "leaders", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}}
	writeJSON(t, filepath.Join(dir, "manifest.json"), deck)
	outline := pptspec.Outline{Sections: []pptspec.Section{{ID: "sec_aaaaaa", Title: "Section", Purpose: "Test section", Slides: []pptspec.SlideNode{}, Subsections: []pptspec.Subsection{{ID: "sub_aaaaaa", Title: "Sub", Purpose: "Test subsection", Slides: []pptspec.SlideNode{{SlideID: "sli_aaaaaa", Title: "One", Role: "evidence"}, {SlideID: "sli_bbbbbb", Title: "Two", Role: "evidence"}, {SlideID: "sli_cccccc", Title: "Three", Role: "evidence"}}}}}}}
	writeJSON(t, filepath.Join(dir, "outline.json"), outline)
	design := pptspec.Design{
		Direction:         "test direction",
		LayoutPreferences: []string{"Prefer open grids"},
		Decorations:       pptspec.Decorations{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "none", KeyMessage: "none"},
	}
	writeJSON(t, filepath.Join(dir, "design.json"), design)
	slides := map[string]model.Slide{}
	for _, loc := range pptspec.FlattenOutline(outline) {
		id := loc.Slide.SlideID
		bp := pptspec.SlideSpec{
			KeyMessage: "Message " + id,
			Elements: []pptspec.Element{
				{Type: "chart", Intent: "Show growth"},
				{Type: "asset", Intent: "growth chart"},
			},
			Layout: "two-column",
		}
		writeJSON(t, filepath.Join(dir, "slides", id, "spec.json"), bp)
		html := `<!doctype html><html><head><title>` + id + `</title><style>:root{--color:red}</style></head><body><main id="slide" data-slide="` + id + `"><section class="hero token-accent"><h1>` + loc.Slide.Title + `</h1><img src="asset.png" alt="asset"></section></main></body></html>`
		if err := os.WriteFile(filepath.Join(dir, "slides", id, "index.html"), []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
		slides[id] = model.Slide{ID: id, ProjectID: "p1"}
	}
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

func spec(selection model.ScopeSelectionKind) model.RunCommand {
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
	assembler := testAssembler(store, nil)
	cases := []struct {
		level   model.ScopeSelectionKind
		profile ProfileID
	}{
		{model.ScopeAllPages, ProfilePPTDeck}, {model.ScopeCurrentPage, ProfilePPTSlide},
	}
	for _, tc := range cases {
		t.Run(string(tc.profile), func(t *testing.T) {
			req := ContextRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: spec(tc.level), Budget: DefaultBudget()}
			pack, err := assembler.Assemble(context.Background(), req, project)
			if err != nil {
				t.Fatal(err)
			}
			if pack.Profile != tc.profile {
				t.Fatalf("profile=%s", pack.Profile)
			}
			if tc.level == model.ScopeAllPages && pack.Target.SlideHTML != "" {
				t.Fatal("deck target received full HTML")
			}
			if tc.level == model.ScopeCurrentPage {
				if pack.Target.SlideSpec == nil || len(pack.Target.SlideIDs) != 1 || pack.Target.SlideIDs[0] != "sli_bbbbbb" || pack.Target.SlideSpec.KeyMessage != "Message sli_bbbbbb" {
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

func TestMentionedPagesKeepSummarySegmentAndHTMLRefUnderTightBudget(t *testing.T) {
	project, store := fixture(t)
	command := spec(model.ScopeAllPages)
	command.MentionedPages = []model.MentionedPage{{
		Kind: "slide", SlideID: "sli_bbbbbb", Ordinal: 2, Title: "Two",
		SpecState: "ready", HTMLState: "available",
	}}
	budget := DefaultBudget()
	budget.InputLimit = 1
	budget.SegmentCaps[SegmentRelated] = 1
	pack, err := testAssembler(store, nil).Assemble(context.Background(), ContextRequest{
		RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: command, Budget: budget,
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.RelatedSlides) != 1 || pack.RelatedSlides[0].ID != "sli_bbbbbb" {
		t.Fatalf("mentioned summary was not retained: %#v", pack.RelatedSlides)
	}
	foundRequiredSegment := false
	for _, segment := range pack.Manifest.Segments {
		if segment.Kind == SegmentRelated && segment.Required && segment.Priority == 100 {
			foundRequiredSegment = true
		}
	}
	if !foundRequiredSegment {
		t.Fatal("mentioned summary segment was not marked required")
	}
	foundRef := false
	for _, ref := range pack.Manifest.Refs {
		if ref.Kind == RefSlideHTML && ref.TargetID == "sli_bbbbbb" {
			foundRef = true
		}
	}
	if !foundRef {
		t.Fatal("mentioned slide HTML ContextRef is unavailable")
	}
}

func TestPPTContextKeepsThemeOutOfModelInput(t *testing.T) {
	project, store := fixture(t)
	pack, err := testAssembler(store, nil).Assemble(context.Background(), ContextRequest{
		RunID: "r1", ThreadID: "t1", ProjectID: "p1",
		Command: spec(model.ScopeCurrentPage), Budget: DefaultBudget(),
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
	compiled, err := (PromptCompiler{}).Compile(pack, "SYSTEM")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(compiled.User, project.Theme) || strings.Contains(compiled.User, "theme_context") {
		t.Fatalf("selected theme leaked into model context: %s", compiled.User)
	}
}

func TestContentChangeChangesPackHash(t *testing.T) {
	project, store := fixture(t)
	assembler := testAssembler(store, nil)
	req := ContextRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: spec(model.ScopeCurrentPage), Budget: DefaultBudget()}
	before, err := assembler.Assemble(context.Background(), req, project)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project.WorkDir, "slides", "sli_bbbbbb", "spec.json")
	var slide pptspec.SlideSpec
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &slide); err != nil {
		t.Fatal(err)
	}
	slide.KeyMessage += " changed"
	writeJSON(t, path, slide)
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

func TestLargeHTMLDowngradesToRefAndRefIsRunBound(t *testing.T) {
	project, store := fixture(t)
	path := filepath.Join(project.WorkDir, "slides", "sli_bbbbbb", "index.html")
	if err := os.WriteFile(path, []byte(`<html><body><main><p>`+strings.Repeat("large ", 20000)+`</p></main></body></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := NewRefRegistry()
	assembler := testAssembler(store, registry)
	budget := DefaultBudget()
	budget.InputLimit = 5000
	budget.ContextWindow = 9000
	budget.OutputReserve = 4000
	pack, err := assembler.Assemble(context.Background(), ContextRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: spec(model.ScopeCurrentPage), Budget: budget}, project)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Target.SlideHTML != "" || pack.Target.SlideHTMLRef == nil {
		t.Fatal("large HTML was not downgraded")
	}
	resolver := ContextRefResolver{Registry: registry}
	ref := pack.Target.SlideHTMLRef
	if _, err := resolver.Read(context.Background(), RefReadRequest{RunID: "other", ThreadID: "t1", ProjectID: "p1", RefID: ref.ID, Detail: DetailFull, RemainingBudget: 100000}); code(err) != CodeRefForbidden {
		t.Fatalf("cross-run err=%v", err)
	}
	if _, err := resolver.Read(context.Background(), RefReadRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", RefID: ref.ID, Detail: DetailFull, RemainingBudget: 1}); code(err) != CodeBudgetExceeded {
		t.Fatalf("budget err=%v", err)
	}
}

func TestBudgetDropsOptionalSegmentsBeforeRequiredTarget(t *testing.T) {
	project, store := fixture(t)
	budget := DefaultBudget()
	budget.InputLimit, budget.ContextWindow, budget.OutputReserve = 500, 1000, 500
	for kind := range budget.SegmentCaps {
		budget.SegmentCaps[kind] = 100000
	}
	pack, err := testAssembler(store, nil).Assemble(context.Background(), ContextRequest{
		RunID: "r", ThreadID: "t", ProjectID: "p1",
		Command: spec(model.ScopeCurrentPage), Budget: budget,
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Target.SlideSpec == nil || pack.Command.Instruction == "" {
		t.Fatal("required target or RunCommand was cropped")
	}
	if len(pack.Manifest.Dropped) == 0 || len(pack.Manifest.Warnings) == 0 {
		t.Fatalf("manifest lacks budget diagnosis: %+v", pack.Manifest)
	}
	raw, err := json.Marshal(pack)
	if err != nil || !json.Valid(raw) {
		t.Fatal("budgeting produced invalid JSON")
	}
}

func TestRefStaleAfterHTMLContentChange(t *testing.T) {
	project, store := fixture(t)
	registry := NewRefRegistry()
	pack, err := testAssembler(store, registry).Assemble(context.Background(), ContextRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", Command: spec(model.ScopeCurrentPage), Budget: DefaultBudget()}, project)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project.WorkDir, model.SlideHTMLPath("sli_bbbbbb")), []byte("<html><body>Changed</body></html>"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = (&ContextRefResolver{Registry: registry}).Read(context.Background(), RefReadRequest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", RefID: pack.Target.SlideHTMLRef.ID, Detail: DetailFull, RemainingBudget: 100000})
	if code(err) != CodeRefStale {
		t.Fatalf("err=%v", err)
	}
}

func TestPromptCompilerSnapshotSeparatesUserInstruction(t *testing.T) {
	p := ContextPack{SchemaVersion: SchemaVersion, Command: spec(model.ScopeAllPages), Project: ProjectContext{ID: "p1"}}
	got, err := (PromptCompiler{}).Compile(p, "SYSTEM")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.User, "<user_instruction>\n\"improve target\"\n</user_instruction>") || strings.Contains(got.System, "<user_instruction>") {
		t.Fatalf("%+v", got)
	}
	if strings.Contains(got.System, "improve target") {
		t.Fatal("user instruction leaked into system layer")
	}
	if got.System != "SYSTEM" {
		t.Fatalf("system prompt contains dynamic context: %q", got.System)
	}
	if !strings.Contains(got.User, "untrusted source data") || !strings.Contains(got.User, "<run_command>") {
		t.Fatal("stable partitions missing")
	}
	for _, forbidden := range []string{`"id":"p1"`, `"project_id"`, `"revision"`, "available_context_refs"} {
		if strings.Contains(got.User, forbidden) {
			t.Fatalf("model projection leaked %s: %s", forbidden, got.User)
		}
	}
}

func TestPromptCompilerIncludesExactRepositoryResourceCatalog(t *testing.T) {
	p := ContextPack{
		SchemaVersion: SchemaVersion,
		Command:       spec(model.ScopeAllPages),
		Project:       ProjectContext{ID: "p1"},
		Components: []ComponentCandidate{{
			ID: "feature-card", Name: "能力卡片", Description: "聚焦一项能力", Tags: []string{"card"},
		}},
		Skills: []SkillCandidate{{
			ID: "story-architect", Name: "演示叙事架构", Description: "组织演示叙事", Tags: []string{"methodology"},
		}},
	}
	got, err := (PromptCompiler{}).Compile(p, "SYSTEM")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"<available_resources>", `"id":"feature-card"`, `"id":"story-architect"`,
	} {
		if !strings.Contains(got.User, expected) {
			t.Fatalf("compiled resource catalog missing %q: %s", expected, got.User)
		}
	}
}

func TestAssemblerLoadsEnabledRepositoryCatalogForEveryProfile(t *testing.T) {
	project, store := fixture(t)
	assembler := testAssembler(store, NewRefRegistry()).
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
		Command: spec(model.ScopeAllPages),
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

func TestPolishContextIsTargetAwareBoundedAndHasNoRuntimeRefs(t *testing.T) {
	project, store := fixture(t)
	if err := NewFSTranscriptStore().Replace(project.WorkDir, "t1", []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("保持整体克制")},
	}); err != nil {
		t.Fatal(err)
	}
	pack, err := NewContextAssembler(store, NewRefRegistry()).AssemblePolish(context.Background(), PolishContextRequest{
		ThreadID: "t1", Command: spec(model.ScopeCurrentPage),
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Target.Spec == nil || pack.Target.SlideID != "sli_bbbbbb" || pack.Target.Spec.KeyMessage != "Message sli_bbbbbb" || pack.Target.HTMLTitle != "sli_bbbbbb" {
		t.Fatalf("target context missing: %+v", pack.Target)
	}
	if len(pack.RecentTurns) != 1 {
		t.Fatalf("thread context missing: turns=%+v", pack.RecentTurns)
	}
	if pack.EstimatedTokens > PolishContextTokenBudget {
		t.Fatalf("polish context exceeded budget: %d", pack.EstimatedTokens)
	}
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"context_id", "available_context_refs", "slide_html_ref", "run_id"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("polish context leaked runtime field %q: %s", forbidden, raw)
		}
	}
	compiled, err := CompilePolishContext(pack)
	if err != nil || !strings.Contains(compiled, "untrusted reference data") || strings.Contains(compiled, commandInstruction(pack)) {
		t.Fatalf("compiled polish context mismatch: %v %s", err, compiled)
	}
}

func commandInstruction(PolishContext) string { return "improve target" }

func TestCompileForRunnerKeepsRuntimeStateOutOfSystemPrompt(t *testing.T) {
	p := ContextPack{SchemaVersion: SchemaVersion, Command: spec(model.ScopeCurrentPage), Project: ProjectContext{ID: "p1"}}
	state := `{"requirements":[{"id":"req-1","text":"keep this dynamic"}],"approved_plan":{"title":"user-approved"}}`
	system, user := CompileForRunner(&p, "STATIC SYSTEM", state)
	if system != "STATIC SYSTEM" {
		t.Fatalf("dynamic content entered system prompt: %q", system)
	}
	for _, expected := range []string{"keep this dynamic", "user-approved", "<runtime_state>"} {
		if !strings.Contains(user, expected) {
			t.Fatalf("user runtime input missing %q: %s", expected, user)
		}
	}
}

func TestMissingTargetAndCorruptSourcesFail(t *testing.T) {
	project, store := fixture(t)
	bad := spec(model.ScopeCurrentPage)
	bad.Scope.SlideIDs = []string{"sli_missing"}
	if _, err := testAssembler(store, nil).Assemble(context.Background(), ContextRequest{RunID: "r", ThreadID: "t", ProjectID: "p1", Command: bad, Budget: DefaultBudget()}, project); err == nil {
		t.Fatal("missing target accepted")
	}
	if err := os.WriteFile(filepath.Join(project.WorkDir, "outline.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := testAssembler(store, nil).Assemble(context.Background(), ContextRequest{RunID: "r", ThreadID: "t", ProjectID: "p1", Command: spec(model.ScopeAllPages), Budget: DefaultBudget()}, project); !errors.Is(err, ErrRequiredMissing) {
		t.Fatalf("err=%v", err)
	}
}

func TestMissingHTMLIsDiagnosed(t *testing.T) {
	project, store := fixture(t)
	if err := os.Remove(filepath.Join(project.WorkDir, "slides", "sli_bbbbbb", "index.html")); err != nil {
		t.Fatal(err)
	}
	pack, err := testAssembler(store, nil).Assemble(context.Background(), ContextRequest{
		RunID: "r", ThreadID: "t", ProjectID: "p1",
		Command: spec(model.ScopeCurrentPage), Budget: DefaultBudget(),
	}, project)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Target.SlideHTML != "" || pack.Target.SlideHTMLRef != nil {
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

func code(err error) string {
	var re *RefError
	if errors.As(err, &re) {
		return re.Code
	}
	return ""
}
