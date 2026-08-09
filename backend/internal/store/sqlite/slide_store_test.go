package sqlite

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func activeRunSpec() model.WorkSpec {
	return model.WorkSpec{
		Target:      model.RunTarget{Artifact: model.ArtifactSpec, Level: model.TargetDeck},
		Interaction: model.RunInteraction{Intent: model.IntentExecute},
		Instruction: "test",
	}
}

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
		{ID: "s0", ProjectID: "p1", SpecRevision: 1},
		{ID: "s1", ProjectID: "p1", SpecRevision: 1},
	}
	if err := s.ReplaceSlides(ctx, "p1", slides); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := s.ListSlides(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// DB no longer stores order/layout; paths are derived from the stable id.
	if len(got) != 2 || got[0].SpecPath != "slides/s0/spec.json" || got[1].HTMLPath != "slides/s1/index.html" {
		t.Fatalf("bad slides: %+v", got)
	}

	// 幂等替换：再次以 3 页替换，旧的被清掉。
	three := append(slides, model.Slide{ID: "s2", ProjectID: "p1"})
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
		{ID: "s0", ProjectID: "p1"},
		{ID: "s1", ProjectID: "p1"},
	}
	if err := s.ReplaceSlides(ctx, "p1", slides); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := s.GetSlide(ctx, "s1")
	if err != nil {
		t.Fatalf("get slide: %v", err)
	}
	if got.ProjectID != "p1" || got.HTMLPath != "slides/s1/index.html" {
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
		no, err := s.NextVersionNo(ctx, "outline", "p1")
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if no != want {
			t.Fatalf("want version %d, got %d", want, no)
		}
		if err := s.CreateVersion(ctx, model.Version{
			ID: "v" + itoaLocal(no), TargetType: "outline", TargetID: "p1", VersionNo: no,
			SnapshotPath: "versions/spec-deck/v" + itoaLocal(no) + ".json", CreatedAt: 1,
		}); err != nil {
			t.Fatalf("create version: %v", err)
		}
	}
	vs, err := s.ListVersions(ctx, "outline", "p1")
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

func TestListSlidesReturnsMembership(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	_ = s.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "b", ProjectID: "p1"},
		{ID: "a", ProjectID: "p1"},
	})
	got, _ := s.ListSlides(ctx, "p1")
	// Order is file-projected; the store returns membership deterministically by id.
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("expected membership a,b got %+v", got)
	}
}

func TestHasActiveRun(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	if err := s.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(ctx, model.Run{ID: "r1", ProjectID: "p1", ThreadID: "t1", WorkSpec: activeRunSpec(), Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
	on, err := s.HasActiveRun(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("expected active")
	}
	if err := s.SetRunStatus(ctx, "r1", model.RunDone); err != nil {
		t.Fatal(err)
	}
	off, _ := s.HasActiveRun(ctx, "p1")
	if off {
		t.Fatal("expected inactive after done")
	}
}

func TestInsertDeleteMembership(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	_ = s.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1"},
		{ID: "b", ProjectID: "p1"}})
	if err := s.InsertSlide(ctx, model.Slide{ID: "c", ProjectID: "p1"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, _ := s.ListSlides(ctx, "p1")
	if len(got) != 3 {
		t.Fatalf("insert wrong: %+v", got)
	}
	// SetSlidesOrder is now a no-op (order lives in outline.json); it must succeed
	// and leave membership intact.
	if err := s.SetSlidesOrder(ctx, "p1", map[string]int{"a": 30, "b": 20, "c": 10}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if err := s.DeleteSlideByID(ctx, "b"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = s.ListSlides(ctx, "p1")
	if len(got) != 2 {
		t.Fatalf("delete wrong: %+v", got)
	}
	for _, sl := range got {
		if sl.ID == "b" {
			t.Fatalf("deleted slide still present: %+v", got)
		}
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
