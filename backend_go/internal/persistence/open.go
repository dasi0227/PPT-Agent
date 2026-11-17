// Package persistence orders workspace recovery before consumers inspect history.
package persistence

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"gorm.io/gorm"
	"log/slog"
)

func NewStore(db *gorm.DB) (*sqlite.Store, error) {
	var manager *projecthistory.Manager
	st, err := sqlite.NewStoreWithRecovery(db, func(s *sqlite.Store, root string) error {
		// Resolve durable project switches before claiming execution ownership.
		// A failed project stays isolated while unaffected projects may start.
		manager = projecthistory.New(s, run.NewLockManager(), root)
		return manager.Initialize(context.Background())
	})
	if err != nil {
		return nil, err
	}
	projects, err := st.ListProjects(context.Background())
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if err := manager.Collect(project.ID); err != nil {
			slog.Warn("project checkpoint cleanup deferred", "project_id", project.ID, "error", err)
		}
	}
	return st, nil
}
