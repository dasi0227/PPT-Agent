package outline

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// memStore 是 outline.Store 的内存实现（不打真实 SQLite）。
type memStore struct {
	slides   []model.Slide
	versions []model.Version
}

func (m *memStore) ReplaceSlides(_ context.Context, _ string, slides []model.Slide) error {
	m.slides = slides
	return nil
}
func (m *memStore) NextVersionNo(_ context.Context, _, _ string) (int, error) {
	return len(m.versions), nil
}
func (m *memStore) CreateVersion(_ context.Context, v model.Version) error {
	m.versions = append(m.versions, v)
	return nil
}

func newTool(t *testing.T, store Store, slideCount int) (*SubmitOutlineTool, string) {
	t.Helper()
	dir := t.TempDir()
	sb, err := tools.NewSandbox(dir)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	seq := 0
	newID := func() string { seq++; return "id-" + itoa(seq) }
	return NewSubmitOutlineTool(store, sb, "p1", "r1", slideCount, func() int64 { return 100 }, newID), dir
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

func slide(layout, title string, bullets ...string) map[string]any {
	m := map[string]any{"layout": layout, "title": title}
	if len(bullets) > 0 {
		bs := make([]any, len(bullets))
		for i, b := range bullets {
			bs[i] = b
		}
		m["bullets"] = bs
	}
	return m
}

func validOutline() []any {
	return []any{
		slide("cover", "封面", "副标题"),
		slide("bullets", "背景", "点1", "点2"),
		slide("bullets", "架构", "点1"),
		slide("bullets", "指标", "点1"),
		slide("bullets", "落地", "点1"),
		slide("bullets", "总结", "点1"),
		slide("thanks", "谢谢", "联系我们"),
	}
}

// AC-OUTLINE-001：提交后每页通过 schema，含 title/layout/(bullets|content_intent)，落库+落盘。
func TestOutlineSubmitValid(t *testing.T) {
	store := &memStore{}
	tool, dir := newTool(t, store, 0)

	res, err := tool.Execute(context.Background(), map[string]any{"slides": validOutline()})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got: %s", res.Observation)
	}
	if len(store.slides) != 7 {
		t.Fatalf("want 7 slides in store, got %d", len(store.slides))
	}
	// 服务端权威回填 id/idx，并按间隔分配 order。
	for i, s := range store.slides {
		if s.Idx != i {
			t.Errorf("slide %d idx=%d", i, s.Idx)
		}
		if s.ID == "" {
			t.Errorf("slide %d missing id", i)
		}
		if s.Order != i*10 {
			t.Errorf("slide %d order=%d, want %d", i, s.Order, i*10)
		}
		if s.JSONPath != model.SlideJSONPath(s.ID) {
			t.Errorf("slide %d json_path=%q, want %q", i, s.JSONPath, model.SlideJSONPath(s.ID))
		}
		if s.HTMLPath != model.SlideHTMLPath(s.ID) {
			t.Errorf("slide %d html_path=%q, want %q", i, s.HTMLPath, model.SlideHTMLPath(s.ID))
		}
	}
	// slide.json 落盘到 slides/<id>/ 且合法。
	raw, err := os.ReadFile(filepath.Join(dir, model.SlideJSONPath(store.slides[0].ID)))
	if err != nil {
		t.Fatalf("read slide.json: %v", err)
	}
	var sj map[string]any
	if err := json.Unmarshal(raw, &sj); err != nil {
		t.Fatalf("slide.json not valid json: %v", err)
	}
	if sj["layout"] != "cover" {
		t.Errorf("slide 0 layout=%v", sj["layout"])
	}
	// SPEC-OUTLINE-008：version 0 已创建。
	if len(store.versions) != 1 || store.versions[0].VersionNo != 0 {
		t.Fatalf("expected version 0, got %+v", store.versions)
	}
	if store.versions[0].TargetType != "project" || store.versions[0].RunID != "r1" {
		t.Errorf("bad version meta: %+v", store.versions[0])
	}
}

// AC-OUTLINE-002：首页非 cover → 拒绝。
func TestOutlineRejectsNonCoverFirst(t *testing.T) {
	store := &memStore{}
	tool, _ := newTool(t, store, 0)
	bad := validOutline()
	bad[0] = slide("bullets", "不是封面", "x")
	res, _ := tool.Execute(context.Background(), map[string]any{"slides": bad})
	if res.OK {
		t.Fatal("expected rejection for non-cover first page")
	}
	if len(store.slides) != 0 {
		t.Error("must not persist on structure failure")
	}
}

// AC-OUTLINE-002：末页非 thanks/cta → 拒绝。
func TestOutlineRejectsBadLastPage(t *testing.T) {
	store := &memStore{}
	tool, _ := newTool(t, store, 0)
	bad := validOutline()
	bad[len(bad)-1] = slide("bullets", "普通页", "x")
	res, _ := tool.Execute(context.Background(), map[string]any{"slides": bad})
	if res.OK {
		t.Fatal("expected rejection for last page not in {thanks,cta}")
	}
}

// AC-OUTLINE-002：末页 cta 可接受。
func TestOutlineAcceptsCtaLast(t *testing.T) {
	store := &memStore{}
	tool, _ := newTool(t, store, 0)
	o := validOutline()
	o[len(o)-1] = slide("cta", "立即行动", "扫码")
	res, _ := tool.Execute(context.Background(), map[string]any{"slides": o})
	if !res.OK {
		t.Fatalf("cta last page should be accepted: %s", res.Observation)
	}
}

// AC-OUTLINE-002：未指定页数时，中间页数 <5 → 拒绝。
func TestOutlineRejectsTooFewBody(t *testing.T) {
	store := &memStore{}
	tool, _ := newTool(t, store, 0)
	few := []any{
		slide("cover", "封面", "x"),
		slide("bullets", "唯一正文", "x"),
		slide("thanks", "谢谢", "x"),
	}
	res, _ := tool.Execute(context.Background(), map[string]any{"slides": few})
	if res.OK {
		t.Fatal("expected rejection: body pages < 5")
	}
}

// AC-OUTLINE-002：指定页数则必须精确匹配。
func TestOutlineFixedCountEnforced(t *testing.T) {
	store := &memStore{}
	tool, _ := newTool(t, store, 3)                                                        // 要求恰 3 页
	res, _ := tool.Execute(context.Background(), map[string]any{"slides": validOutline()}) // 提交 7 页
	if res.OK {
		t.Fatal("expected rejection: count mismatch")
	}

	store2 := &memStore{}
	tool2, _ := newTool(t, store2, 3)
	three := []any{
		slide("cover", "封面", "x"),
		slide("bullets", "正文", "x"),
		slide("thanks", "谢谢", "x"),
	}
	res2, _ := tool2.Execute(context.Background(), map[string]any{"slides": three})
	if !res2.OK {
		t.Fatalf("exact count should pass: %s", res2.Observation)
	}
}

// SPEC-OUTLINE-003：缺 bullets 且缺 content_intent → 拒绝。
func TestOutlineRejectsMissingContent(t *testing.T) {
	store := &memStore{}
	tool, _ := newTool(t, store, 0)
	o := validOutline()
	o[1] = map[string]any{"layout": "bullets", "title": "空内容页"} // 无 bullets/content_intent
	res, _ := tool.Execute(context.Background(), map[string]any{"slides": o})
	if res.OK {
		t.Fatal("expected rejection for missing bullets/content_intent")
	}
}

// 两阶段边界：即便 LLM 误塞 html 字段，也 MUST NOT 出现在落盘的 slide-json 中（被剥离）。
func TestOutlineStripsHTMLLeak(t *testing.T) {
	store := &memStore{}
	tool, dir := newTool(t, store, 0)
	o := validOutline()
	m := o[1].(map[string]any)
	m["html"] = "<div>leak</div>"
	res, err := tool.Execute(context.Background(), map[string]any{"slides": o})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok (html stripped, not rejected): %s", res.Observation)
	}
	// 落盘的第 1 页 slide.json 不得含 html 字段（两阶段边界）。
	raw, err := os.ReadFile(filepath.Join(dir, model.SlideJSONPath(store.slides[1].ID)))
	if err != nil {
		t.Fatalf("read slide.json: %v", err)
	}
	if bytes.Contains(raw, []byte("html")) || bytes.Contains(raw, []byte("leak")) {
		t.Errorf("html leaked into persisted slide-json: %s", raw)
	}
}
