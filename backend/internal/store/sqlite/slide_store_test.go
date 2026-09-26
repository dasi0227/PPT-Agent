package sqlite

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

func activeRunSpec() model.RunCommand {
	return model.RunCommand{
		Scope:       model.NewRunScope(model.ScopeAllPages),
		Mode:        model.ModeExecute,
		Instruction: "test",
	}
}

func seedProject(t *testing.T, s *Store) {
	t.Helper()
	if err := s.CreateProject(context.Background(), model.Project{
		ID: "p1", Title: "t", WorkDir: "/tmp/p1", Theme: "default", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
}

func TestReplaceSlidesAndList(t *testing.T) {
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
	got, err := s.ListSlides(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].ID != "s0" || got[1].ID != "s1" {
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
	if got.ProjectID != "p1" || got.ID != "s1" {
		t.Fatalf("unexpected slide: %+v", got)
	}

	// 不存在的 id MUST 返回 gorm.ErrRecordNotFound（handler 映射 404）。
	if _, err := s.GetSlide(ctx, "missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound, got %v", err)
	}
}

func TestCommitWorkflowAtomicallyRecordsMutationReceiptAndToolResult(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(ctx, model.Run{ID: "run", ThreadID: "t", ProjectID: "p1", Command: activeRunSpec(), Status: model.RunRunning, CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.AcquireIdempotency(ctx, model.IdempotencyRecord{
		Scope: "tool_call", OwnerID: "run", Key: "call_1", RequestHash: "tool-request", Status: "in_progress",
	}); err != nil {
		t.Fatal(err)
	}
	commit := model.ArtifactCommit{
		ProjectID: "p1", RunID: "run", OperationID: "call_1",
		RequestHash: "mutation-request", ToolResultJSON: `{"ok":true}`,
	}
	if err := s.CommitWorkflow(ctx, commit); err != nil {
		t.Fatal(err)
	}
	receipt, err := s.GetIdempotency(ctx, "artifact_commit", "run", "call_1")
	if err != nil || receipt.RequestHash != "mutation-request" || receipt.Status != "completed" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	toolCall, err := s.GetIdempotency(ctx, "tool_call", "run", "call_1")
	if err != nil || toolCall.Status != "completed" || toolCall.ResultJSON != `{"ok":true}` {
		t.Fatalf("tool call=%+v err=%v", toolCall, err)
	}
	commit.RequestHash = "conflicting-request"
	if err := s.CommitWorkflow(ctx, commit); err == nil {
		t.Fatal("conflicting operation receipt was accepted")
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
	if err := s.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(ctx, model.Run{ID: "r1", ProjectID: "p1", ThreadID: "t1", Command: activeRunSpec(), Status: model.RunRunning}); err != nil {
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

func TestSlideMembershipDeletionAndReceiptAreAtomic(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s)
	ctx := context.Background()
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(ctx, model.Run{ID: "run", ThreadID: "t", ProjectID: "p1", Command: activeRunSpec(), Status: model.RunRunning, CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	slide := model.Slide{ID: "pending", ProjectID: "p1"}
	if err := s.InsertSlide(ctx, slide); err != nil {
		t.Fatal(err)
	}
	commit := model.ArtifactCommit{ProjectID: "p1", RunID: "run", OperationID: "delete", RequestHash: "hash", DeletedSlideIDs: []string{slide.ID}, Slides: []model.Slide{{ID: "invalid", ProjectID: "missing-project"}}}
	if err := s.CommitWorkflow(ctx, commit); err == nil {
		t.Fatal("invalid commit accepted")
	}
	if _, err := s.GetSlide(ctx, slide.ID); err != nil {
		t.Fatal("failed transaction lost slide", err)
	}
	if _, err := s.GetIdempotency(ctx, "artifact_commit", "run", "delete"); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("failed commit has receipt: %v", err)
	}
	commit.Slides = nil
	if err := s.CommitWorkflow(ctx, commit); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSlide(ctx, slide.ID); err == nil {
		t.Fatal("deleted slide remains in membership")
	}
	if err := s.InsertSlide(ctx, slide); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSlide(ctx, slide.ID); err != nil {
		t.Fatal("recreated slide missing", err)
	}
	if err := s.CommitWorkflow(ctx, commit); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSlide(ctx, slide.ID); err != nil {
		t.Fatal("late replay deleted recreated slide", err)
	}
	commit.RequestHash = "conflict"
	if err := s.CommitWorkflow(ctx, commit); err == nil {
		t.Fatal("conflicting replay accepted")
	}
	if _, err := s.GetSlide(ctx, slide.ID); err != nil {
		t.Fatal("conflicting replay changed metadata", err)
	}
}
