package generate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// pageEmitter 记录 page progress 事件与错误。
type pageEmitter struct {
	mu       sync.Mutex
	pageMsgs []harness.ProgressPayload
	errored  bool
}

func (e *pageEmitter) Emit(evt model.EventType, payload any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch evt {
	case model.EventProgress:
		if p, ok := payload.(harness.ProgressPayload); ok && p.Stage == "page" {
			e.pageMsgs = append(e.pageMsgs, p)
		}
	case model.EventError:
		e.errored = true
	}
}

// writeSlideClient 是每页返回一次 write_slide(合规 html) + finish 的 fake LLM。
// marker 注入 html（作为 data 属性），用于区分不同轮次的产物（验证单页重生成 hash 变化）。
type writeSlideClient struct {
	mu     sync.Mutex
	seen   map[string]bool // 已为某 idx 提交过 write_slide
	marker string
}

func newWriteSlideClient(marker string) *writeSlideClient {
	return &writeSlideClient{seen: map[string]bool{}, marker: marker}
}

func (c *writeSlideClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *writeSlideClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

// CallTool：从 user 指令解析出 slide_idx（"slide_idx=N"），首次返回 write_slide，之后 finish。
func (c *writeSlideClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := parseIdxFromMessages(req.Messages)
	key := itoa(idx)
	if !c.seen[key] {
		c.seen[key] = true
		html := fmt.Sprintf(`<!doctype html><html><head>`+
			`<link rel="stylesheet" href="../../common/tokens.css">`+
			`<link rel="stylesheet" href="../../common/base.css"></head>`+
			`<body><div class="slide-scaler"><section class="slide-stage" data-gen="%s">`+
			`<h1 class="slide-title">页 %d</h1></section></div></body></html>`, c.marker, idx)
		return llm.ToolCallResponse{
			Thought:  "生成本页",
			ToolCall: &llm.ToolCall{ID: "c1", Name: "write_slide", Args: map[string]any{"slide_idx": idx, "html": html}},
		}, nil
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{ID: "c2", Name: "finish", Args: map[string]any{"summary": "done"}}}, nil
}

func parseIdxFromMessages(msgs []llm.Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != llm.RoleUser {
			continue
		}
		s := msgs[i].Content
		if k := indexOf(s, "slide_idx="); k >= 0 {
			n := 0
			for j := k + len("slide_idx="); j < len(s) && s[j] >= '0' && s[j] <= '9'; j++ {
				n = n*10 + int(s[j]-'0')
			}
			return n
		}
	}
	return 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// setupGen 落盘 slide.json 并构造 store + runner（主题 tokyo-night）。
func setupGen(t *testing.T, pageCount int) (*memStore, string, []model.Slide) {
	t.Helper()
	dir := t.TempDir()
	slides := make([]model.Slide, pageCount)
	layouts := []string{"cover", "bullets", "bullets", "bullets", "bullets", "bullets", "thanks"}
	for i := 0; i < pageCount; i++ {
		layout := "bullets"
		if i < len(layouts) {
			layout = layouts[i]
		}
		sj := slidejson.SlideJSON{ID: itoa(i), Idx: i, Layout: layout, Title: "标题" + itoa(i), Bullets: []string{"点"}}
		raw, _ := json.MarshalIndent(sj, "", "  ")
		p := filepath.Join(dir, fmt.Sprintf("slides/%03d", i))
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "slide.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		slides[i] = model.Slide{ID: itoa(i), ProjectID: "p1", Idx: i, Layout: layout, Title: sj.Title,
			JSONPath: fmt.Sprintf("slides/%03d/slide.json", i), HTMLPath: fmt.Sprintf("slides/%03d/index.html", i)}
	}
	store := newMemStore()
	store.slides = slides
	tokens, err := asset.ReadSeedFile("assets/themes/tokyo-night/tokens.css")
	if err != nil {
		t.Fatalf("read seed tokens: %v", err)
	}
	writeAtGen(t, dir, "_assets/themes/tokyo-night/manifest.json", `{
  "name":"tokyo-night","version":"1.0.0","kind":"theme","source":"preset",
  "description":"theme","assets":{"tokens":"tokens.css"}
}`)
	writeAtGen(t, dir, "_assets/themes/tokyo-night/tokens.css", string(tokens))
	store.themes = []model.Asset{{
		ID: "theme-preset", Name: "tokyo-night", Kind: "theme", Source: "preset",
		ManifestPath: "_assets/themes/tokyo-night/manifest.json", Dir: "_assets/themes/tokyo-night",
	}}
	return store, dir, slides
}

func newGenRunner(store *memStore, dir, theme, marker string, pageIndex *int) *Runner {
	seq := 0
	newID := func() string { seq++; return "v-" + itoa(seq) }
	return NewRunner(newWriteSlideClient(marker), store, Params{
		RunID: "r1", ProjectID: "p1", WorkDir: dir, WorkRoot: dir, Theme: theme, PageIndex: pageIndex,
	}, func() int64 { return 1 }, newID)
}

// AC-GEN-001 / AC-GEN-006：整套生成 → N 个 page progress，每页产出合规 html，公共层写入。
func TestGenerateDeck(t *testing.T) {
	store, dir, slides := setupGen(t, 7)
	r := newGenRunner(store, dir, "tokyo-night", "gen1", nil)

	em := &pageEmitter{}
	out := r.Run(context.Background(), em, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}
	// AC-GEN-006：page progress 事件数 == 页数。
	if len(em.pageMsgs) != len(slides) {
		t.Errorf("want %d page progress, got %d", len(slides), len(em.pageMsgs))
	}
	// 公共层写入（整套生成）。
	for _, f := range []string{"common/tokens.css", "common/base.css"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("common layer %s not written: %v", f, err)
		}
	}
	// 每页 index.html 落盘。
	for i := range slides {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("slides/%03d/index.html", i))); err != nil {
			t.Errorf("page %d html missing: %v", i, err)
		}
	}
	// project 状态 → ready。
	if store.projStatus != "ready" {
		t.Errorf("project status=%q, want ready", store.projStatus)
	}
}

// AC-SEED-004：仓库异常无 theme 资产时，生成回退内嵌 preset 主题，不崩溃。
func TestGenerateColdStartFallbackUsesSeedTheme(t *testing.T) {
	store, dir, _ := setupGen(t, 1)
	store.themes = nil

	r := newGenRunner(store, dir, "", "fallback", nil)
	out := r.Run(context.Background(), &pageEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("generate should fall back to seed theme, got %s: %s", out.Status, out.Message)
	}
	if _, err := os.Stat(filepath.Join(dir, "common/tokens.css")); err != nil {
		t.Fatalf("tokens.css should be written from fallback seed theme: %v", err)
	}
}

func TestGenerateUsesThemeFromAssetRepository(t *testing.T) {
	store, dir, _ := setupGen(t, 1)
	tokens, err := asset.ReadSeedFile("assets/themes/tokyo-night/tokens.css")
	if err != nil {
		t.Fatalf("read seed tokens: %v", err)
	}
	userTokens := strings.Replace(string(tokens), "--color-primary: #7aa2f7;", "--color-primary: #123456;", 1)
	writeAtGen(t, dir, "_assets/themes/custom-brand/manifest.json", `{
  "name":"custom-brand","version":"1.0.0","kind":"theme","source":"user",
  "description":"custom","assets":{"tokens":"tokens.css"}
}`)
	writeAtGen(t, dir, "_assets/themes/custom-brand/tokens.css", userTokens)
	store.themes = append(store.themes, model.Asset{
		ID: "theme-user", Name: "custom-brand", Kind: "theme", Source: "user",
		ManifestPath: "_assets/themes/custom-brand/manifest.json", Dir: "_assets/themes/custom-brand",
	})

	out := newGenRunner(store, dir, "custom-brand", "custom", nil).Run(context.Background(), &pageEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("generate should use repository theme, got %s: %s", out.Status, out.Message)
	}
	written, err := os.ReadFile(filepath.Join(dir, "common/tokens.css"))
	if err != nil {
		t.Fatalf("read generated tokens: %v", err)
	}
	if string(written) != userTokens {
		t.Fatalf("tokens.css must come from _assets user theme, got:\n%s", written)
	}
}

// AC-GEN-009：重生成第 5 页 → 仅该页 hash 变化，其余页与公共层不变。
func TestSinglePageRegenIsolation(t *testing.T) {
	store, dir, slides := setupGen(t, 7)
	// 先整套生成（marker=gen1）。
	if out := newGenRunner(store, dir, "tokyo-night", "gen1", nil).Run(context.Background(), &pageEmitter{}, nil, nil); out.Status != harness.OutcomeFinished {
		t.Fatalf("initial gen failed: %s", out.Message)
	}
	before := hashTree(t, dir, slides)

	// 重生成第 5 页（PageIndex=5，marker=gen2 使产物不同）。
	five := 5
	em := &pageEmitter{}
	out := newGenRunner(store, dir, "tokyo-night", "gen2", &five).Run(context.Background(), em, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("regen failed: %s", out.Message)
	}
	if len(em.pageMsgs) != 1 {
		t.Errorf("single-page regen should emit exactly 1 page progress, got %d", len(em.pageMsgs))
	}
	after := hashTree(t, dir, slides)

	for path, h := range before {
		isTarget := path == "slides/005/index.html"
		if isTarget {
			if after[path] == h {
				t.Errorf("target page %s hash should change on regen", path)
			}
			continue
		}
		if after[path] != h {
			t.Errorf("non-target %s hash changed on single-page regen (AC-GEN-009 violated)", path)
		}
	}
}

// hashTree 计算公共层 + 各页 index.html 的内容 hash。
func hashTree(t *testing.T, dir string, slides []model.Slide) map[string]string {
	t.Helper()
	out := map[string]string{}
	paths := []string{"common/tokens.css", "common/base.css"}
	for i := range slides {
		paths = append(paths, fmt.Sprintf("slides/%03d/index.html", i))
	}
	for _, p := range paths {
		raw, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			out[p] = "MISSING"
			continue
		}
		sum := sha256.Sum256(raw)
		out[p] = fmt.Sprintf("%x", sum)
	}
	return out
}

func writeAtGen(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
