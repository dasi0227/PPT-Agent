package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

var errSlideInjected = errors.New("injected slide failure")

// TestReadContentFromDisk：ReadContent 从 slide.json 读全文（含 bullets）。
func TestReadContentFromDisk(t *testing.T) {
	svc, st, workDir := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{{ID: "s1", ProjectID: "p1", Order: 10, Layout: "bullets", Title: "标题",
		JSONPath: model.SlideJSONPath("s1"), HTMLPath: model.SlideHTMLPath("s1")}})
	writeAt(t, workDir, model.SlideJSONPath("s1"),
		`{"id":"s1","layout":"bullets","title":"标题","bullets":["a","b"]}`)
	got, err := svc.ReadContent(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "标题" || len(got.Bullets) != 2 {
		t.Fatalf("got %+v", got)
	}
}

// TestPatchContentUpdatesAndMarksDirty：局部更新 slide.json + 有 html 页置脏 + 同步 title 元数据。
func TestPatchContentUpdatesAndMarksDirty(t *testing.T) {
	svc, st, workDir := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{{ID: "s1", ProjectID: "p1", Order: 10, Layout: "bullets", Title: "旧",
		JSONPath: model.SlideJSONPath("s1"), HTMLPath: model.SlideHTMLPath("s1")}})
	writeAt(t, workDir, model.SlideJSONPath("s1"), `{"id":"s1","layout":"bullets","title":"旧","bullets":["a"]}`)
	writeAt(t, workDir, model.SlideHTMLPath("s1"), `<html></html>`) // 有 html → 应置脏
	newTitle := "新标题"
	got, err := svc.PatchContent(ctx, "s1", service.SlidePatch{Title: &newTitle})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "新标题" {
		t.Fatalf("json title not updated: %+v", got)
	}
	sl, _ := st.GetSlide(ctx, "s1")
	if !sl.OutlineDirty {
		t.Fatal("expected outline_dirty=true when html exists")
	}
	if sl.Title != "新标题" {
		t.Fatal("db meta title not synced")
	}
}

// TestPatchContentRejectedWhenRunActive：project 有活跃 run 时手动 PATCH 返回 ErrRunActive。
func TestPatchContentRejectedWhenRunActive(t *testing.T) {
	svc, st, workDir := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{{ID: "s1", ProjectID: "p1", Order: 10, Layout: "bullets", Title: "旧",
		JSONPath: model.SlideJSONPath("s1"), HTMLPath: model.SlideHTMLPath("s1")}})
	writeAt(t, workDir, model.SlideJSONPath("s1"), `{"id":"s1","layout":"bullets","title":"旧","bullets":["a"]}`)
	if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(ctx, model.Run{ID: "r1", ProjectID: "p1", ThreadID: "t1", Kind: model.KindOutline, Scope: model.ScopeCurrent, Mode: model.ModeNormal, Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
	newTitle := "X"
	if _, err := svc.PatchContent(ctx, "s1", service.SlidePatch{Title: &newTitle}); !errors.Is(err, service.ErrRunActive) {
		t.Fatalf("want ErrRunActive, got %v", err)
	}
}

// activateRun 在 p1 造一个 running run（附带合法 thread FK），用于 RUN_ACTIVE 互斥断言。
func activateRun(t *testing.T, st *sqlitestore.Store) {
	t.Helper()
	ctx := context.Background()
	if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(ctx, model.Run{ID: "r1", ProjectID: "p1", ThreadID: "t1", Kind: model.KindOutline, Scope: model.ScopeCurrent, Mode: model.ModeNormal, Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
}

// TestAddSlideCreatesBlankJSONAndOrders：AddSlide 在锚点后插入空白页，磁盘落 slide.json，列表新增且顺序正确。
func TestAddSlideCreatesBlankJSONAndOrders(t *testing.T) {
	svc, st, workDir := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1", Idx: 0, Order: 10, Layout: "cover", Title: "A", JSONPath: model.SlideJSONPath("a"), HTMLPath: model.SlideHTMLPath("a")},
		{ID: "b", ProjectID: "p1", Idx: 1, Order: 20, Layout: "thanks", Title: "B", JSONPath: model.SlideJSONPath("b"), HTMLPath: model.SlideHTMLPath("b")},
	})
	got, err := svc.AddSlide(ctx, "p1", "a", "bullets")
	if err != nil {
		t.Fatalf("add slide: %v", err)
	}
	if got.Order <= 10 || got.Order >= 20 {
		t.Fatalf("new slide order should sit between anchors, got %d", got.Order)
	}
	full := filepath.Join(workDir, filepath.FromSlash(model.SlideJSONPath(got.ID)))
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("blank slide.json not written: %v", err)
	}
	list, _ := st.ListSlides(ctx, "p1")
	if len(list) != 3 || list[0].ID != "a" || list[1].ID != got.ID || list[2].ID != "b" {
		t.Fatalf("insert order wrong: %+v", list)
	}
}

// TestAddSlideAtEndAppends：无后继锚点时 order = 锚点 + 10。
func TestAddSlideAtEndAppends(t *testing.T) {
	svc, st, _ := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1", Idx: 0, Order: 10, Layout: "cover", Title: "A", JSONPath: model.SlideJSONPath("a"), HTMLPath: model.SlideHTMLPath("a")},
	})
	got, err := svc.AddSlide(ctx, "p1", "a", "bullets")
	if err != nil {
		t.Fatalf("add slide: %v", err)
	}
	if got.Order != 20 {
		t.Fatalf("append order should be anchor+10=20, got %d", got.Order)
	}
}

// TestDeleteSlideRemovesRowAndDir：DeleteSlide 删 DB 行 + slides/<id>/ 目录。
func TestDeleteSlideRemovesRowAndDir(t *testing.T) {
	svc, st, workDir := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1", Idx: 0, Order: 10, Layout: "cover", Title: "A", JSONPath: model.SlideJSONPath("a"), HTMLPath: model.SlideHTMLPath("a")},
		{ID: "b", ProjectID: "p1", Idx: 1, Order: 20, Layout: "thanks", Title: "B", JSONPath: model.SlideJSONPath("b"), HTMLPath: model.SlideHTMLPath("b")},
	})
	writeAt(t, workDir, model.SlideJSONPath("a"), `{"id":"a","idx":0,"layout":"cover","title":"A"}`)
	writeAt(t, workDir, model.SlideHTMLPath("a"), `<html></html>`)
	if err := svc.DeleteSlide(ctx, "a"); err != nil {
		t.Fatalf("delete slide: %v", err)
	}
	list, _ := st.ListSlides(ctx, "p1")
	if len(list) != 1 || list[0].ID != "b" {
		t.Fatalf("delete wrong: %+v", list)
	}
	dir := filepath.Join(workDir, filepath.FromSlash(model.SlideDir("a")))
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("slide dir should be removed, stat err=%v", err)
	}
}

// TestReorderSlidesAssignsIntervalOrder：ReorderSlides 按给定顺序赋 order=i*10。
func TestReorderSlidesAssignsIntervalOrder(t *testing.T) {
	svc, st, _ := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1", Idx: 0, Order: 10, Layout: "cover", Title: "A"},
		{ID: "b", ProjectID: "p1", Idx: 1, Order: 20, Layout: "bullets", Title: "B"},
		{ID: "c", ProjectID: "p1", Idx: 2, Order: 30, Layout: "thanks", Title: "C"},
	})
	if err := svc.ReorderSlides(ctx, "p1", []string{"c", "a", "b"}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	list, _ := st.ListSlides(ctx, "p1")
	if list[0].ID != "c" || list[1].ID != "a" || list[2].ID != "b" {
		t.Fatalf("reorder order wrong: %+v", list)
	}
	if list[0].Order != 0 || list[1].Order != 10 || list[2].Order != 20 {
		t.Fatalf("interval order wrong: %+v", list)
	}
}

// TestStructuralOpsRejectedWhenRunActive：活跃 run 时 Add/Delete/Reorder 均返回 ErrRunActive。
func TestStructuralOpsRejectedWhenRunActive(t *testing.T) {
	svc, st, _ := newSlideServiceWithProject(t)
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1", Idx: 0, Order: 10, Layout: "cover", Title: "A", JSONPath: model.SlideJSONPath("a"), HTMLPath: model.SlideHTMLPath("a")},
		{ID: "b", ProjectID: "p1", Idx: 1, Order: 20, Layout: "thanks", Title: "B", JSONPath: model.SlideJSONPath("b"), HTMLPath: model.SlideHTMLPath("b")},
	})
	activateRun(t, st)
	if _, err := svc.AddSlide(ctx, "p1", "a", "bullets"); !errors.Is(err, service.ErrRunActive) {
		t.Fatalf("AddSlide want ErrRunActive, got %v", err)
	}
	if err := svc.DeleteSlide(ctx, "a"); !errors.Is(err, service.ErrRunActive) {
		t.Fatalf("DeleteSlide want ErrRunActive, got %v", err)
	}
	if err := svc.ReorderSlides(ctx, "p1", []string{"b", "a"}); !errors.Is(err, service.ErrRunActive) {
		t.Fatalf("ReorderSlides want ErrRunActive, got %v", err)
	}
}

type failingSlideStore struct {
	*sqlitestore.Store
	failCreateVersion   bool
	failSetSlideVersion bool
}

func (s *failingSlideStore) CreateVersion(ctx context.Context, v model.Version) error {
	if s.failCreateVersion {
		return errSlideInjected
	}
	return s.Store.CreateVersion(ctx, v)
}

func (s *failingSlideStore) SetSlideVersion(ctx context.Context, slideID string, versionNo int) error {
	if s.failSetSlideVersion {
		return errSlideInjected
	}
	return s.Store.SetSlideVersion(ctx, slideID, versionNo)
}

// setupRollback 建库 + project work_dir + 一页两版本快照（v0/v1），当前 v1。
func setupRollback(t *testing.T) (*service.SlideService, *sqlitestore.Store, string, string) {
	t.Helper()
	work := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(work, "t.db"), WorkRoot: work}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ctx := context.Background()
	now := time.Now().Unix()
	workDir := filepath.Join(work, "p1")
	if err := st.CreateProject(ctx, model.Project{ID: "p1", Title: "t", WorkDir: workDir, Theme: "x", Status: "ready", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	// slide s1，current_version=1。
	slide := model.Slide{ID: "s1", ProjectID: "p1", Idx: 0, Order: 0, Layout: "cover", Title: "T",
		JSONPath: model.SlideJSONPath("s1"), HTMLPath: model.SlideHTMLPath("s1"), CurrentVersion: 1}
	if err := st.ReplaceSlides(ctx, "p1", []model.Slide{slide}); err != nil {
		t.Fatal(err)
	}

	// 落两版本快照 + 当前 html=v1 内容。
	writeAt(t, workDir, model.SlideVersionSnapshot("s1", 0), "<html>V0</html>")
	writeAt(t, workDir, model.SlideVersionSnapshot("s1", 1), "<html>V1</html>")
	writeAt(t, workDir, model.SlideHTMLPath("s1"), "<html>V1</html>")
	for no := 0; no <= 1; no++ {
		if err := st.CreateVersion(ctx, model.Version{
			ID: "ver" + itoa(no), TargetType: "slide", TargetID: model.SlideVersionTarget("p1", "s1"), VersionNo: no,
			SnapshotPath: model.SlideVersionSnapshot("s1", no), CreatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return service.NewSlideService(st), st, workDir, "s1"
}

// AC-VERSION-004 / AC-EDIT-005：回滚到 v0 → 文件内容==v0，新增 v2（内容=v0），current_version=2。
func TestRollbackCreatesNewVersion(t *testing.T) {
	svc, st, workDir, slideID := setupRollback(t)
	ctx := context.Background()

	sl, err := svc.RollbackSlide(ctx, slideID, 0)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// 文件恢复为 v0 内容。
	raw, _ := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath("s1"))))
	if string(raw) != "<html>V0</html>" {
		t.Errorf("file not restored to v0: %q", raw)
	}
	// 新增 v2（不删旧版本），内容=v0。
	versions, _ := st.ListVersions(ctx, "slide", model.SlideVersionTarget("p1", "s1"))
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions after rollback (v0,v1,v2), got %d", len(versions))
	}
	newSnap, _ := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideVersionSnapshot("s1", 2))))
	if string(newSnap) != "<html>V0</html>" {
		t.Errorf("new version snapshot content should equal v0, got %q", newSnap)
	}
	// current_version 指向新版本 2（DATA-VERSION-005）。
	if sl.CurrentVersion != 2 {
		t.Errorf("returned current_version=%d, want 2", sl.CurrentVersion)
	}
	slides, _ := st.ListSlides(ctx, "p1")
	if slides[0].CurrentVersion != 2 {
		t.Errorf("persisted current_version=%d, want 2", slides[0].CurrentVersion)
	}
}

// 回滚到不存在的版本号 → ErrVersionNotFound。
func TestRollbackUnknownVersion(t *testing.T) {
	svc, _, _, slideID := setupRollback(t)
	if _, err := svc.RollbackSlide(context.Background(), slideID, 99); err != service.ErrVersionNotFound {
		t.Errorf("want ErrVersionNotFound, got %v", err)
	}
}

func TestRollbackCreateVersionFailureRestoresCurrentFile(t *testing.T) {
	_, st, workDir, slideID := setupRollback(t)
	svc := service.NewSlideService(&failingSlideStore{Store: st, failCreateVersion: true})

	_, err := svc.RollbackSlide(context.Background(), slideID, 0)
	if !errors.Is(err, errSlideInjected) {
		t.Fatalf("expected injected create version error, got %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath("s1"))))
	if string(raw) != "<html>V1</html>" {
		t.Fatalf("current file must be restored when CreateVersion fails: %s", raw)
	}
	slides, _ := st.ListSlides(context.Background(), "p1")
	if slides[0].CurrentVersion != 1 {
		t.Fatalf("current_version must remain unchanged, got %d", slides[0].CurrentVersion)
	}
	versions, _ := st.ListVersions(context.Background(), "slide", model.SlideVersionTarget("p1", "s1"))
	if len(versions) != 2 {
		t.Fatalf("failed rollback must not append version rows, got %+v", versions)
	}
}

func TestRollbackSetSlideVersionFailureRestoresCurrentFile(t *testing.T) {
	_, st, workDir, slideID := setupRollback(t)
	svc := service.NewSlideService(&failingSlideStore{Store: st, failSetSlideVersion: true})

	_, err := svc.RollbackSlide(context.Background(), slideID, 0)
	if !errors.Is(err, errSlideInjected) {
		t.Fatalf("expected injected set slide version error, got %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath("s1"))))
	if string(raw) != "<html>V1</html>" {
		t.Fatalf("current file must be restored when SetSlideVersion fails: %s", raw)
	}
	slides, _ := st.ListSlides(context.Background(), "p1")
	if slides[0].CurrentVersion != 1 {
		t.Fatalf("current_version must remain unchanged, got %d", slides[0].CurrentVersion)
	}
	versions, _ := st.ListVersions(context.Background(), "slide", model.SlideVersionTarget("p1", "s1"))
	if len(versions) != 2 {
		t.Fatalf("failed SetSlideVersion must remove appended version row, got %+v", versions)
	}
}

func TestSlideVersionsAreProjectScoped(t *testing.T) {
	work := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(work, "scope.db"), WorkRoot: work}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ctx := context.Background()
	now := time.Now().Unix()
	for _, p := range []string{"p1", "p2"} {
		workDir := filepath.Join(work, p)
		if err := st.CreateProject(ctx, model.Project{ID: p, Title: p, WorkDir: workDir, Theme: "x", Status: "ready", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := st.ReplaceSlides(ctx, p, []model.Slide{{
			ID: p + "-s0", ProjectID: p, Idx: 0, Layout: "cover", Title: "T",
			JSONPath: "slides/000/slide.json", HTMLPath: "slides/000/index.html",
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateVersion(ctx, model.Version{ID: "v-p1", TargetType: "slide", TargetID: "slide-000", VersionNo: 0, SnapshotPath: "versions/slide-000/v0.html", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateVersion(ctx, model.Version{ID: "v-p2", TargetType: "slide", TargetID: "slide-000", VersionNo: 1, SnapshotPath: "versions/slide-000/v1.html", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	svc := service.NewSlideService(st)
	versions, err := svc.ListVersions(ctx, "p1-s0")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("p1 must not see legacy/global slide-000 versions from other projects, got %+v", versions)
	}
}

// store.GetSlide 按 id 反查。
func TestGetSlideByID(t *testing.T) {
	_, st, _, slideID := setupRollback(t)
	sl, err := st.GetSlide(context.Background(), slideID)
	if err != nil {
		t.Fatalf("get slide: %v", err)
	}
	if sl.Idx != 0 || sl.ProjectID != "p1" {
		t.Errorf("unexpected slide: %+v", sl)
	}
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

// newSlideServiceWithProject 建库 + 用 ProjectService 造 project "p1"，返回 svc/store/workDir。
func newSlideServiceWithProject(t *testing.T) (*service.SlideService, *sqlitestore.Store, string) {
	t.Helper()
	work := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(work, "t.db"), WorkRoot: work}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ctx := context.Background()
	now := time.Now().Unix()
	workDir := filepath.Join(work, "p1")
	if err := st.CreateProject(ctx, model.Project{ID: "p1", Title: "t", WorkDir: workDir, Theme: "x", Status: "draft", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	return service.NewSlideService(st), st, workDir
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
