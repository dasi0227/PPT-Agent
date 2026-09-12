package service

import (
	"context"
	"encoding/json"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestPolishUsesAuthoritativeContextAndDoesNotTouchActiveRun(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "polish.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "p1")
	project := model.Project{ID: "p1", Title: "Board narrative", WorkDir: projectDir, Theme: "default", Status: "draft", CreatedAt: 1, UpdatedAt: 1}
	if err := st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(context.Background(), model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	writePolishFixture(t, projectDir)
	activeCommand := model.RunCommand{Scope: model.NewRunScope(model.ScopeObjectPresentation, model.ScopeCurrentPage, "sli_aaaaaa"), Mode: model.ModeExecute, Instruction: "build"}
	if err := st.CreateRun(context.Background(), model.Run{ID: "active", ThreadID: "t1", ProjectID: "p1", Command: activeCommand, Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
	provider := &llmtest.FakeProvider{ProviderName: "fake", ModelName: "polish-model", Script: []llm.GenerateResponse{{
		Content: llm.TextContent("请强化当前页面的核心结论与视觉层级，同时保持董事会叙事的克制风格。"),
	}}}
	defaultProvider := &llmtest.FakeProvider{ProviderName: "fake-default", ModelName: "default-model"}
	registry, err := llm.NewRegistryWithProfiles("Default", []llm.Profile{
		llm.NewTestProfile("Default", "https://default.example.invalid", defaultProvider),
		llm.NewTestProfile("Polish", "https://example.invalid", provider),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewPolishService(st, registry).Polish(context.Background(), "p1", PolishParams{
		Instruction: "这一页更有冲击力", ThreadID: "t1",
		ScopeInput: model.CreateRunScopeInput{Object: model.ScopeObjectPresentation, Selection: model.ScopeSelectionInput{
			Kind: model.ScopeCurrentPage, CurrentSlideID: "sli_aaaaaa",
		}},
		Mode: model.ModeExecute, Model: "Polish",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.PromptVersion == "" || !strings.Contains(result.Instruction, "核心结论") {
		t.Fatalf("unexpected result: %+v", result)
	}
	requests := provider.Requests()
	if len(requests) != 1 || len(requests[0].Tools) != 0 || requests[0].Continuation != nil || len(requests[0].Messages) != 2 {
		t.Fatalf("unexpected provider request: %+v", requests)
	}
	if len(defaultProvider.Requests()) != 0 {
		t.Fatal("polish ignored the selected profile and called the registry default")
	}
	if requests[0].Reasoning != llm.ReasoningDisabled || requests[0].MaxOutputTokens != maxPolishOutputTokens {
		t.Fatalf("polish generation policy mismatch: %+v", requests[0])
	}
	system := requests[0].Messages[0].Text()
	user := requests[0].Messages[1].Text()
	if system != prompts.MustLoad("command.polish").Body || result.PromptVersion != prompts.Version {
		t.Fatal("polish did not use the catalog policy/version")
	}
	for _, value := range []string{"Board narrative", "Board decision", "这一页更有冲击力"} {
		if strings.Contains(system, value) || !strings.Contains(user, value) {
			t.Fatalf("dynamic value crossed prompt layers: %s", value)
		}
	}
	active, err := st.GetRun(context.Background(), "active")
	if err != nil || active.Status != model.RunRunning {
		t.Fatalf("active run changed: %+v err=%v", active, err)
	}
}

func writePolishFixture(t *testing.T, dir string) {
	t.Helper()
	write := func(path string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "manifest.json"), pptspec.Manifest{
		SchemaVersion: pptspec.SchemaVersion, Revision: 1, ProjectID: "p1", Title: "Board narrative", Goal: "Secure investment", Audience: "Board", Language: "zh-CN", Requirements: []string{"Evidence first"}, Prohibitions: []string{}, Canvas: pptspec.CanvasSettings{AspectRatio: "16:9"}, Numbering: pptspec.NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1,
	})
	write(filepath.Join(dir, "outline.json"), pptspec.Outline{
		SchemaVersion: pptspec.SchemaVersion, Revision: 1, ProjectID: "p1",
		Sections: []pptspec.Section{{ID: "sec_aaaaaa", Title: "Decision", Purpose: "Decision support", Slides: []pptspec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Board decision", Role: "conclusion"}}, Subsections: []pptspec.Subsection{}}}, CreatedAt: 1, UpdatedAt: 1,
	})
	write(filepath.Join(dir, "design.json"), pptspec.Design{
		SchemaVersion: pptspec.SchemaVersion, Revision: 1, ProjectID: "p1", Theme: "editorial", Direction: "restrained board style", Density: "medium", Chrome: []pptspec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1,
	})
	write(filepath.Join(dir, "slides", "sli_aaaaaa", "spec.json"), pptspec.SlideSpec{
		SchemaVersion: pptspec.SchemaVersion, Revision: 1, ProjectID: "p1", SlideID: "sli_aaaaaa", KeyMessage: "Approve the investment", Elements: []pptspec.Element{{Type: "metric", Intent: "show return"}}, CreatedAt: 1, UpdatedAt: 1,
	})
	if err := os.WriteFile(filepath.Join(dir, "slides", "sli_aaaaaa", "index.html"), []byte("<html><head><title>Board decision</title></head><body><main><h1>Approve the investment</h1></main></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
}
