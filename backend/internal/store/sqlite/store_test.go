package sqlite

import (
	"context"
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
		"INSERT INTO slides (id,project_id,position,layout,json_path,html_path) VALUES (?,?,?,?,?,?)",
		"s1", "does-not-exist", 0, "cover", "a", "b",
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
		"INSERT INTO slides (id,project_id,position,layout,json_path,html_path) VALUES (?,?,?,?,?,?)",
		"s1", "p1", 0, "cover", "a", "b",
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

func TestRunContextRoundTripStoresManifestOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "p", WorkDir: "/tmp/p", Status: "draft", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", HistoryPath: "threads/t.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	runModel := model.Run{ID: "r", ThreadID: "t", ProjectID: "p", WorkSpec: model.WorkSpec{
		Target:      model.RunTarget{Artifact: model.ArtifactBlueprint, Level: model.TargetDeck},
		Interaction: model.RunInteraction{Intent: model.IntentApply, Clarification: model.ClarifyNever}, Instruction: "x",
	}, Status: model.RunPending, CreatedAt: 1, UpdatedAt: 1}
	if err := s.CreateRun(ctx, runModel); err != nil {
		t.Fatal(err)
	}
	want := model.RunContext{RunID: "r", ContextID: "ctx_1", Profile: "blueprint/deck", PackHash: "hash", EstimatedTokens: 10, BudgetTokens: 100, ManifestJSON: `{"segments":[]}`, CreatedAt: 1}
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
