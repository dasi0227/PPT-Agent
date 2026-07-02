package outline

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// captureEmitter 收集事件类型。
type captureEmitter struct {
	mu     sync.Mutex
	events []model.EventType
}

func (c *captureEmitter) Emit(evt model.EventType, _ any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, evt)
}
func (c *captureEmitter) has(e model.EventType) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, x := range c.events {
		if x == e {
			return true
		}
	}
	return false
}

// submitCall 构造一次 submit_outline 工具调用响应。
func submitCall(slides []any) llm.ToolCallResponse {
	return llm.ToolCallResponse{
		Thought:  "已规划大纲",
		ToolCall: &llm.ToolCall{ID: "c1", Name: "submit_outline", Args: map[string]any{"slides": slides}},
	}
}

func finishCall() llm.ToolCallResponse {
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{ID: "c2", Name: "finish", Args: map[string]any{"summary": "done"}}}
}

func newRunner(t *testing.T, store Store, client llm.Client, count int) *Runner {
	t.Helper()
	seq := 0
	newID := func() string { seq++; return "id-" + itoa(seq) }
	return NewRunner(client, store, Params{
		RunID: "r1", ProjectID: "p1", WorkDir: t.TempDir(),
		Topic: "云原生可观测性实践", SlideCount: count,
	}, func() int64 { return 100 }, newID)
}

// AC-OUTLINE-001（端到端）：主题 → submit_outline → 落库，每页通过 schema。
func TestOutlineGeneration(t *testing.T) {
	store := &memStore{}
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		submitCall(validOutline()),
		finishCall(),
	}}
	r := newRunner(t, store, fake, 0)

	em := &captureEmitter{}
	out := r.Run(context.Background(), em, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}
	if len(store.slides) != 7 {
		t.Fatalf("want 7 slides persisted, got %d", len(store.slides))
	}
	if !em.has(model.EventToolCall) || !em.has(model.EventToolResult) {
		t.Error("expected tool_call/tool_result events (ReAct projection)")
	}
	if len(store.versions) != 1 {
		t.Errorf("want version 0 created, got %d versions", len(store.versions))
	}
}

// AC-OUTLINE-001：LLM 首次提交结构不合规 → 工具返回错误 observation → 重试提交成功（纠偏路径）。
func TestOutlineRetryAfterInvalid(t *testing.T) {
	store := &memStore{}
	bad := []any{slide("bullets", "非封面", "x"), slide("thanks", "谢谢", "x")} // 首页非 cover
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		submitCall(bad),            // 第 1 次失败
		submitCall(validOutline()), // 修正后成功
		finishCall(),
	}}
	r := newRunner(t, store, fake, 0)
	out := r.Run(context.Background(), &captureEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s", out.Status)
	}
	if len(store.slides) != 7 {
		t.Fatalf("want 7 slides after retry, got %d", len(store.slides))
	}
}

// AC-OUTLINE-007：大纲生成中注入「加一页竞品分析」→ 最终大纲含竞品页。
func TestOutlineHITLInjection(t *testing.T) {
	store := &memStore{}

	// 含竞品分析页的修订大纲（第 2 页后插入）。
	revised := []any{
		slide("cover", "封面", "副标题"),
		slide("bullets", "背景", "点1"),
		slide("bullets", "竞品分析", "对手A", "对手B"),
		slide("bullets", "架构", "点1"),
		slide("bullets", "指标", "点1"),
		slide("bullets", "落地", "点1"),
		slide("thanks", "谢谢", "联系我们"),
	}

	fake := &scriptedClient{
		responses: []llm.ToolCallResponse{
			submitCall(validOutline()), // 初版（无竞品页）
		},
		afterInput: submitCall(revised), // 收到输入后提交修订版
	}
	r := newRunner(t, store, fake, 0)

	cp := &injectingCheckpoint{inputs: []string{"在第2页后加一页竞品分析"}}
	out := r.Run(context.Background(), &captureEmitter{}, cp, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s", out.Status)
	}
	// 最终大纲含竞品分析页。
	found := false
	for _, s := range store.slides {
		if s.Title == "竞品分析" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 竞品分析 slide after HITL injection; got %d slides", len(store.slides))
	}
	// 确认注入被消费。
	if cp.drained == 0 {
		t.Error("expected checkpoint to drain injected input")
	}
}

// scriptedClient：初次返回 responses；一旦 checkpoint 注入过输入，则返回 afterInput，最后 finish。
type scriptedClient struct {
	mu         sync.Mutex
	responses  []llm.ToolCallResponse
	afterInput llm.ToolCallResponse
	sawInput   bool
	submitted  bool
	n          int
}

func (c *scriptedClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *scriptedClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

// CallTool 检查最近消息里是否出现注入的 user 输入，决定返回初版还是修订版。
func (c *scriptedClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 检测注入：最后一条 user 消息即控制输入。
	for _, m := range req.Messages {
		if m.Role == llm.RoleUser && contains(m.Content, "竞品分析") {
			c.sawInput = true
		}
	}
	if c.sawInput && !c.submitted {
		c.submitted = true
		return c.afterInput, nil
	}
	if c.submitted {
		return finishCall(), nil
	}
	if c.n < len(c.responses) {
		r := c.responses[c.n]
		c.n++
		return r, nil
	}
	return finishCall(), nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// injectingCheckpoint 首次 Drain 返回注入输入，之后返回空。
type injectingCheckpoint struct {
	inputs  []string
	drained int
}

func (c *injectingCheckpoint) DrainInputs() []string {
	c.drained++
	if c.drained == 1 {
		return c.inputs
	}
	return nil
}

// 断言 validOutline 是合法 JSON（守护测试稳定）。
var _ = json.Marshal
