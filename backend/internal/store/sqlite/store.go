package sqlite

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

const currentProjectLayoutVersion = 6

// Store 是基于 GORM 的 store.Store 实现。PO 与 GORM tag 仅存在于本包（ARCH-BACKEND-006）。
type Store struct {
	db  *gorm.DB
	log *zap.Logger
}

var _ store.Store = (*Store)(nil)

// NewStore 打开后执行迁移（migrations/ 为 schema 权威源），返回可用的 store 实现。
func NewStore(db *gorm.DB, log *zap.Logger) (*Store, error) {
	if err := Migrate(db, log); err != nil {
		return nil, err
	}
	return &Store{db: db, log: log}, nil
}

func (s *Store) Health(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec("SELECT 1").Error
}
