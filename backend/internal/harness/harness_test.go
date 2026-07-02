package harness

import (
	"context"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// captureEmitter 收集发出的事件供断言。
type captureEmitter struct {
	mu     sync.Mutex
	events []model.EventType
}

func (c *captureEmitter) Emit(evt model.EventType, _ any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, evt)
}

func (c *captureEmitter) has(evt model.EventType) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e == evt {
			return true
		}
	}
	return false
}

func (c *captureEmitter) count(evt model.EventType) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, e := range c.events {
		if e == evt {
			n++
		}
	}
	return n
}

// scopedTool 是测试用的带 scope 元数据工具。
type scopedTool struct {
	name   string
	class  tools.Class
	scopes []model.Scope
}

func (s scopedTool) Name() string               { return s.name }
func (s scopedTool) Description() string        { return s.name }
func (s scopedTool) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (s scopedTool) Class() tools.Class         { return s.class }
func (s scopedTool) Scopes() []model.Scope      { return s.scopes }
func (s scopedTool) Execute(context.Context, map[string]any) (tools.Result, error) {
	return tools.Result{OK: true, Observation: "ok"}, nil
}

func finishTool() tools.Tool { return tools.NewFinishTool() }

// AC-HARNESS-002 + ReAct：一次工具化循环——tool_call → tool_result → finish。
func TestReActLoop(t *testing.T) {
	fake := &llmtest.FakeClient{
		Script: []llm.ToolCallResponse{
			{Thought: "patch it", ToolCall: &llm.ToolCall{Name: "demo", Args: map[string]any{}}},
			{Thought: "done", ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}},
		},
	}
	demo := scopedTool{name: "demo", class: tools.ClassWrite, scopes: []model.Scope{model.ScopeCurrent}}
	loop := New(fake, Config{
		RunID: "r1", Kind: model.KindEdit, Scope: model.ScopeCurrent, Mode: model.ModeNormal,
		Tools: []tools.Tool{demo, finishTool()},
	})
	em := &captureEmitter{}
	out := loop.Run(context.Background(), em, nil)

	if out.Status != OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}
	for _, want := range []model.EventType{model.EventRunStarted, model.EventThought, model.EventToolCall, model.EventToolResult} {
		if !em.has(want) {
			t.Errorf("missing event %s", want)
		}
	}
}

// AC-HARNESS-001：/page scope 下，改公共层/别页的工具 MUST NOT 出现在门控后的集合。
func TestDynamicToolGating(t *testing.T) {
	pageTool := scopedTool{name: "patch_slide", class: tools.ClassWrite, scopes: []model.Scope{model.ScopeCurrent, model.ScopePage, model.ScopeOverview}}
	designTool := scopedTool{name: "patch_design", class: tools.ClassWrite, scopes: []model.Scope{model.ScopeOverview}}
	repoTool := scopedTool{name: "create_asset", class: tools.ClassWrite, scopes: []model.Scope{model.ScopeRepo}}
	all := []tools.Tool{pageTool, designTool, repoTool, finishTool()}

	gated := Gate(all, model.ScopePage, model.ModeNormal)
	names := map[string]bool{}
	for _, tl := range gated {
		names[tl.Name()] = true
	}
	if !names["patch_slide"] || !names["finish"] {
		t.Error("page scope must include patch_slide + finish")
	}
	if names["patch_design"] {
		t.Error("page scope MUST NOT expose patch_design (overview-only)")
	}
	if names["create_asset"] {
		t.Error("page scope MUST NOT expose create_asset (repo-only)")
	}
}

// AC-RUN-006 / talk：talk 模式只留控制工具，不注册任何读写产物工具。
func TestTalkModeGatesOutWriteTools(t *testing.T) {
	writeTool := scopedTool{name: "patch_slide", class: tools.ClassWrite, scopes: []model.Scope{model.ScopeCurrent}}
	readTool := scopedTool{name: "read_slide", class: tools.ClassRead, scopes: []model.Scope{model.ScopeCurrent}}
	gated := Gate([]tools.Tool{writeTool, readTool, finishTool()}, model.ScopeCurrent, model.ModeTalk)
	if len(gated) != 1 || gated[0].Name() != "finish" {
		t.Errorf("talk mode must expose only finish, got %d tools", len(gated))
	}
}

// AC-HARNESS-004：不调用 finish 的异常循环达到 MAX_TURNS → error(MAX_TURNS_EXCEEDED)。
func TestStopConditionsMaxTurns(t *testing.T) {
	// 脚本耗尽后 ExhaustedResponse 恒返回非 finish 工具调用，制造无限循环。
	fake := &llmtest.FakeClient{
		ExhaustedResponse: llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "demo", Args: map[string]any{}}},
	}
	demo := scopedTool{name: "demo", class: tools.ClassWrite, scopes: nil}
	loop := New(fake, Config{
		RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, MaxTurns: 5,
		Tools: []tools.Tool{demo, finishTool()},
	})
	em := &captureEmitter{}
	out := loop.Run(context.Background(), em, nil)
	if out.Status != OutcomeMaxTurns || out.Code != CodeMaxTurns {
		t.Fatalf("want max_turns, got %s/%s", out.Status, out.Code)
	}
	if out.Turns != 5 {
		t.Errorf("want 5 turns, got %d", out.Turns)
	}
}

// AC-HARNESS-004：finish 工具显式退出。
func TestStopConditionsFinish(t *testing.T) {
	fake := &llmtest.FakeClient{
		Script: []llm.ToolCallResponse{
			{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "bye"}}},
		},
	}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, Tools: []tools.Tool{finishTool()}})
	out := loop.Run(context.Background(), &captureEmitter{}, nil)
	if out.Status != OutcomeFinished || out.Summary != "bye" {
		t.Fatalf("want finished/bye, got %s/%q", out.Status, out.Summary)
	}
}

// ARCH-HARNESS-STOP-003：连续工具失败 → 熔断退出。
func TestStopConditionsCircuitBreaker(t *testing.T) {
	fake := &llmtest.FakeClient{
		ExhaustedResponse: llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "flaky", Args: map[string]any{}}},
	}
	flaky := failingTool{}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, MaxTurns: 20, Tools: []tools.Tool{flaky, finishTool()}})
	out := loop.Run(context.Background(), &captureEmitter{}, nil)
	if out.Status != OutcomeCircuit || out.Code != CodeCircuit {
		t.Fatalf("want circuit, got %s/%s", out.Status, out.Code)
	}
}

type failingTool struct{}

func (failingTool) Name() string               { return "flaky" }
func (failingTool) Description() string        { return "always fails" }
func (failingTool) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (failingTool) Class() tools.Class         { return tools.ClassWrite }
func (failingTool) Scopes() []model.Scope      { return nil }
func (failingTool) Execute(context.Context, map[string]any) (tools.Result, error) {
	return tools.Result{OK: false, Observation: "boom"}, nil
}

// ARCH-LLM-FC-001：CallTool 返回错误 → 最小失败退出（不死循环）。
func TestStopConditionsLLMError(t *testing.T) {
	fake := &llmtest.FakeClient{CallToolErr: llm.ErrBadToolCall}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, Tools: []tools.Tool{finishTool()}})
	out := loop.Run(context.Background(), &captureEmitter{}, nil)
	if out.Status != OutcomeLLMError {
		t.Fatalf("want llm_error, got %s", out.Status)
	}
	if fake.CallToolCount() != 1 {
		t.Errorf("must not retry on bad tool call, calls=%d", fake.CallToolCount())
	}
}

// ARCH-HARNESS-STOP-004：ctx 取消 → 安全终止。
func TestStopConditionsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &llmtest.FakeClient{ExhaustedResponse: llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "x"}}}}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, Tools: []tools.Tool{finishTool()}})
	out := loop.Run(ctx, &captureEmitter{}, nil)
	if out.Status != OutcomeCanceled {
		t.Fatalf("want canceled, got %s", out.Status)
	}
}

// AC-RUN-002：checkpoint 排空控制输入并纳入后续上下文。
func TestCheckpointDrainsInputs(t *testing.T) {
	fake := &llmtest.FakeClient{
		Script: []llm.ToolCallResponse{
			{ToolCall: &llm.ToolCall{Name: "demo", Args: map[string]any{}}},
			{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}},
		},
	}
	demo := scopedTool{name: "demo", class: tools.ClassWrite, scopes: nil}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, Tools: []tools.Tool{demo, finishTool()}})
	cp := &fakeCheckpoint{inputs: [][]string{{"用深色主题"}, nil}}
	out := loop.Run(context.Background(), &captureEmitter{}, cp)
	if out.Status != OutcomeFinished {
		t.Fatalf("want finished, got %s", out.Status)
	}
	if cp.drainCalls == 0 {
		t.Error("expected checkpoint drain to be called")
	}
}

type fakeCheckpoint struct {
	inputs     [][]string
	drainCalls int
}

func (f *fakeCheckpoint) DrainInputs() []string {
	var out []string
	if f.drainCalls < len(f.inputs) {
		out = f.inputs[f.drainCalls]
	}
	f.drainCalls++
	return out
}
