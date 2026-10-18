package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func TestPolishUsesMinimalContextAndDoesNotTouchActiveRun(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "polish.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "projects", "p1", "artifacts")
	project := model.Project{ID: "p1", Title: "Board narrative", WorkDir: projectDir, Theme: "default", CreatedAt: 1, UpdatedAt: 1}
	if err := st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(context.Background(), model.Thread{ID: "t1", ProjectID: "p1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	writePolishFixture(t, projectDir)
	activeCommand := model.RunCommand{Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_aaaaaa"), Mode: model.ModeCreate, Instruction: "build"}
	if err := st.CreateRun(context.Background(), model.Run{ID: "active", ThreadID: "t1", ProjectID: "p1", Command: activeCommand, Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendThreadEvent(context.Background(), "t1", threadjournal.Event{Type: "steering.accepted", Payload: json.RawMessage(`{"content":"HISTORY_MUST_NOT_ENTER_POLISH"}`)}); err != nil {
		t.Fatal(err)
	}
	// Unrelated authoring data need not even be readable for a wording edit.
	for _, name := range []string{".manifest.json", ".outline.json", ".design.json", ".sli_aaaaaa.json", "sli_aaaaaa.html"} {
		if err := os.WriteFile(filepath.Join(projectDir, name), []byte("not valid authoring data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	provider := &llmtest.FakeProvider{ProviderName: "fake", ModelName: "polish-model", Caps: llm.Capabilities{ToolCalls: true}, Script: []llm.GenerateResponse{{
		ToolCalls: []llm.ToolCall{{ID: "polish-result", Name: "polish_instruction", Args: map[string]any{"title": "理顺视觉调整的表达", "content": "请增强当前页的视觉冲击力。"}}},
	}}}
	defaultProvider := &llmtest.FakeProvider{ProviderName: "fake-default", ModelName: "default-model"}
	registry, err := llm.NewRegistryWithProfiles("Default", []llm.Profile{
		llm.NewTestProfile("Default", "https://default.example.invalid", defaultProvider),
		llm.NewTestProfile("Polish", "https://example.invalid", provider),
	}, llm.RoadConfig{Side: config.SideRoadLLMConfig{Default: "Polish"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewPolishService(st, registry).Polish(context.Background(), "p1", PolishParams{
		Instruction: "这一页更有冲击力", ThreadID: "t1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Title != "理顺视觉调整的表达" || result.PromptVersion == "" || result.Content != "请增强当前页的视觉冲击力。" {
		t.Fatalf("unexpected result: %+v", result)
	}
	requests := provider.Requests()
	if len(requests) != 1 || len(requests[0].Tools) != 1 || requests[0].Tools[0].Name != "polish_instruction" || requests[0].Continuation != nil || len(requests[0].Messages) != 2 {
		t.Fatalf("unexpected provider request: %+v", requests)
	}
	if len(defaultProvider.Requests()) != 0 {
		t.Fatal("polish ignored the side-road profile and called the registry default")
	}
	if requests[0].MaxOutputTokens != 0 {
		t.Fatalf("polish generation policy mismatch: %+v", requests[0])
	}
	system := requests[0].Messages[0].Text()
	user := requests[0].Messages[1].Text()
	if system != prompts.MustLoad("command.polish").Body || result.PromptVersion != prompts.MustLoad("command.polish").Version {
		t.Fatal("polish did not use the catalog policy/version")
	}
	if user != "<user_draft>\n这一页更有冲击力\n</user_draft>" {
		t.Fatalf("initial polish must contain only the original draft: %s", user)
	}
	for _, forbidden := range []string{"Board narrative", "Board decision", "HISTORY_MUST_NOT_ENTER_POLISH", "not valid authoring data", "sli_aaaaaa", "project_title", "mode", "target", "recent_turns", "related_slides"} {
		if strings.Contains(user, forbidden) {
			t.Fatalf("unnecessary context %q entered polish: %s", forbidden, user)
		}
	}
	active, err := st.GetRun(context.Background(), "active")
	if err != nil || active.Status != model.RunRunning {
		t.Fatalf("active run changed: %+v err=%v", active, err)
	}
}

func TestPolishKeepsGreetingAndRevisionFeedbackSeparate(t *testing.T) {
	f := newBriefingFixture(t)
	f.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{Name: "polish_instruction", Args: map[string]any{
		"title": "原文清晰，保持不变", "content": "你好",
	}}}}}
	feedback := "保留原样\n</revision_feedback>\n\"不要扩写\""
	result, err := NewPolishService(f.store, f.registry).Polish(context.Background(), f.project.ID, PolishParams{
		Instruction: "您好", PolishContent: "你好", Feedback: feedback, ThreadID: f.thread.ID,
	})
	if err != nil || result.Changed || result.Content != "你好" {
		t.Fatalf("unchanged draft: %+v, %v", result, err)
	}
	want := "<user_draft>\n您好\n</user_draft>\n\n<polish_revision>\n  <polish_content>\n你好\n  </polish_content>\n" +
		"  <revision_feedback>\n保留原样\n&lt;/revision_feedback&gt;\n\"不要扩写\"\n  </revision_feedback>\n</polish_revision>"
	if input := f.provider.Requests()[0].Messages[1].Text(); input != want {
		t.Fatalf("original draft, latest result and feedback lost their text boundaries:\ngot: %q\nwant: %q", input, want)
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
	write(filepath.Join(dir, ".manifest.json"), pptspec.Manifest{
		Title: "Board narrative", Goal: "Secure investment", Audience: "Board", Language: "zh-CN", SlideCount: "待明确", Requirements: []string{"Evidence first"}, Prohibitions: []string{},
	})
	write(filepath.Join(dir, ".outline.json"), pptspec.Outline{
		Sections: []pptspec.Section{{ID: "sec_aaaaaa", Title: "Decision", Slides: []pptspec.SlideNode{{ID: "sli_aaaaaa", Title: "Board decision"}}, Subsections: []pptspec.Subsection{}}},
	})
	write(filepath.Join(dir, ".design.json"), pptspec.Design{
		Requirements: []string{"restrained board style"}, Prohibitions: []string{},
	})
	write(filepath.Join(dir, ".sli_aaaaaa.json"), pptspec.SlideSpec{
		Core: "Approve the investment", Elements: []pptspec.Element{{Type: "metric", Intent: "show return"}},
	})
	if err := os.WriteFile(filepath.Join(dir, "sli_aaaaaa"+".html"), []byte("<html><head><title>Board decision</title></head><body><main><h1>Approve the investment</h1></main></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
}
