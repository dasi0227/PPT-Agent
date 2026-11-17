package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
)

type serviceFixture struct {
	store   *sqlitestore.Store
	project model.Project
	thread  model.Thread
	locks   *run.LockManager
}

func newServiceFixture(t *testing.T) serviceFixture {
	t.Helper()
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.RuntimeConfig{WorkRoot: root, DBPath: filepath.Join(root, "test.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	project := model.Project{ID: "p1", Title: "Board narrative", WorkDir: filepath.Join(root, "projects", "p1", "artifacts"), Theme: "default", CreatedAt: 1, UpdatedAt: 1}
	thread := model.Thread{ID: "t1", ProjectID: project.ID, CreatedAt: 1, UpdatedAt: 1}
	if err := st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(context.Background(), thread); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project.WorkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return serviceFixture{store: st, project: project, thread: thread, locks: run.NewLockManager()}
}
