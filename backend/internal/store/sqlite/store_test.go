package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{WorkRoot: root, DBPath: filepath.Join(root, "test.db")}
	db, cleanup, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	s, err := NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	return s
}

func TestSchemaCreatesTables(t *testing.T) {
	s := newTestStore(t)

	want := []string{"projects", "slides", "threads", "runs", "steering_inbox", "idempotency_records", "resources", "tags", "resource_tags", "shortcut_settings", "command_executions", "thread_event_outbox"}
	for _, name := range want {
		var count int64
		if err := s.db.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
		).Scan(&count).Error; err != nil {
			t.Fatalf("query table %s: %v", name, err)
		}
		if count != 1 {
			t.Errorf("table %s: want 1, got %d", name, count)
		}
	}
}

func TestForeignKeyEnforced(t *testing.T) {
	s := newTestStore(t)

	// 插入引用不存在 project 的 slide，外键开启时必须失败。
	err := s.db.Exec(
		"INSERT INTO slides (id,project_id) VALUES (?,?)",
		"s1", "does-not-exist",
	).Error
	if err == nil {
		t.Fatal("expected foreign key violation, got nil")
	}
}

func TestCascadeDelete(t *testing.T) {
	s := newTestStore(t)

	if err := s.db.Exec(
		"INSERT INTO projects (id,title,theme,created_at,updated_at) VALUES (?,?,?,?,?)",
		"p1", "t", "/w", 1, 1,
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := s.db.Exec(
		"INSERT INTO slides (id,project_id) VALUES (?,?)",
		"s1", "p1",
	).Error; err != nil {
		t.Fatalf("insert slide: %v", err)
	}
	if err := s.db.Exec("DELETE FROM projects WHERE id=?", "p1").Error; err != nil {
		t.Fatalf("delete project: %v", err)
	}

	var remaining int64
	if err := s.db.Raw("SELECT COUNT(*) FROM slides").Scan(&remaining).Error; err != nil {
		t.Fatalf("count slides: %v", err)
	}
	if remaining != 0 {
		t.Errorf("cascade delete failed: %d slides remain", remaining)
	}
}

func TestHealth(t *testing.T) {
	s := newTestStore(t)
	if err := s.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
}

func TestSchemaInitializationIdempotent(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{WorkRoot: root, DBPath: filepath.Join(root, "idem.db")}
	db, cleanup, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	if err := initializeSchema(db); err != nil {
		t.Fatalf("first initialization: %v", err)
	}
	if err := initializeSchema(db); err != nil {
		t.Fatalf("second initialization (should be idempotent): %v", err)
	}
}
