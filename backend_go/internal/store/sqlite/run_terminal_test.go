package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

type terminalTestExecution func(context.Context, workflow.EventEmitter, run.Checkpointer, run.Prompter) workflow.StructuredOutcome

func (f terminalTestExecution) Run(ctx context.Context, em workflow.EventEmitter, cp run.Checkpointer, p run.Prompter) workflow.StructuredOutcome {
	return f(ctx, em, cp, p)
}

func terminalStoreForTest(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	if err := s.CreateProject(context.Background(), model.Project{ID: "p", Title: "Project", Theme: "theme", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(context.Background(), model.Thread{ID: "t", ProjectID: "p", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	return s
}
func waitStoredRunForTest(t *testing.T, ctx context.Context, s *Store, id string) model.Run {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := s.GetRun(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status.Terminal() || current.Status == model.RunPaused {
			return current
		}
		select {
		case <-ctx.Done():
			t.Fatal("run did not stop", ctx.Err())
			return model.Run{}
		case <-ticker.C:
		}
	}
}

func TestSchedulerTerminalWriteFailureLeavesRecoverableRun(t *testing.T) {
	s := terminalStoreForTest(t)
	if err := s.db.Exec(`CREATE TRIGGER fail_terminal BEFORE UPDATE OF status ON runs WHEN NEW.status='done' BEGIN SELECT RAISE(ABORT,'injected state failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	engine := run.NewEngine(s, run.NewLockManager(), zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	execution := terminalTestExecution(func(_ context.Context, em workflow.EventEmitter, _ run.Checkpointer, _ run.Prompter) workflow.StructuredOutcome {
		em.Emit(model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase("r"), MessageID: "final", Text: "完成", AffectedTargets: []model.PublicTarget{}})
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})
	if _, err := engine.Start(ctx, model.Run{ID: "r", ThreadID: "t", ProjectID: "p", SceneRevision: 1, ClientRequestID: "input", Command: activeRunSpec()}, execution); err != nil {
		t.Fatal(err)
	}
	stored := waitStoredRunForTest(t, ctx, s, "r")
	if stored.Status != model.RunPaused {
		t.Fatalf("failed finalization not recoverable: %+v", stored)
	}
	var count int64
	if err := s.reader.Table("timeline_items").Where("group_id='run:r' AND kind='terminal_notice'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("uncommitted terminal published: %d %v", count, err)
	}
	if _, err := engine.RequestCancelWithReason(sessionstate.WithRequestScene(ctx, 1), "r", model.RunCancelUserRequested, currentRunAction(t, s, "r", "cancel")); err != nil {
		t.Fatal(err)
	}
	if active, err := s.HasActiveRun(ctx, "p"); err != nil || active {
		t.Fatalf("paused cancel retained lock: %v %v", active, err)
	}
}
