package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

func TestSnapshotRestoresProjectStateAndInvalidatesOldWorker(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject(ctx, model.Project{ID: "other", Title: "other", Theme: "theme", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec(`INSERT INTO resources VALUES('snippet','global','global','global','library',0,1,1)`).Error; err != nil {
		t.Fatal(err)
	}
	raw, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string][]map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 7 {
		t.Fatalf("project inventory: %v", snapshot)
	}
	if _, exists := snapshot["thread_event_outbox"]; exists {
		t.Fatal("outbox entered history")
	}
	if err := s.db.Exec(`UPDATE projects SET title='changed' WHERE id='p'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreProject(ctx, "p", raw); err != nil {
		t.Fatal(err)
	}
	project, err := s.GetProject(ctx, "p")
	if err != nil || project.Title != "project" {
		t.Fatalf("project=%+v err=%v", project, err)
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil || current.Status != model.RunPaused || current.ExecutionRevision != 2 {
		t.Fatalf("run=%+v err=%v", current, err)
	}
	cp.CheckpointRevision = 1
	if err := s.SaveCheckpoint(ctx, cp); !errors.Is(err, run.ErrRunRevisionConflict) {
		t.Fatalf("old checkpoint accepted: %v", err)
	}
	if _, err := s.GetProject(ctx, "other"); err != nil {
		t.Fatal("other project removed")
	}
	var count int64
	if err := s.db.Table("resources").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("global resources changed: %d %v", count, err)
	}
	snapshot["runs"][0]["status"] = "invalid"
	broken, _ := json.Marshal(snapshot)
	if err := s.RestoreProject(ctx, "p", broken); err == nil {
		t.Fatal("invalid snapshot accepted")
	}
	current, err = s.GetRun(ctx, "r")
	if err != nil || current.ExecutionRevision != 2 {
		t.Fatalf("failed restore changed state: %+v %v", current, err)
	}
}
