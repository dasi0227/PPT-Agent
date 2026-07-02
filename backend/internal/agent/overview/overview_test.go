package overview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ── test doubles ──────────────────────────────────────

type memStore struct {
	versions    []model.Version
	slideVer    map[int]int
	nextVerByTg map[string]int
	failCreate  bool
}

func newMemStore() *memStore {
	return &memStore{slideVer: map[int]int{}, nextVerByTg: map[string]int{}}
}
func (m *memStore) NextVersionNo(_ context.Context, tt, tid string) (int, error) {
	return m.nextVerByTg[tt+"|"+tid], nil
}
func (m *memStore) CreateVersion(_ context.Context, v model.Version) error {
	if m.failCreate {
		return os.ErrPermission
	}
	m.versions = append(m.versions, v)
	m.nextVerByTg[v.TargetType+"|"+v.TargetID] = v.VersionNo + 1
	return nil
}
func (m *memStore) DeleteVersion(_ context.Context, tt, tid string, no int) error {
	for i, v := range m.versions {
		if v.TargetType == tt && v.TargetID == tid && v.VersionNo == no {
			m.versions = append(m.versions[:i], m.versions[i+1:]...)
			break
		}
	}
	return nil
}
func (m *memStore) SetSlideVersion(_ context.Context, _ string, idx, no int) error {
	m.slideVer[idx] = no
	return nil
}
func (m *memStore) ListAssets(_ context.Context, _ string) ([]model.Asset, error) {
	return nil, nil
}
func (m *memStore) GetAsset(_ context.Context, _ string) (model.Asset, error) {
	return model.Asset{}, os.ErrNotExist
}

// scriptClient 按调用序返回预设 tool call；用于驱动主循环/子代理。
type scriptClient struct {
	calls []llm.ToolCall
	i     int
}

func (c *scriptClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *scriptClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c *scriptClient) CallTool(context.Context, llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	if c.i >= len(c.calls) {
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "done"}}}, nil
	}
	tc := c.calls[c.i]
	c.i++
	return llm.ToolCallResponse{ToolCall: &tc}, nil
}

// pageSubAgentClient：给 fanout 的子代理用——每次都发一个 patch_slide 把「原标题」→「新标题」，
// 子代理下一轮会 finish（脚本用尽后自动 finish）。因为每个子代理是新的 Loop，用共享脚本会串号，
// 故用「按 slide_idx 从消息里解析」的自适应 client。
type fanoutSubClient struct{}

func (fanoutSubClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (fanoutSubClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (fanoutSubClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	// 若历史里已有 tool observation（说明已 patch 过），则 finish。
	for _, m := range req.Messages {
		if m.Role == llm.RoleTool {
			return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}}, nil
		}
	}
	// 从 user 消息解析 slide_idx（EditUser 里含 "slide_idx=N"）。
	idx := 0
	for _, m := range req.Messages {
		if m.Role == llm.RoleUser {
			if p := strings.Index(m.Content, "slide_idx="); p >= 0 {
				rest := m.Content[p+len("slide_idx="):]
				n := 0
				got := false
				for j := 0; j < len(rest) && rest[j] >= '0' && rest[j] <= '9'; j++ {
					n = n*10 + int(rest[j]-'0')
					got = true
				}
				if got {
					idx = n
				}
			}
		}
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{
		Name: "patch_slide",
		Args: map[string]any{
			"slide_idx": idx,
			"edits":     []any{map[string]any{"old_text": "原标题", "new_text": "新标题"}},
		},
	}}, nil
}

type nopEmitter struct{}

func (nopEmitter) Emit(model.EventType, any) {}

// ── helpers ──────────────────────────────────────

const validSlide = `<!doctype html><html><head>` +
	`<link rel="stylesheet" href="../../common/tokens.css">` +
	`<link rel="stylesheet" href="../../common/base.css"></head>` +
	`<body><div class="slide-scaler"><section class="slide-stage">` +
	`<h1 class="slide-title">原标题</h1></section></div></body></html>`

const tokensCSS = `:root{
  --color-primary: #ff0000;
  --font-size-base: 16px;
}`

func setupProject(t *testing.T, pages int) string {
	t.Helper()
	dir := t.TempDir()
	writeAt(t, dir, "common/tokens.css", tokensCSS)
	writeAt(t, dir, "common/base.css", "/* base */")
	for i := 0; i < pages; i++ {
		writeAt(t, dir, "slides/"+pad(i)+"/index.html", validSlide)
	}
	return dir
}

func writeAt(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hashTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		data, _ := os.ReadFile(p)
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func pad(n int) string {
	s := itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func seqID() func() string {
	i := 0
	return func() string { i++; return "v-" + itoa(i) }
}

func mustSandbox(t *testing.T, dir string) *tools.Sandbox {
	t.Helper()
	sb, err := tools.NewSandbox(dir)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	return sb
}

// AC-CMD-OVERVIEW-001 / AC-EDIT-004：patch_design 改主色 → 仅 tokens.css 变，页 html hash 全不变。
func TestOverviewPatchDesignOnlyCommonLayer(t *testing.T) {
	dir := setupProject(t, 8)
	before := hashTree(t, dir)

	client := &scriptClient{calls: []llm.ToolCall{
		{Name: "patch_design", Args: map[string]any{
			"edits": []any{map[string]any{"old_text": "#ff0000", "new_text": "#0047ff"}},
		}},
		{Name: "finish", Args: map[string]any{"summary": "主色改蓝"}},
	}}
	r := NewRunner(client, newMemStore(), Params{
		RunID: "r1", ProjectID: "p1", WorkDir: dir, PageCount: 8, Instruction: "主色改成品牌蓝",
	}, func() int64 { return 1 }, seqID())

	out := r.Run(context.Background(), nopEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("expected finished, got %s: %s", out.Status, out.Message)
	}

	after := hashTree(t, dir)
	// 8 页 html hash 不变。
	for i := 0; i < 8; i++ {
		key := "slides/" + pad(i) + "/index.html"
		if before[key] != after[key] {
			t.Errorf("page %d html hash changed (must not): %s", i, key)
		}
	}
	// tokens.css 变了。
	if before["common/tokens.css"] == after["common/tokens.css"] {
		t.Error("common/tokens.css must change")
	}
	// 内容确实变蓝。
	raw, _ := os.ReadFile(filepath.Join(dir, "common/tokens.css"))
	if !strings.Contains(string(raw), "#0047ff") || strings.Contains(string(raw), "#ff0000") {
		t.Errorf("tokens not updated: %s", raw)
	}
}

// AC-CMD-OVERVIEW-003：fanout「每页加页脚」→ 子代理逐页 patch，8 页各产新版本，公共层不变。
func TestOverviewFanoutSubAgentPerPage(t *testing.T) {
	dir := setupProject(t, 8)
	beforeTokens := hashTree(t, dir)["common/tokens.css"]
	store := newMemStore()

	// 直接测 fanout 工具（用自适应子 client），精确断言逐页子代理行为。
	sb := mustSandbox(t, dir)
	fan := NewFanoutPagePatchTool(fanoutSubClient{}, store, sb, "p1", "r1", 8, func() int64 { return 1 }, seqID())
	res, err := fan.Execute(context.Background(), map[string]any{"instruction": "把标题从原标题改为新标题"})
	if err != nil {
		t.Fatalf("fanout: %v", err)
	}
	if !res.OK {
		t.Fatalf("fanout failed: %s", res.Observation)
	}

	// 8 页各产一个 slide 版本。
	slideVersions := 0
	for _, v := range store.versions {
		if v.TargetType == "slide" {
			slideVersions++
		}
	}
	if slideVersions != 8 {
		t.Errorf("expected 8 slide versions (one per page), got %d", slideVersions)
	}
	// 8 页 html 都改了。
	for i := 0; i < 8; i++ {
		raw, _ := os.ReadFile(filepath.Join(dir, "slides/"+pad(i)+"/index.html"))
		if !strings.Contains(string(raw), "新标题") {
			t.Errorf("page %d not patched by sub-agent", i)
		}
	}
	// 公共层不变（fanout 不碰 tokens）。
	afterTokens := hashTree(t, dir)["common/tokens.css"]
	if beforeTokens != afterTokens {
		t.Error("common/tokens.css must not change during fanout")
	}
}

// SPEC-CMD-OVERVIEW-006：某页无锚点 → 该页失败，其它页正常落盘（失败隔离）。
func TestOverviewFanoutPerPageFailureIsolation(t *testing.T) {
	dir := setupProject(t, 3)
	// 第 1 页替换掉「原标题」为无法匹配的内容，使子代理锚点失败。
	writeAt(t, dir, "slides/001/index.html", strings.Replace(validSlide, "原标题", "特殊标题", 1))
	store := newMemStore()
	sb := mustSandbox(t, dir)

	fan := NewFanoutPagePatchTool(fanoutSubClient{}, store, sb, "p1", "r1", 3, func() int64 { return 1 }, seqID())
	res, _ := fan.Execute(context.Background(), map[string]any{"instruction": "原标题→新标题"})

	// 第 0、2 页成功，第 1 页失败。
	if !strings.Contains(res.Observation, "失败 1 页") {
		t.Errorf("expected 1 failed page in observation: %s", res.Observation)
	}
	for _, i := range []int{0, 2} {
		raw, _ := os.ReadFile(filepath.Join(dir, "slides/"+pad(i)+"/index.html"))
		if !strings.Contains(string(raw), "新标题") {
			t.Errorf("page %d should have succeeded", i)
		}
	}
	// 第 1 页未变（仍是「特殊标题」，无「新标题」）。
	raw1, _ := os.ReadFile(filepath.Join(dir, "slides/001/index.html"))
	if strings.Contains(string(raw1), "新标题") {
		t.Error("page 1 must remain unchanged on its own failure")
	}
	// 只产 2 个 slide 版本。
	slideVersions := 0
	for _, v := range store.versions {
		if v.TargetType == "slide" {
			slideVersions++
		}
	}
	if slideVersions != 2 {
		t.Errorf("expected 2 slide versions (failed page produces none), got %d", slideVersions)
	}
}

// patch_design 产 design 版本（SPEC-CMD-OVERVIEW-004）。
func TestPatchDesignProducesDesignVersion(t *testing.T) {
	dir := setupProject(t, 2)
	store := newMemStore()
	sb := mustSandbox(t, dir)
	tool := NewPatchDesignTool(store, sb, "p1", "r1", func() int64 { return 1 }, seqID())
	res, err := tool.Execute(context.Background(), map[string]any{
		"edits": []any{map[string]any{"old_text": "#ff0000", "new_text": "#0047ff"}},
	})
	if err != nil || !res.OK {
		t.Fatalf("patch_design: %v %s", err, res.Observation)
	}
	if len(store.versions) != 1 || store.versions[0].TargetType != "design" {
		t.Errorf("expected 1 design version, got %+v", store.versions)
	}
}

func TestPatchDesignRestoresTokensWhenVersionCreateFails(t *testing.T) {
	dir := setupProject(t, 1)
	store := newMemStore()
	store.failCreate = true
	sb := mustSandbox(t, dir)
	before, _ := os.ReadFile(filepath.Join(dir, "common/tokens.css"))
	tool := NewPatchDesignTool(store, sb, "p1", "r1", func() int64 { return 1 }, seqID())

	res, err := tool.Execute(context.Background(), map[string]any{
		"edits": []any{map[string]any{"old_text": "#ff0000", "new_text": "#0047ff"}},
	})
	if err == nil {
		t.Fatal("expected CreateVersion error")
	}
	if res.OK {
		t.Fatalf("result must not be OK on DB failure: %+v", res)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "common/tokens.css"))
	if string(after) != string(before) {
		t.Fatalf("tokens.css must be restored on DB failure:\n%s", after)
	}
}
