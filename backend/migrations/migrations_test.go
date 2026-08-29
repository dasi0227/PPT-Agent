package migrations

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestContentRevisionsLiveInFiles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	if err := applyAllMigrations(t, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	cols := tableColumns(t, db, "slides")
	for _, want := range []string{"id", "project_id", "current_version", "last_export_at"} {
		if !cols[want] {
			t.Fatalf("slides table missing column %q; got %v", want, cols)
		}
	}
	for _, removed := range []string{
		"idx", "order", "outline_dirty", "position", "layout", "title", "spec_path", "html_path",
		"spec_revision", "html_revision", "source_outline_revision", "source_spec_revision", "source_design_revision",
	} {
		if cols[removed] {
			t.Fatalf("legacy slides column %q still exists", removed)
		}
	}
	projectCols := tableColumns(t, db, "projects")
	for _, removed := range []string{"design_path", "outline_path", "outline_revision", "design_revision"} {
		if projectCols[removed] {
			t.Fatalf("legacy projects column %q still exists", removed)
		}
	}
	if err := db.Exec(`
		INSERT INTO projects(id,title,work_dir,theme,status,layout_version,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)
	`, "layout-v6", "Deck", "/tmp/layout-v6", "default", "draft", 6, 1, 1).Error; err != nil {
		t.Fatalf("layout version 6 is not accepted: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO versions(id,target_type,target_id,version_no,snapshot_path,created_at)
		VALUES(?,?,?,?,?,?)
	`, "manifest-version", "manifest", "layout-v6", 0, "versions/manifest/v0.json", 1).Error; err != nil {
		t.Fatalf("manifest version target is not accepted: %v", err)
	}
	runCols := tableColumns(t, db, "runs")
	for _, want := range []string{"owner_instance_id", "pause_reason", "paused_at"} {
		if !runCols[want] {
			t.Fatalf("runs table missing lifecycle column %q; got %v", want, runCols)
		}
	}
	if err := db.Exec(`
		INSERT INTO threads(id,project_id,title,history_path,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?)
	`, "pause-thread", "layout-v6", "", "threads/pause-thread.jsonl", "active", 1, 1).Error; err != nil {
		t.Fatalf("insert thread: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO runs(id,thread_id,project_id,scope_artifact,scope_level,mode,run_command_json,status,paused_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
	`, "paused-run", "pause-thread", "layout-v6", "ppt", "deck", "execute", `{}`, "paused", 2, 1, 2).Error; err != nil {
		t.Fatalf("paused run status is not accepted: %v", err)
	}
}

// applyAllMigrations 执行 migrations/ 内全部 SQL（按 ; 切分，跳过注释/空白）。
func applyAllMigrations(t *testing.T, db *gorm.DB) error {
	t.Helper()
	files, err := Files()
	if err != nil {
		return err
	}
	for _, name := range files {
		if err := applyMigrationFile(db, name); err != nil {
			return err
		}
	}
	return nil
}

func applyMigrationFile(db *gorm.DB, name string) error {
	content, err := FS.ReadFile(name)
	if err != nil {
		return err
	}
	for _, chunk := range strings.Split(string(content), ";") {
		stmt := strings.TrimSpace(chunk)
		if stmt == "" {
			continue
		}
		hasSQL := false
		for _, line := range strings.Split(stmt, "\n") {
			l := strings.TrimSpace(line)
			if l != "" && !strings.HasPrefix(l, "--") {
				hasSQL = true
				break
			}
		}
		if !hasSQL {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// tableColumns 收集某表的列名集合（PRAGMA table_info）。
func tableColumns(t *testing.T, db *gorm.DB, table string) map[string]bool {
	t.Helper()
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(" + table + ")").Scan(&rows).Error; err != nil {
		t.Fatalf("pragma table_info(%s): %v", table, err)
	}
	cols := make(map[string]bool, len(rows))
	for _, r := range rows {
		cols[r.Name] = true
	}
	return cols
}
