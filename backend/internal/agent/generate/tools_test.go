package generate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// memStore 是 generate.Store 的内存实现。
type memStore struct {
	slides      []model.Slide
	versions    []model.Version
	slideVer    map[string]int
	dirtyByID   map[string]bool
	projStatus  string
	themes      []model.Asset
	nextVerByTg map[string]int
	failCreate  bool
}

func newMemStore() *memStore {
	return &memStore{slideVer: map[string]int{}, dirtyByID: map[string]bool{}, nextVerByTg: map[string]int{}}
}

func (m *memStore) ListSlides(_ context.Context, _ string) ([]model.Slide, error) {
	return m.slides, nil
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
func (m *memStore) SetSlideVersion(_ context.Context, slideID string, no int) error {
	m.slideVer[slideID] = no
	return nil
}
func (m *memStore) SetOutlineDirty(_ context.Context, slideID string, dirty bool) error {
	m.dirtyByID[slideID] = dirty
	return nil
}
func (m *memStore) SetProjectStatus(_ context.Context, _, status string) error {
	m.projStatus = status
	return nil
}
func (m *memStore) ListAssets(_ context.Context, kind string) ([]model.Asset, error) {
	if kind == "theme" {
		return m.themes, nil
	}
	return nil, nil
}

const goodHTML = `<!doctype html><html><head>` +
	`<link rel="stylesheet" href="../../common/tokens.css">` +
	`<link rel="stylesheet" href="../../common/base.css"></head>` +
	`<body><div class="slide-scaler"><section class="slide-stage"><h1 class="slide-title">T</h1></section></div></body></html>`

func newWriteTool(t *testing.T, store Store, idx int) (*WriteSlideTool, string, string) {
	t.Helper()
	dir := t.TempDir()
	sb, err := tools.NewSandbox(dir)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	seq := 0
	newID := func() string { seq++; return "v-" + itoa(seq) }
	slideID := "s" + itoa(idx)
	return NewWriteSlideTool(store, sb, "p1", "r1", idx, slideID, func() int64 { return 1 }, newID), dir, slideID
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

// AC-HTML-001 / ARCH-TOOLS-004：合规 html 写入成功、落盘、落版本、同步 slide 版本。
func TestWriteSlideValidPersists(t *testing.T) {
	store := newMemStore()
	tool, dir, slideID := newWriteTool(t, store, 0)
	store.dirtyByID[slideID] = true // 预置脏标记，验证写页成功后被清除
	res, err := tool.Execute(context.Background(), map[string]any{"slide_idx": 0, "html": goodHTML})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got: %s", res.Observation)
	}
	if !tool.Written() {
		t.Error("Written() should be true after success")
	}
	if _, err := os.Stat(filepath.Join(dir, model.SlideHTMLPath(slideID))); err != nil {
		t.Errorf("index.html not written: %v", err)
	}
	if len(store.versions) != 1 || store.versions[0].TargetType != "slide" {
		t.Errorf("expected 1 slide version, got %+v", store.versions)
	}
	if store.slideVer[slideID] != 0 {
		t.Errorf("slide %s version not synced: %d", slideID, store.slideVer[slideID])
	}
	if store.dirtyByID[slideID] {
		t.Errorf("slide %s outline_dirty should be cleared after regenerate", slideID)
	}
}

// ARCH-TOOLS-004：不合规 html（无公共层）落盘前被拒，不写文件。
func TestWriteSlideRejectsInvalid(t *testing.T) {
	store := newMemStore()
	tool, dir, slideID := newWriteTool(t, store, 0)
	bad := `<!doctype html><html><body><div class="slide-stage">no common layer</div></body></html>`
	res, _ := tool.Execute(context.Background(), map[string]any{"slide_idx": 0, "html": bad})
	if res.OK {
		t.Fatal("expected rejection for invalid html")
	}
	if _, err := os.Stat(filepath.Join(dir, model.SlideHTMLPath(slideID))); !os.IsNotExist(err) {
		t.Error("must not write file when validation fails")
	}
	if len(store.versions) != 0 {
		t.Error("must not create version on validation failure")
	}
}

func TestWriteSlideRemovesNewFileWhenVersionCreateFails(t *testing.T) {
	store := newMemStore()
	store.failCreate = true
	tool, dir, slideID := newWriteTool(t, store, 0)
	res, err := tool.Execute(context.Background(), map[string]any{"slide_idx": 0, "html": goodHTML})
	if err == nil {
		t.Fatal("expected CreateVersion error")
	}
	if res.OK {
		t.Fatalf("result must not be OK on DB failure: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, model.SlideHTMLPath(slideID))); !os.IsNotExist(err) {
		t.Fatalf("new current file must be removed on DB failure, stat err=%v", err)
	}
}

// 子代理页锁定：写非本页 slide_idx → 越权失败。
func TestWriteSlidePageLock(t *testing.T) {
	store := newMemStore()
	tool, _, _ := newWriteTool(t, store, 3) // 锁定第 3 页
	res, _ := tool.Execute(context.Background(), map[string]any{"slide_idx": 5, "html": goodHTML})
	if res.OK {
		t.Fatal("expected rejection: writing another page")
	}
}

// AC-TOOLS-004（validate_slide）：显式校验返回逐项结果。
func TestValidateSlideTool(t *testing.T) {
	tool := NewValidateSlideTool()
	ok, _ := tool.Execute(context.Background(), map[string]any{"html": goodHTML})
	if !ok.OK {
		t.Fatalf("valid html should pass: %s", ok.Observation)
	}
	bad, _ := tool.Execute(context.Background(), map[string]any{"html": `<div>x</div>`})
	if bad.OK {
		t.Fatal("expected validate to fail for non-compliant html")
	}
}
