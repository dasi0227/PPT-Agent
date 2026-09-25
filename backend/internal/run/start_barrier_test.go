package run

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

func TestStartBarrierPrecedesWorkerAndRejectsFailedAcceptance(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "failed"}[fail], func(t *testing.T) {
			store := newMemStore()
			engine := NewEngine(store, NewLockManager(), zap.NewNop())
			called := 0
			ctx := WithStartBarrier(context.Background(), func() error {
				called++
				if row, err := store.GetRun(context.Background(), "r1"); err != nil || row.Status != model.RunPending {
					t.Fatal("barrier ran before durable acceptance")
				}
				if fail {
					return errors.New("acceptance disk failure")
				}
				return nil
			})
			started := make(chan bool, 1)
			execution := scriptRunner(func(context.Context, workflow.EventEmitter, Checkpointer, Prompter) workflow.StructuredOutcome {
				started <- called == 1
				return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
			})
			_, err := engine.Start(ctx, testRun("r1"), execution)
			if called != 1 {
				t.Fatalf("barrier calls = %d", called)
			}
			if fail {
				if err == nil {
					t.Fatal("failed acceptance started worker")
				}
				row, _ := store.GetRun(context.Background(), "r1")
				if row.Status != model.RunFailed {
					t.Fatal("orphan active run")
				}
				select {
				case <-started:
					t.Fatal("worker ran after failed acceptance")
				default:
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			waitRunStatus(t, store, "r1", model.RunDone)
			if !<-started {
				t.Fatal("worker ran before barrier")
			}
		})
	}
}
