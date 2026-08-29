package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	cfg := &config.Config{DBPath: filepath.Join(t.TempDir(), "test.db")}
	db, cleanup, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	s, err := NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store (migrate): %v", err)
	}
	return s
}

func TestMigrateCreatesTables(t *testing.T) {
	s := newTestStore(t)

	want := []string{"projects", "slides", "versions", "threads", "runs", "run_events", "run_contexts", "assets"}
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
		"INSERT INTO projects (id,title,work_dir,created_at,updated_at) VALUES (?,?,?,?,?)",
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

func TestMigrateIdempotent(t *testing.T) {
	cfg := &config.Config{DBPath: filepath.Join(t.TempDir(), "idem.db")}
	db, cleanup, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	if err := Migrate(db, zap.NewNop()); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(db, zap.NewNop()); err != nil {
		t.Fatalf("second migrate (should be idempotent): %v", err)
	}
}

func TestMigrateLegacyDeckProjectToManifest(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "project")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "deck.json"), []byte(`{"version":"4.0","revision":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DBPath: filepath.Join(root, "legacy.db")}
	db, cleanup, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	for _, statement := range []string{
		`CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`,
		`INSERT INTO schema_migrations(name, applied_at) VALUES
			('0001_init.sql', 1),
			('0002_run_pause_lifecycle.sql', 1),
			('0003_add_deck_version_target.sql', 1)`,
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			work_dir TEXT NOT NULL,
			layout_version INTEGER NOT NULL
		)`,
		`INSERT INTO projects(id, work_dir, layout_version) VALUES ('p1', '` + workDir + `', 6)`,
		`CREATE TABLE versions (
			id TEXT PRIMARY KEY,
			target_type TEXT NOT NULL CHECK (target_type IN ('deck','outline','slide_spec','slide_html','design','asset')),
			target_id TEXT NOT NULL,
			version_no INTEGER NOT NULL,
			snapshot_path TEXT NOT NULL,
			run_id TEXT,
			created_at INTEGER NOT NULL,
			UNIQUE (target_type, target_id, version_no)
		)`,
		`CREATE INDEX idx_versions_target ON versions(target_type, target_id)`,
		`CREATE UNIQUE INDEX idx_versions_run_target ON versions(run_id, target_type, target_id) WHERE run_id IS NOT NULL`,
		`INSERT INTO versions(id, target_type, target_id, version_no, snapshot_path, created_at)
			VALUES ('v1', 'deck', 'p1', 1, 'versions/deck/v1.json', 1)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare legacy database: %v", err)
		}
	}

	if err := Migrate(db, zap.NewNop()); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	var targetType string
	if err := db.Raw("SELECT target_type FROM versions WHERE id = 'v1'").Scan(&targetType).Error; err != nil {
		t.Fatal(err)
	}
	if targetType != "manifest" {
		t.Fatalf("target_type=%q want manifest", targetType)
	}
	if _, err := os.Stat(filepath.Join(workDir, "manifest.json")); err != nil {
		t.Fatalf("manifest.json was not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "deck.json")); !os.IsNotExist(err) {
		t.Fatalf("deck.json still exists after migration: %v", err)
	}
	if err := Migrate(db, zap.NewNop()); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
}

func TestRunContextRoundTripStoresManifestOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "p", WorkDir: "/tmp/p", Status: "draft", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", HistoryPath: "threads/t.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	runModel := model.Run{ID: "r", ThreadID: "t", ProjectID: "p", Command: model.RunCommand{
		Scope: model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeDeck},
		Mode:  model.ModeExecute, Instruction: "x",
	}, Status: model.RunPending, CreatedAt: 1, UpdatedAt: 1}
	if err := s.CreateRun(ctx, runModel); err != nil {
		t.Fatal(err)
	}
	want := model.RunContext{RunID: "r", ContextID: "ctx_1", Profile: "spec/deck", PackHash: "hash", EstimatedTokens: 10, BudgetTokens: 100, ManifestJSON: `{"segments":[]}`, CreatedAt: 1}
	if err := s.SaveRunContext(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRunContext(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}
