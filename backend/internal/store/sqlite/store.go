package sqlite

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// Store 是基于 GORM 的 store.Store 实现。PO 与 GORM tag 仅存在于本包（ARCH-BACKEND-006）。
type Store struct {
	instanceID       string
	db               *gorm.DB
	log              *zap.Logger
	workRoot         string
	deliveryLocks    sync.Map
	deliveryFailures sync.Map
}

var _ store.Store = (*Store)(nil)

// NewStore accepts only the current format and recovers committed log intents.
func NewStore(db *gorm.DB, log *zap.Logger) (*Store, error) {
	return NewStoreWithRecovery(db, log, nil)
}
func NewStoreWithRecovery(db *gorm.DB, log *zap.Logger, recoverProject func(*Store, string) error) (*Store, error) {
	if err := initializeSchema(db); err != nil {
		return nil, err
	}
	workRoot, err := workRootFromDB(db)
	if err != nil {
		return nil, err
	}
	s := &Store{instanceID: model.MustShortID("store"), db: db, log: log, workRoot: workRoot}
	if recoverProject != nil {
		if err := recoverProject(s, workRoot); err != nil {
			return nil, err
		}
	}
	if err := s.recoverOutbox(context.Background()); err != nil {
		return nil, err
	}
	if err := s.interruptCommands(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Health(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec("SELECT 1").Error
}
