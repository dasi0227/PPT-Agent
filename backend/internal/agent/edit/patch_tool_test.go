package edit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// memStore 是 edit.Store 的内存实现。
type memStore struct {
	slides      []model.Slide
	versions    []model.Version
	slideVer    map[string]int
	nextVerByTg map[string]int
	failCreate  bool
}

func newMemStore() *memStore {
	return &memStore{slideVer: map[string]int{}, nextVerByTg: map[string]int{}}
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
func (m *memStore) ListAssets(_ context.Context, _ string) ([]model.Asset, error) {
	return nil, nil
}
func (m *memStore) GetAsset(_ context.Context, _ string) (model.Asset, error) {
	return model.Asset{}, os.ErrNotExist
}

// validSlide 是一份合规 slide html（含公共层引用 + 舞台 + 一个可锚定的标题）。
const validSlide = `<!doctype html><html><head>` +
	`<link rel="stylesheet" href="../../common/tokens.css">` +
	`<link rel="stylesheet" href="../../common/base.css"></head>` +
	`<body><div class="slide-scaler"><section class="slide-stage">` +
	`<h1 class="slide-title">原标题</h1></section></div></body></html>`

func setupPatch(t *testing.T, idx int, html string) (*PatchSlideTool, *memStore, string, string) {
	t.Helper()
	dir := t.TempDir()
	sb, err := tools.NewSandbox(dir)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	slideID := "s" + itoa(idx)
	rel := filepath.Join(dir, filepath.FromSlash(model.SlideHTMLPath(slideID)))
	if err := os.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rel, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newMemStore()
	seq := 0
	newID := func() string { seq++; return "v-" + itoa(seq) }
	tool := NewPatchSlideTool(store, sb, "p1", "r1", idx, slideID, func() int64 { return 1 }, newID)
	return tool, store, dir, slideID
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

func edits(pairs ...string) []any {
	var out []any
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, map[string]any{"old_text": pairs[i], "new_text": pairs[i+1]})
	}
	return out
}

// AC-EDIT-005 / DATA-VERSION-001：合规锚定替换 → 落盘 + 产新版本 + 更新 current_version。
func TestPatchValidPersistsAndVersions(t *testing.T) {
	tool, store, dir, slideID := setupPatch(t, 3, validSlide)
	res, err := tool.Execute(context.Background(), map[string]any{
		"slide_idx": 3, "edits": edits("原标题", "新标题"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got: %s", res.Observation)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.SlideHTMLPath(slideID))))
	if !contains(string(raw), "新标题") || contains(string(raw), "原标题") {
		t.Errorf("patch not applied: %s", raw)
	}
	if len(store.versions) != 1 || store.versions[0].TargetID != model.SlideVersionTarget("p1", slideID) {
		t.Errorf("expected 1 slide version, got %+v", store.versions)
	}
	if store.slideVer[slideID] != 0 {
		t.Errorf("current_version not synced: %d", store.slideVer[slideID])
	}
}

// ARCH-TOOLS-003：锚点不唯一 → 整体失败，文件不变，无版本。
func TestPatchAnchorNotUnique(t *testing.T) {
	dup := `<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage">` +
		`<span>x</span><span>x</span></section></div></body></html>`
	tool, store, dir, slideID := setupPatch(t, 0, dup)
	res, _ := tool.Execute(context.Background(), map[string]any{
		"slide_idx": 0, "edits": edits("<span>x</span>", "<span>y</span>"),
	})
	if res.OK {
		t.Fatal("expected failure: anchor not unique")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.SlideHTMLPath(slideID))))
	if string(raw) != dup {
		t.Error("file must be unchanged on anchor failure")
	}
	if len(store.versions) != 0 {
		t.Error("no version on failure")
	}
}

// ARCH-TOOLS-003：锚点不存在 → 整体失败。
func TestPatchAnchorMissing(t *testing.T) {
	tool, _, _, _ := setupPatch(t, 0, validSlide)
	res, _ := tool.Execute(context.Background(), map[string]any{
		"slide_idx": 0, "edits": edits("不存在的锚点", "x"),
	})
	if res.OK {
		t.Fatal("expected failure: anchor missing")
	}
}

func TestPatchRestoresFileWhenVersionCreateFails(t *testing.T) {
	tool, store, dir, slideID := setupPatch(t, 0, validSlide)
	store.failCreate = true

	res, err := tool.Execute(context.Background(), map[string]any{
		"slide_idx": 0, "edits": edits("原标题", "新标题"),
	})
	if err == nil {
		t.Fatal("expected CreateVersion error")
	}
	if res.OK {
		t.Fatalf("result must not be OK on DB failure: %+v", res)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.SlideHTMLPath(slideID))))
	if string(raw) != validSlide {
		t.Fatalf("current slide must be restored on DB failure:\n%s", raw)
	}
}

// ARCH-TOOLS-004：替换后破坏 html-output-spec（删掉公共层引用）→ 拒绝落盘。
func TestPatchRejectsInvalidResult(t *testing.T) {
	tool, store, dir, slideID := setupPatch(t, 0, validSlide)
	res, _ := tool.Execute(context.Background(), map[string]any{
		"slide_idx": 0,
		"edits":     edits(`<link rel="stylesheet" href="../../common/base.css">`, ""),
	})
	if res.OK {
		t.Fatal("expected failure: result violates html-output-spec")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.SlideHTMLPath(slideID))))
	if string(raw) != validSlide {
		t.Error("file must be unchanged when validation fails")
	}
	if len(store.versions) != 0 {
		t.Error("no version on validation failure")
	}
}

// 页锁定：改非目标页 slide_idx → 越权失败。
func TestPatchPageLock(t *testing.T) {
	tool, _, _, _ := setupPatch(t, 3, validSlide)
	res, _ := tool.Execute(context.Background(), map[string]any{
		"slide_idx": 5, "edits": edits("原标题", "x"),
	})
	if res.OK {
		t.Fatal("expected failure: editing another page")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// writeFileAt 在 dir 下按相对路径写文件（建目录）。
func writeFileAt(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
