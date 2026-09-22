package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func TestCommandActivityRetrySurvivesRollbackAndIgnoresLateAttempt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO projects(id,title,work_dir,created_at,updated_at) VALUES ('p','project','/p',1,1)`,
		`INSERT INTO threads(id,project_id,history_path,created_at,updated_at) VALUES ('t','p','threads/t.jsonl',1,1)`,
	} {
		if err := s.db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	first := model.CommandActivity{ID: "polish:1", AttemptID: "first", ThreadID: "t", ProjectID: "p", Kind: "polish", Method: "auto", Status: "loading", Phase: -1, Request: json.RawMessage(`{"instruction":"原文"}`), CreatedAt: 1000, UpdatedAt: 1000}
	first, err := s.BeginCommandActivity(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginCommandActivity(ctx, first); !errors.Is(err, store.ErrCommandActivityConflict) {
		t.Fatalf("duplicate active attempt accepted: %v", err)
	}
	baseline, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	first.Status = "completed"
	first.Result = json.RawMessage(`{"title":"明确任务","content":"第一版"}`)
	if err = s.SaveCommandActivity(ctx, first); err != nil {
		t.Fatal(err)
	}
	// A failed authoring mutation restores a baseline without in-flight rows.
	if err = s.RestoreProject(ctx, "p", baseline); err != nil {
		t.Fatal(err)
	}
	first.Status = "canceled"
	if err = s.SaveCommandActivity(ctx, first); err != nil {
		t.Fatal(err)
	}
	next := first
	next.AttemptID = "second"
	next.Status = "loading"
	next.CreatedAt = 2000
	next, err = s.BeginCommandActivity(ctx, next)
	if err != nil {
		t.Fatal(err)
	}
	if next.CreatedAt != 1000 || string(next.Result) != string(first.Result) {
		t.Fatal("retry lost original position or prior result")
	}
	next.Status = "completed"
	next.Result = json.RawMessage(`{"title":"明确任务","content":"第二版"}`)
	if err = s.SaveCommandActivity(ctx, next); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveCommandActivity(ctx, first); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListThreadCommandActivities(ctx, "t")
	if err != nil || len(rows) != 1 || rows[0].Status != "completed" || string(rows[0].Result) != string(next.Result) {
		t.Fatalf("late attempt overwrote retry: %+v %v", rows, err)
	}
}
