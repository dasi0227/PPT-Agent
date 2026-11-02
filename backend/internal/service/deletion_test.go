package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
)

func TestDeletionWorkerRecoversRemainingWindowAfterRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, cleanup, err := sqlite.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "test.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	st, err := sqlite.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject(ctx, model.Project{ID: "p", Title: "Retained"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p"}); err != nil {
		t.Fatal(err)
	}
	projects, threads := &ProjectService{store: st}, &ThreadService{store: st}
	newService := func() *DeletionService {
		return NewDeletionService(projects, threads, root, zap.NewNop(), func(context.Context, store.Deletion) error { return nil }, func(string) func() { return func() {} })
	}
	first := newService()
	target, err := first.DeleteProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := first.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	second := newService()
	defer second.Shutdown(ctx)
	if err := second.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	pending, err := second.Pending(ctx)
	if err != nil || len(pending) != 1 || pending[0].Token != target.Token || pending[0].UndoUntil != target.UndoUntil {
		t.Fatalf("restart reset the undo window: %+v %v", pending, err)
	}
	if _, err := second.Restore(ctx, "project", "p", target.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetProject(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetThread(ctx, "t"); err != nil {
		t.Fatal(err)
	}
}
