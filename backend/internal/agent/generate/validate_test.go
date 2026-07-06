package generate

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

// --- Stage 4 crossPageLint 单元测试 ---

// 合规页（tokens 在 base 前、无字面字体）→ 无 issue。
func TestCrossPageLintPass(t *testing.T) {
	html := []byte(`<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage"><h1 class="slide-title">T</h1></section></div></body></html>`)
	if issues := crossPageLint(html, nil); len(issues) != 0 {
		t.Fatalf("compliant page should pass, got %v", issues)
	}
}

// 公共层顺序错误（base 在 tokens 前）→ 命中 common-layer-order（LintSlide 只查存在性，不查顺序）。
func TestCrossPageLintHitsCommonLayerOrder(t *testing.T) {
	html := []byte(`<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/base.css">` +
		`<link rel="stylesheet" href="../../common/tokens.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage"><h1>T</h1></section></div></body></html>`)
	issues := crossPageLint(html, nil)
	if !containsSubstr(issues, "common-layer-order") {
		t.Fatalf("want common-layer-order issue, got %v", issues)
	}
}

// 设计语言一致性：字面 font-family 不在 design_spec 声明集合 → 命中 design-font-consistency；
// 声明集合内的字体或 var(--token) → 放行。
func TestCrossPageLintFontConsistency(t *testing.T) {
	spec := &DesignSpec{Type: DesignType{
		Display: DesignFont{Family: "Space Grotesk"},
		Body:    DesignFont{Family: "Inter"},
	}}
	bad := []byte(`<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage" style="font-family: 'Comic Sans MS';"><h1>T</h1></section></div></body></html>`)
	if !containsSubstr(crossPageLint(bad, spec), "design-font-consistency") {
		t.Fatalf("literal off-spec font should be flagged, got %v", crossPageLint(bad, spec))
	}

	okDeclared := []byte(`<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage" style="font-family: 'Inter', sans-serif;"><h1>T</h1></section></div></body></html>`)
	if issues := crossPageLint(okDeclared, spec); len(issues) != 0 {
		t.Fatalf("declared font should pass, got %v", issues)
	}

	okToken := []byte(`<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage" style="font-family: var(--font-display);"><h1>T</h1></section></div></body></html>`)
	if issues := crossPageLint(okToken, spec); len(issues) != 0 {
		t.Fatalf("token-driven font should pass, got %v", issues)
	}
}

func containsSubstr(items []string, sub string) bool {
	for _, s := range items {
		if indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

// --- Stage 4/5 集成测试 ---

// wroteInThread 判断本子代理线程内是否已提交过 write_slide（每页子代理独立上下文）。
func wroteInThread(msgs []llm.Message) bool {
	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			if tc.Name == "write_slide" {
				return true
			}
		}
	}
	return false
}

// fixLoopClient：对 badPages 里的页始终产出「通过 LintSlide 但违反跨页顺序」的 html，
// 使 Stage 4 修复子循环无论几轮都无法修好；其余页产出合规 html。
type fixLoopClient struct {
	mu         sync.Mutex
	badPages   map[int]bool
	designDone bool
}

func (c *fixLoopClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *fixLoopClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func (c *fixLoopClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if hasTool(req.Tools, "submit_design_spec") {
		if !c.designDone {
			c.designDone = true
			return llm.ToolCallResponse{ToolCall: &llm.ToolCall{ID: "d1", Name: "submit_design_spec", Args: designSpecArgs()}}, nil
		}
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{ID: "d2", Name: "finish", Args: map[string]any{"summary": "design done"}}}, nil
	}

	if wroteInThread(req.Messages) {
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{ID: "f", Name: "finish", Args: map[string]any{"summary": "done"}}}, nil
	}
	idx := parseIdxFromMessages(req.Messages)
	html := goodOrderHTML(idx)
	if c.badPages[idx] {
		html = badOrderHTML(idx) // 通过 LintSlide，但 base 在 tokens 前 → 跨页顺序检查恒失败。
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{ID: "w", Name: "write_slide", Args: map[string]any{"slide_idx": idx, "html": html}}}, nil
}

func goodOrderHTML(idx int) string {
	return `<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage"><h1 class="slide-title">页 ` + itoa(idx) + `</h1></section></div></body></html>`
}

func badOrderHTML(idx int) string {
	return `<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/base.css">` +
		`<link rel="stylesheet" href="../../common/tokens.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage"><h1 class="slide-title">页 ` + itoa(idx) + `</h1></section></div></body></html>`
}

// AC-V2-PIPE-004：某页两次修复仍不合规 → 该页标 failed 并入 done.result.warnings，其余页正常交付。
func TestFixLoopBoundedWarnings(t *testing.T) {
	store, dir, slides := setupGen(t, 4)
	seq := 0
	r := NewRunner(&fixLoopClient{badPages: map[int]bool{1: true}}, store, Params{
		RunID: "r1", ProjectID: "p1", WorkDir: dir, WorkRoot: dir, Theme: "tokyo-night",
	}, func() int64 { return 1 }, func() string { seq++; return "v-" + itoa(seq) })

	em := &pageEmitter{}
	out := r.Run(context.Background(), em, nil, nil)
	// 修复超限 MUST NOT 阻塞整体 done（V2-STOP-002）。
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("bounded fix failure must still finish, got %s (%s)", out.Status, out.Message)
	}
	if out.Result == nil {
		t.Fatal("expected structured result")
	}
	warns, ok := out.Result["warnings"].([]Warning)
	if !ok || len(warns) != 1 {
		t.Fatalf("want exactly 1 warning, got %#v", out.Result["warnings"])
	}
	if warns[0].PageIndex != 1 || warns[0].Code != warnFixExceeded {
		t.Fatalf("warning should target page 1 with FIX_EXCEEDED, got %+v", warns[0])
	}
	// slide_count 反映全部页（含失败页仍交付其余）。
	if got := out.Result["slide_count"]; got != len(slides) {
		t.Fatalf("slide_count=%v, want %d", got, len(slides))
	}
	// 失败页 step 最终标记为 failed。
	if last := lastStatusFor(em.updates, pageStepID(1)); last != planStatusFailed {
		t.Fatalf("failed page step last status=%q, want failed", last)
	}
	// 其余页仍完成。
	for _, sl := range slides {
		if sl.Idx == 1 {
			continue
		}
		if last := lastStatusFor(em.updates, pageStepID(sl.Idx)); last != planStatusCompleted {
			t.Errorf("page %d last status=%q, want completed", sl.Idx, last)
		}
	}
}

// AC-V2-PIPE-001/§8.2：整套生成结构化 done.result 含 project_id/slide_count/design_spec_ref/signature/warnings。
func TestStructuredDoneResult(t *testing.T) {
	store, dir, slides := setupGen(t, 3)
	out := newGenRunner(store, dir, "tokyo-night", "gen1", nil).Run(context.Background(), &pageEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}
	res := out.Result
	if res == nil {
		t.Fatal("expected structured result, got nil")
	}
	if res["project_id"] != "p1" {
		t.Errorf("project_id=%v, want p1", res["project_id"])
	}
	if res["slide_count"] != len(slides) {
		t.Errorf("slide_count=%v, want %d", res["slide_count"], len(slides))
	}
	if res["design_spec_ref"] != "design/design-spec.json" {
		t.Errorf("design_spec_ref=%v", res["design_spec_ref"])
	}
	if sig, _ := res["signature"].(string); sig == "" {
		t.Error("signature must be non-empty")
	}
	if w, ok := res["warnings"].([]Warning); !ok || len(w) != 0 {
		t.Errorf("warnings should be empty slice, got %#v", res["warnings"])
	}
	// 指定主题时 theme 用主题名。
	if res["theme"] != "tokyo-night" {
		t.Errorf("theme=%v, want tokyo-night", res["theme"])
	}
}

// 未指定主题时 theme 标为 project-custom（tokens 由 design_spec 生成）。
func TestStructuredDoneResultCustomTheme(t *testing.T) {
	store, dir, _ := setupGen(t, 2)
	store.themes = nil // 无仓库主题 → 回退 seed，但用户未指定 Theme。
	out := newGenRunner(store, dir, "", "gen1", nil).Run(context.Background(), &pageEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}
	if out.Result["theme"] != "project-custom" {
		t.Errorf("theme=%v, want project-custom", out.Result["theme"])
	}
}

// V2-STOP-003：取消 → 各阶段安全终止，保留已落盘页与 design_spec。
func TestCancelPreservesArtifacts(t *testing.T) {
	store, dir, slides := setupGen(t, 3)
	// 先完整生成一次，落盘页与 design-spec.json。
	if out := newGenRunner(store, dir, "tokyo-night", "gen1", nil).Run(context.Background(), &pageEmitter{}, nil, nil); out.Status != harness.OutcomeFinished {
		t.Fatalf("initial gen failed: %s", out.Message)
	}
	before := hashTree(t, dir, slides)
	specPath := filepath.Join(dir, "design/design-spec.json")
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("design-spec.json should exist after gen: %v", err)
	}

	// 取消上下文再发起生成：应安全终止，已落盘产物保留。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	em := &pageEmitter{}
	out := newGenRunner(store, dir, "tokyo-night", "gen2", nil).Run(ctx, em, nil, nil)
	if out.Status != harness.OutcomeCanceled {
		t.Fatalf("canceled run should return canceled, got %s", out.Status)
	}
	// 已落盘页保持不变（未被 gen2 覆盖或删除）。
	after := hashTree(t, dir, slides)
	for p, h := range before {
		if after[p] != h {
			t.Errorf("artifact %s changed on canceled run (must be preserved)", p)
		}
	}
	// design-spec.json 保留。
	if _, err := os.Stat(specPath); err != nil {
		t.Errorf("design-spec.json must be preserved on cancel: %v", err)
	}
}

// lastStatusFor 返回某 step 的最后一次 plan.update 状态。
func lastStatusFor(updates []harness.PlanUpdatePayload, stepID string) string {
	last := ""
	for _, u := range updates {
		if u.StepID == stepID {
			last = u.Status
		}
	}
	return last
}
