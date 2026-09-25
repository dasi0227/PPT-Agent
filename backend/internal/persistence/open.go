// Package persistence orders workspace recovery before consumers inspect history.
package persistence

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewStore(db *gorm.DB, log *zap.Logger) (*sqlite.Store, error) {
	var manager *projecthistory.Manager
	st, err := sqlite.NewStoreWithRecovery(db, log, func(s *sqlite.Store, root string) error {
		// A torn whole-project restore can temporarily disagree with the journal
		// watermark. Restore its preimage before validating or delivering any log.
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
		if err := manager.CollectPayloads(context.Background(), project); err != nil {
			return nil, err
		}
	}
	return st, nil
}
