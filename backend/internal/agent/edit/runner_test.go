package edit

import (
	"context"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// fakeOverviewTool 模拟一个 overview scope 的越权写工具（改公共层）。
type fakeOverviewTool struct{}

func (fakeOverviewTool) Name() string               { return "patch_design" }
func (fakeOverviewTool) Description() string        { return "改公共层（越权）" }
func (fakeOverviewTool) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (fakeOverviewTool) Class() tools.Class         { return tools.ClassWrite }
func (fakeOverviewTool) Scopes() []model.Scope      { return []model.Scope{model.ScopeOverview} }
func (fakeOverviewTool) Execute(context.Context, map[string]any) (tools.Result, error) {
	return tools.Result{}, nil
}

// AC-HARNESS-001：page scope 下，越权工具（overview 的 patch_design）MUST NOT 进入注册 schema。
func TestToolGatingExcludesCrossScope(t *testing.T) {
	all := []tools.Tool{
		NewReadSlideTool(nil, 3),
		NewPatchSlideTool(nil, nil, "p1", "r1", 3, nil, nil),
		NewValidateSlideTool(),
		tools.NewFinishTool(),
		fakeOverviewTool{}, // 越权
	}
	loop := harness.New(&llmtestNop{}, harness.Config{
		RunID: "r1", Kind: model.KindEdit, Scope: model.ScopePage, Mode: model.ModeNormal,
		Tools: all,
	})
	names := map[string]bool{}
	for _, tl := range loop.GatedTools() {
		names[tl.Name()] = true
	}
	if names["patch_design"] {
		t.Error("cross-scope tool patch_design MUST NOT be registered under page scope")
	}
	for _, want := range []string{"read_slide", "patch_slide", "validate_slide", "finish"} {
		if !names[want] {
			t.Errorf("expected tool %q registered under page scope", want)
		}
	}
}

// current scope 同样只注册单页工具（与 page 一致）。
func TestToolGatingCurrentScope(t *testing.T) {
	all := []tools.Tool{
		NewPatchSlideTool(nil, nil, "p1", "r1", 0, nil, nil),
		fakeOverviewTool{},
		tools.NewFinishTool(),
	}
	loop := harness.New(&llmtestNop{}, harness.Config{
		RunID: "r1", Kind: model.KindEdit, Scope: model.ScopeCurrent, Mode: model.ModeNormal,
		Tools: all,
	})
	for _, tl := range loop.GatedTools() {
		if tl.Name() == "patch_design" {
			t.Error("patch_design must not be registered under current scope")
		}
	}
}

// AGENT-CTX-001：编辑第 3 页时，装配的上下文（EditUser）不含别页 html。
func TestContextScopeIsolation(t *testing.T) {
	// 目标页 html 含哨兵；别页内容不应出现在上下文里（runner 只读目标页）。
	target := 3
	dir := t.TempDir()
	writePage(t, dir, target, `<section class="slide-stage">TARGET-PAGE-sentinel</section>`)
	writePage(t, dir, 0, `<section class="slide-stage">OTHER-PAGE-sentinel</section>`)
	writePage(t, dir, 4, `<section class="slide-stage">ANOTHER-sentinel</section>`)

	// 用 runner 装配上下文：捕获发给 LLM 的消息。
	capture := &captureClient{}
	r := NewRunner(capture, newMemStore(), Params{
		RunID: "r1", ProjectID: "p1", WorkDir: dir, Scope: model.ScopePage,
		PageIndex: target, Instruction: "把标题改大",
	}, func() int64 { return 1 }, func() string { return "v" })
	r.Run(context.Background(), nopEmitter{}, nil, nil)

	joined := strings.Join(capture.messages, "\n")
	if !strings.Contains(joined, "TARGET-PAGE-sentinel") {
		t.Error("context MUST include target page html")
	}
	if strings.Contains(joined, "OTHER-PAGE-sentinel") || strings.Contains(joined, "ANOTHER-sentinel") {
		t.Error("context MUST NOT include other pages' html (AGENT-CTX-001)")
	}
}

// ── test doubles ──────────────────────────────────────

type llmtestNop struct{}

func (llmtestNop) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (llmtestNop) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (llmtestNop) CallTool(context.Context, llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}}, nil
}

// captureClient 记录第一次 CallTool 的全部消息内容，然后 finish。
type captureClient struct {
	messages []string
	called   bool
}

func (c *captureClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *captureClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c *captureClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	if !c.called {
		c.called = true
		for _, m := range req.Messages {
			c.messages = append(c.messages, m.Content)
		}
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}}, nil
}

type nopEmitter struct{}

func (nopEmitter) Emit(model.EventType, any) {}

func writePage(t *testing.T, dir string, idx int, html string) {
	t.Helper()
	full := `<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head><body>` +
		`<div class="slide-scaler">` + html + `</div></body></html>`
	writeFileAt(t, dir, "slides/"+padIdx(idx)+"/index.html", full)
}
