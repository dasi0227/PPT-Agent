package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestBriefingCompletionIsAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.CreateProject(ctx, model.Project{ID: "p1", Title: "Deck", WorkDir: t.TempDir(), Theme: "default", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for number := 1; number <= 3; number++ {
		accepted, _, err := st.AcceptCommand(ctx, "t1", model.CommandRequest{RequestKey: fmt.Sprint("briefing-", number), Kind: "handoff", Input: json.RawMessage(`{}`)}, 1)
		if err != nil {
			t.Fatal(err)
		}
		execution := accepted.Command
		version := model.BriefingVersion{BriefingID: "b1", ThreadID: "t1", ProjectID: "p1", Kind: model.BriefingHandoff, VersionNo: number, Title: "启动任务", Content: "content", CreatedAt: int64(number)}
		execution.Status = "completed"
		execution.Result, _ = json.Marshal(map[string]any{"briefing": model.Briefing{BriefingID: "b1", ThreadID: "t1", ProjectID: "p1", Kind: model.BriefingHandoff, Versions: []model.BriefingVersion{version}}})
		effects := sessionstate.CommandEffects{Briefing: &version}
		if number == 2 {
			if err := st.db.Exec(`CREATE TRIGGER reject_terminal BEFORE UPDATE ON command_executions WHEN NEW.status = 'completed' BEGIN SELECT RAISE(ABORT,'failure'); END`).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := st.CompleteCommand(ctx, execution, effects); err == nil {
				t.Fatal("expected rollback")
			}
			latest, err := st.LatestBriefingVersion(ctx, "t1", "b1")
			if err != nil || latest.VersionNo != 1 {
				t.Fatalf("candidate escaped rollback: %+v %v", latest, err)
			}
			if err := st.db.Exec(`DROP TRIGGER reject_terminal`).Error; err != nil {
				t.Fatal(err)
			}
		}
		saved, err := st.CompleteCommand(ctx, execution, effects)
		if err != nil || saved.Status != "completed" {
			t.Fatalf("completion: %+v %v", saved, err)
		}
		retried, err := st.CompleteCommand(ctx, execution, effects)
		if err != nil || retried.UpdatedSeq != saved.UpdatedSeq {
			t.Fatalf("retry committed twice: %+v %v", retried, err)
		}
	}
	latest, err := st.LatestBriefingVersion(ctx, "t1", "b1")
	if err != nil || latest.VersionNo != 3 || latest.Title != "启动任务" {
		t.Fatalf("latest version: %+v %v", latest, err)
	}
	var count int64
	if err := st.db.Table("briefing_versions").Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("version count=%d error=%v", count, err)
	}
	accepted, _, err := st.AcceptCommand(ctx, "t1", model.CommandRequest{RequestKey: "late-candidate", Kind: "handoff", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	command := accepted.Command
	cancelCtx := sessionstate.WithEntityRevision(sessionstate.WithRequestScene(ctx, 1), command.EntityRevision)
	if _, err := st.RequestCommandCancel(cancelCtx, command.CommandID, command.AttemptID, "cancel-candidate"); err != nil {
		t.Fatal(err)
	}
	version := latest
	version.VersionNo = 4
	command.Status, command.Result = "completed", json.RawMessage(`{"briefing":{}}`)
	committed, err := st.CompleteCommand(ctx, command, sessionstate.CommandEffects{Briefing: &version})
	if err != nil || committed.Status != "canceled" || len(committed.Result) != 0 {
		t.Fatalf("late candidate won cancellation: %+v %v", committed, err)
	}
	latest, err = st.LatestBriefingVersion(ctx, "t1", "b1")
	if err != nil || latest.VersionNo != 3 {
		t.Fatalf("canceled version persisted: %+v %v", latest, err)
	}
}
