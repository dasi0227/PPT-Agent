package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// Store 是基于 GORM 的 store.Store 实现。PO 与 GORM tag 仅存在于本包（ARCH-BACKEND-006）。
type Store struct {
	instanceID      string
	db              *gorm.DB
	reader          *gorm.DB
	workRoot        string
	changeMu        sync.Mutex
	changeListeners map[string]map[chan struct{}]struct{}
}

var _ store.Store = (*Store)(nil)

// NewStore accepts only the current format. Current state is already durable;
// startup recovery handles interrupted executions and project file operations.
func NewStore(db *gorm.DB) (*Store, error) {
	return NewStoreWithRecovery(db, nil)
}
func NewStoreWithRecovery(db *gorm.DB, recoverProject func(*Store, string) error) (*Store, error) {
	if err := initializeSchema(db); err != nil {
		return nil, err
	}
	workRoot, err := workRootFromDB(db)
	if err != nil {
		return nil, err
	}
	db = db.Session(&gorm.Session{QueryFields: true})
	s := &Store{instanceID: model.MustShortID("store"), db: db, reader: db, workRoot: workRoot, changeListeners: make(map[string]map[chan struct{}]struct{})}
	if reader, ok := db.Get("ppt.reader"); ok {
		s.reader = reader.(*gorm.DB).Session(&gorm.Session{QueryFields: true})
	}
	if recoverProject != nil {
		if err := recoverProject(s, workRoot); err != nil {
			return nil, err
		}
	}
	if err := s.interruptCommands(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Health(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec("SELECT 1").Error
}

func workRootFromDB(db *gorm.DB) (string, error) {
	value, _ := db.Get("ppt.work_root")
	root, _ := value.(string)
	if root == "" {
		return "", errors.New("store requires a work directory")
	}
	return root, nil
}

func (s *Store) projectRoot(projectID string) string {
	return filepath.Join(s.workRoot, "projects", projectID)
}

// A pending operation may be waiting for deletion undo before recovery. Do not
// claim its saved workers until both the files and database have settled.
const settledHistoryProjects = "project_id IN (SELECT project_id FROM project_history_state WHERE switch_state = 'ready' AND pending_operation_json IS NULL)"
