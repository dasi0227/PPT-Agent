package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// TestCreateRunRejectsWhenProjectHasActiveRun verifies Phase B point 6: with the
// staging sandbox gone, runs are serialized per project. A create attempt while
// another run is active is rejected with ErrRunActive (mapped to 409 RUN_ACTIVE).
func TestCreateRunRejectsWhenProjectHasActiveRun(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "run.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.CreateProject(ctx, model.Project{
		ID: "p1", Title: "P", WorkDir: filepath.Join(root, "p1"),
		Theme: "default", Status: "draft", OutlineRevision: 1, DesignRevision: 1,
		LayoutVersion: 2, CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{
		ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	spec := model.WorkSpec{
		Target:      model.RunTarget{Artifact: model.ArtifactSpec, Level: model.TargetDeck},
		Interaction: model.RunInteraction{Intent: model.IntentExecute},
		Instruction: "build the deck",
	}
	if err := st.CreateRun(ctx, model.Run{
		ID: "active-run", ThreadID: "t1", ProjectID: "p1", WorkSpec: spec, Status: model.RunRunning,
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewRunServiceWithExecutionFactory(st, nil, nil)
	_, err = svc.CreateRun(ctx, "t1", model.CreateRunParams{
		ClientRequestID: "req-1", WorkSpec: spec,
	})
	if !errors.Is(err, ErrRunActive) {
		t.Fatalf("expected ErrRunActive while a run is active, got %v", err)
	}

	// After the active run reaches a terminal state, creation proceeds past the
	// serialization gate (and fails later only for the unsupported test target).
	if err := st.SetRunStatus(ctx, "active-run", model.RunDone); err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateRun(ctx, "t1", model.CreateRunParams{
		ClientRequestID: "req-2", WorkSpec: spec,
	})
	if errors.Is(err, ErrRunActive) {
		t.Fatalf("run creation still blocked after the active run finished: %v", err)
	}
}
