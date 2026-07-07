package assist

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ── test doubles ──────────────────────────────────────

// captureEmitter 记录发出的事件类型与 info 文本。
type captureEmitter struct {
	events    []model.EventType
	infoTexts []string
	artifacts int
}

func (e *captureEmitter) Emit(evt model.EventType, payload any) {
	e.events = append(e.events, evt)
	switch p := payload.(type) {
	case harness.InfoPayload:
		e.infoTexts = append(e.infoTexts, p.Text)
	case harness.ArtifactPayload:
		e.artifacts++
	}
}
func (e *captureEmitter) has(evt model.EventType) bool {
	for _, x := range e.events {
		if x == evt {
			return true
		}
	}
	return false
}

// chatClient 返回固定文本补全。
type chatClient struct{ reply string }

func (c chatClient) Chat(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Content: c.reply}, nil
}
func (c chatClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c chatClient) CallTool(context.Context, llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	return llm.ToolCallResponse{}, nil
}

// textCallToolClient：CallTool 只返回纯文本（talk 模式：LLM 输出分析文本）。
type textCallToolClient struct{ text string }

func (c textCallToolClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c textCallToolClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c textCallToolClient) CallTool(context.Context, llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	return llm.ToolCallResponse{Text: c.text}, nil
}

type recapStore struct {
	proj     model.Project
	slides   []model.Slide
	versions map[string][]model.Version
}

func (s *recapStore) GetProject(_ context.Context, _ string) (model.Project, error) {
	return s.proj, nil
}
func (s *recapStore) ListSlides(_ context.Context, _ string) ([]model.Slide, error) {
	return s.slides, nil
}
func (s *recapStore) ListVersions(_ context.Context, tt, tid string) ([]model.Version, error) {
	return s.versions[tt+"|"+tid], nil
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// AC-CMD-PROMPT-001：/prompt 仅返回改写文本（info），无 artifact，work_dir 无变更。
func TestPromptRewriteNoSideEffect(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644)
	before := countFiles(t, dir)

	em := &captureEmitter{}
	r := NewPromptRunner(chatClient{reply: "将本页改为两栏布局，主标题加粗、配图右置。"}, "r1", "我想让这页好看点")
	out := r.Run(context.Background(), em, nil, nil)

	if out.Status != harness.OutcomeFinished {
		t.Fatalf("expected finished, got %s", out.Status)
	}
	if !em.has(model.EventInfo) || len(em.infoTexts) == 0 {
		t.Fatal("expected info event with rewritten text")
	}
	if !strings.Contains(em.infoTexts[0], "两栏布局") {
		t.Errorf("rewritten text unexpected: %q", em.infoTexts[0])
	}
	if em.artifacts != 0 {
		t.Error("prompt MUST NOT emit artifacts")
	}
	if em.has(model.EventArtifact) {
		t.Error("prompt MUST NOT emit artifact events")
	}
	if countFiles(t, dir) != before {
		t.Error("prompt MUST NOT change work_dir files")
	}
}

// AC-CMD-RECAP-001：/recap 输出结构化摘要（主题/页数/各页/最近变更），无文件变更。
func TestRecapReadOnly(t *testing.T) {
	dir := t.TempDir()
	before := countFiles(t, dir)

	store := &recapStore{
		proj: model.Project{ID: "p1", Title: "云原生实践", Theme: "tokyo-night", Status: "ready"},
		slides: []model.Slide{
			{ID: "s0", Idx: 0, Layout: "cover", Title: "封面", CurrentVersion: 0},
			{ID: "s1", Idx: 1, Layout: "bullets", Title: "目录", CurrentVersion: 2},
		},
		versions: map[string][]model.Version{
			"slide|" + model.SlideVersionTarget("p1", "s1"): {{VersionNo: 2, CreatedAt: 100, RunID: "run12345678"}},
			"design|" + model.DesignVersionTarget("p1"):     {{VersionNo: 1, CreatedAt: 90, RunID: "run90000000"}},
		},
	}
	em := &captureEmitter{}
	r := NewRecapRunner(store, "r1", "p1")
	out := r.Run(context.Background(), em, nil, nil)

	if out.Status != harness.OutcomeFinished {
		t.Fatalf("expected finished, got %s", out.Status)
	}
	if len(em.infoTexts) == 0 {
		t.Fatal("expected info summary")
	}
	txt := em.infoTexts[0]
	for _, want := range []string{"云原生实践", "tokyo-night", "封面", "bullets", "第1页 v2", "公共层 v1"} {
		if !strings.Contains(txt, want) {
			t.Errorf("recap summary missing %q:\n%s", want, txt)
		}
	}
	if em.artifacts != 0 || em.has(model.EventArtifact) {
		t.Error("recap MUST NOT emit artifacts")
	}
	if countFiles(t, dir) != before {
		t.Error("recap MUST NOT change files")
	}
}

// AC-CMD-TALK-001：/talk 只输出分析（info），无 artifact，无文件变更。
func TestTalkNoArtifact(t *testing.T) {
	dir := t.TempDir()
	before := countFiles(t, dir)

	em := &captureEmitter{}
	r := NewTalkRunner(textCallToolClient{text: "深色方案取舍：对比度需≥4.5，动效建议用 fade-in..."}, "r1", "我想把整体改成深色并加动效")
	out := r.Run(context.Background(), em, nil, nil)

	if out.Status != harness.OutcomeFinished {
		t.Fatalf("expected finished, got %s", out.Status)
	}
	if !em.has(model.EventInfo) {
		t.Fatal("expected info (analysis) event")
	}
	if em.artifacts != 0 || em.has(model.EventArtifact) {
		t.Error("talk MUST NOT emit artifacts")
	}
	if countFiles(t, dir) != before {
		t.Error("talk MUST NOT change files")
	}
}
