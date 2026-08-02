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
		Target:      model.RunTarget{Artifact: model.ArtifactBlueprint, Level: model.TargetDeck},
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
		{ID: "s0", ProjectID: "p1", Position: 0, Layout: "cover", Title: "封面", JSONPath: "slides/s0/slide.json", HTMLPath: "slides/s0/index.html"},
		{ID: "s1", ProjectID: "p1", Position: 1, Layout: "thanks", Title: "谢谢", JSONPath: "slides/s1/slide.json", HTMLPath: "slides/s1/index.html"},
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
	three := append(slides, model.Slide{ID: "s2", ProjectID: "p1", Position: 2, Layout: "cta", Title: "行动", JSONPath: "slides/s2/slide.json", HTMLPath: "x"})
	three[1].Position = 2
	three[1].Layout = "cta"
	three[2].Position = 1
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
		{ID: "s0", ProjectID: "p1", Position: 0, Layout: "cover", Title: "封面", JSONPath: "slides/s0/slide.json", HTMLPath: "slides/s0/index.html"},
		{ID: "s1", ProjectID: "p1", Position: 1, Layout: "thanks", Title: "谢谢", JSONPath: "slides/s1/slide.json", HTMLPath: "slides/s1/index.html"},
	}
	if err := s.ReplaceSlides(ctx, "p1", slides); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := s.GetSlide(ctx, "s1")
	if err != nil {
		t.Fatalf("get slide: %v", err)
	}
	if got.Position != 1 || got.ProjectID != "p1" || got.HTMLPath != "slides/s1/index.html" {
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
		no, err := s.NextVersionNo(ctx, "blueprint_deck", "p1")
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if no != want {
			t.Fatalf("want version %d, got %d", want, no)
		}
		if err := s.CreateVersion(ctx, model.Version{
			ID: "v" + itoaLocal(no), TargetType: "blueprint_deck", TargetID: "p1", VersionNo: no,
			SnapshotPath: "versions/blueprint-deck/v" + itoaLocal(no) + ".json", CreatedAt: 1,
		}); err != nil {
			t.Fatalf("create version: %v", err)
		}
	}
	vs, err := s.ListVersions(ctx, "blueprint_deck", "p1")
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

func TestSlideRevisionMetadataRoundTrip(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	err := s.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "s1", ProjectID: "p1", Position: 10, Layout: "cover", Title: "A",
			BlueprintRevision: 2, PresentationRevision: 3, SourceDeckRevision: 1,
			SourceBlueprintRevision: 2, SourceDesignRevision: 1,
			JSONPath: "slides/s1/slide.json", HTMLPath: "slides/s1/index.html"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSlide(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Position != 10 || got.BlueprintRevision != 2 || got.PresentationRevision != 3 {
		t.Fatalf("round trip lost fields: %+v", got)
	}
}

func TestListSlidesOrderedByPosition(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	_ = s.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "b", ProjectID: "p1", Position: 20, Layout: "content", Title: "B"},
		{ID: "a", ProjectID: "p1", Position: 10, Layout: "cover", Title: "A"},
	})
	got, _ := s.ListSlides(ctx, "p1")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("expected order a,b got %+v", got)
	}
}

func TestUpdateMetaAndRevisions(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	_ = s.ReplaceSlides(ctx, "p1", []model.Slide{{ID: "s1", ProjectID: "p1", Position: 10, Layout: "cover", Title: "A"}})
	if err := s.UpdateSlideMeta(ctx, "s1", "B", "content"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSlideRevisions(ctx, "s1", 2, 3, 1, 2, 1); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSlide(ctx, "s1")
	if got.Title != "B" || got.Layout != "content" || got.BlueprintRevision != 2 || got.PresentationRevision != 3 {
		t.Fatalf("got %+v", got)
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

func TestInsertDeleteReorder(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	_ = s.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1", Position: 10, Layout: "cover", Title: "A"},
		{ID: "b", ProjectID: "p1", Position: 20, Layout: "thanks", Title: "B"}})
	if err := s.InsertSlide(ctx, model.Slide{ID: "c", ProjectID: "p1", Position: 15, Layout: "content", Title: "C"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, _ := s.ListSlides(ctx, "p1")
	if len(got) != 3 || got[1].ID != "c" {
		t.Fatalf("insert/order wrong: %+v", got)
	}
	if err := s.SetSlidesOrder(ctx, "p1", map[string]int{"a": 30, "b": 20, "c": 10}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	got, _ = s.ListSlides(ctx, "p1")
	if got[0].ID != "c" || got[2].ID != "a" {
		t.Fatalf("reorder wrong: %+v", got)
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
