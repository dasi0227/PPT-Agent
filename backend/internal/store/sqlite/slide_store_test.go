package sqlite

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func seedProject(t *testing.T, s *Store) {
	t.Helper()
	if err := s.CreateProject(context.Background(), model.Project{
		ID: "p1", Title: "t", WorkDir: "/tmp/p1", Theme: "default", Status: "draft", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
}

func TestReplaceSlidesAndList(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()

	slides := []model.Slide{
		{ID: "s0", ProjectID: "p1", Idx: 0, Layout: "cover", Title: "封面", JSONPath: "slides/000/slide.json", HTMLPath: "slides/000/index.html"},
		{ID: "s1", ProjectID: "p1", Idx: 1, Layout: "thanks", Title: "谢谢", JSONPath: "slides/001/slide.json", HTMLPath: "slides/001/index.html"},
	}
	if err := s.ReplaceSlides(ctx, "p1", slides); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := s.ListSlides(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].Layout != "cover" || got[1].Layout != "thanks" {
		t.Fatalf("bad slides: %+v", got)
	}

	// 幂等替换：再次以 3 页替换，旧的被清掉。
	three := append(slides, model.Slide{ID: "s2", ProjectID: "p1", Idx: 2, Layout: "cta", Title: "行动", JSONPath: "slides/002/slide.json", HTMLPath: "x"})
	// idx 需唯一：重排为 0,1,2。
	three[1].Idx = 2
	three[1].Layout = "cta"
	three[2].Idx = 1
	three[2].Layout = "bullets"
	if err := s.ReplaceSlides(ctx, "p1", three); err != nil {
		t.Fatalf("replace2: %v", err)
	}
	got2, _ := s.ListSlides(ctx, "p1")
	if len(got2) != 3 {
		t.Fatalf("want 3 after replace, got %d", len(got2))
	}
}

func TestGetSlideByID(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()

	slides := []model.Slide{
		{ID: "s0", ProjectID: "p1", Idx: 0, Layout: "cover", Title: "封面", JSONPath: "slides/000/slide.json", HTMLPath: "slides/000/index.html"},
		{ID: "s1", ProjectID: "p1", Idx: 1, Layout: "thanks", Title: "谢谢", JSONPath: "slides/001/slide.json", HTMLPath: "slides/001/index.html"},
	}
	if err := s.ReplaceSlides(ctx, "p1", slides); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := s.GetSlide(ctx, "s1")
	if err != nil {
		t.Fatalf("get slide: %v", err)
	}
	if got.Idx != 1 || got.ProjectID != "p1" || got.HTMLPath != "slides/001/index.html" {
		t.Fatalf("unexpected slide: %+v", got)
	}

	// 不存在的 id MUST 返回 gorm.ErrRecordNotFound（handler 映射 404）。
	if _, err := s.GetSlide(ctx, "missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound, got %v", err)
	}
}

func TestVersionNoMonotonic(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()

	for want := 0; want < 3; want++ {
		no, err := s.NextVersionNo(ctx, "project", "p1")
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if no != want {
			t.Fatalf("want version %d, got %d", want, no)
		}
		if err := s.CreateVersion(ctx, model.Version{
			ID: "v" + itoaLocal(no), TargetType: "project", TargetID: "p1", VersionNo: no,
			SnapshotPath: "versions/project/v" + itoaLocal(no) + ".json", CreatedAt: 1,
		}); err != nil {
			t.Fatalf("create version: %v", err)
		}
	}
	vs, err := s.ListVersions(ctx, "project", "p1")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(vs) != 3 {
		t.Fatalf("want 3 versions, got %d", len(vs))
	}
}

func TestSetProjectStatus(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	if err := s.SetProjectStatus(ctx, "p1", "ready"); err != nil {
		t.Fatalf("set status: %v", err)
	}
	p, _ := s.GetProject(ctx, "p1")
	if p.Status != "ready" {
		t.Errorf("want ready, got %q", p.Status)
	}
}

func TestSlideOrderAndDirtyRoundTrip(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	err := s.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "s1", ProjectID: "p1", Order: 10, OutlineDirty: true, Layout: "cover", Title: "A",
			JSONPath: "slides/s1/slide.json", HTMLPath: "slides/s1/index.html"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSlide(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Order != 10 || !got.OutlineDirty {
		t.Fatalf("round trip lost fields: %+v", got)
	}
}

func itoaLocal(n int) string {
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
