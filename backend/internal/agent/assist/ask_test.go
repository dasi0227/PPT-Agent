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

// ── ask test doubles ──────────────────────────────────

// askClient：澄清阶段按 decideAsk 决定 ask_user 或 proceed；执行阶段（edit）走 patch_slide→finish。
type askClient struct {
	decideAsk bool
	patched   bool
}

func (c *askClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *askClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c *askClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	// 澄清阶段：工具集含 ask_user/proceed。
	if hasSchema(req.Tools, "ask_user") {
		if c.decideAsk {
			return llm.ToolCallResponse{ToolCall: &llm.ToolCall{
				Name: "ask_user",
				Args: map[string]any{"question": "用什么数据？", "choices": []any{"示例数据", "我来提供"}},
			}}, nil
		}
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "proceed", Args: map[string]any{}}}, nil
	}
	// 执行阶段（edit 主循环）：先 patch_slide，再 finish。
	if !c.patched {
		c.patched = true
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{
			Name: "patch_slide",
			Args: map[string]any{"slide_idx": 0, "edits": []any{map[string]any{"old_text": "原标题", "new_text": "居中标题"}}},
		}}, nil
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "done"}}}, nil
}

func hasSchema(ts []llm.ToolSchema, name string) bool {
	for _, t := range ts {
		if t.Name == name {
			return true
		}
	}
	return false
}

// recordPrompter 记录 needs_input 调用并返回预设应答。
type recordPrompter struct {
	called   bool
	question string
	choices  []string
	answer   string
}

func (p *recordPrompter) NeedsInput(_ context.Context, _ string, question string, choices []string) (string, error) {
	p.called = true
	p.question = question
	p.choices = choices
	return p.answer, nil
}

// askMemStore 实现 edit.Store。
type askMemStore struct {
	versions []model.Version
	next     map[string]int
}

func newAskMemStore() *askMemStore { return &askMemStore{next: map[string]int{}} }
func (m *askMemStore) NextVersionNo(_ context.Context, tt, tid string) (int, error) {
	return m.next[tt+"|"+tid], nil
}
func (m *askMemStore) CreateVersion(_ context.Context, v model.Version) error {
	m.versions = append(m.versions, v)
	m.next[v.TargetType+"|"+v.TargetID] = v.VersionNo + 1
	return nil
}
func (m *askMemStore) SetSlideVersion(context.Context, string, int, int) error { return nil }

const askSlide = `<!doctype html><html><head>` +
	`<link rel="stylesheet" href="../../common/tokens.css">` +
	`<link rel="stylesheet" href="../../common/base.css"></head>` +
	`<body><div class="slide-scaler"><section class="slide-stage">` +
	`<h1 class="slide-title">原标题</h1></section></div></body></html>`

func setupAskPage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, "slides", "000", "index.html")
	os.MkdirAll(filepath.Dir(full), 0o755)
	os.WriteFile(full, []byte(askSlide), 0o644)
	return dir
}

// AC-CMD-ASK-001：有不确定点 → 发 needs_input（带候选）并等待，应答后继续执行。
func TestAskQuestionsWhenUncertain(t *testing.T) {
	dir := setupAskPage(t)
	client := &askClient{decideAsk: true}
	prompter := &recordPrompter{answer: "示例数据"}

	r := NewAskRunner(client, newAskMemStore(), AskParams{
		RunID: "r1", ProjectID: "p1", WorkDir: dir, Scope: model.ScopeCurrent, PageIndex: 0,
		Instruction: "帮我把这页做成图表",
	}, func() int64 { return 1 }, func() string { return "v" })

	out := r.Run(context.Background(), &captureEmitter{}, nil, prompter)
	if !prompter.called {
		t.Fatal("expected needs_input to be raised (AC-CMD-ASK-001)")
	}
	if len(prompter.choices) == 0 {
		t.Error("ask SHOULD provide choices")
	}
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("expected finished after answer, got %s: %s", out.Status, out.Message)
	}
}

// AC-CMD-ASK-002：无歧义 → 不提问直接执行完成。
func TestAskProceedsWhenClear(t *testing.T) {
	dir := setupAskPage(t)
	client := &askClient{decideAsk: false}
	prompter := &recordPrompter{answer: "unused"}

	r := NewAskRunner(client, newAskMemStore(), AskParams{
		RunID: "r1", ProjectID: "p1", WorkDir: dir, Scope: model.ScopeCurrent, PageIndex: 0,
		Instruction: "把封面标题居中",
	}, func() int64 { return 1 }, func() string { return "v" })

	out := r.Run(context.Background(), &captureEmitter{}, nil, prompter)
	if prompter.called {
		t.Error("no-ambiguity case MUST NOT raise needs_input (AC-CMD-ASK-002)")
	}
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("expected finished, got %s: %s", out.Status, out.Message)
	}
	// 确认真的执行了编辑。
	raw, _ := os.ReadFile(filepath.Join(dir, "slides/000/index.html"))
	if !strings.Contains(string(raw), "居中标题") {
		t.Error("ask (clear) should have executed the edit")
	}
}
