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
	for _, want := range []string{"scope_object", "scope_slide_ids_json", "scope_source_json", "scope_include_run_created_slides", "scope_revision", "owner_instance_id", "pause_reason", "paused_at"} {
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
		INSERT INTO runs(id,thread_id,project_id,scope_object,scope_slide_ids_json,scope_source_json,scope_include_run_created_slides,scope_revision,mode,run_command_json,status,paused_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, "paused-run", "pause-thread", "layout-v6", "presentation", `[]`, `{"kind":"all_pages"}`, 1, 1, "execute", `{}`, "paused", 2, 1, 2).Error; err != nil {
		t.Fatalf("paused run status is not accepted: %v", err)
	}
	promptCols := tableColumns(t, db, "prompts")
	for _, want := range []string{"id", "name", "normalized_name", "desc", "value", "created_at", "updated_at"} {
		if !promptCols[want] {
			t.Fatalf("prompts table missing column %q; got %v", want, promptCols)
		}
	}
	for _, removed := range []string{"key_zh", "key_en", "normalized_key_en"} {
		if promptCols[removed] {
			t.Fatalf("legacy prompts column %q still exists", removed)
		}
	}
}

func TestPromptNameSchemaReplacesLegacyPromptData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	if err := applyMigrationFile(db, "0005_repository_metadata.sql"); err != nil {
		t.Fatalf("apply repository metadata migration: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE prompts (
			id TEXT PRIMARY KEY,
			key_zh TEXT NOT NULL UNIQUE,
			key_en TEXT NOT NULL,
			normalized_key_en TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`).Error; err != nil {
		t.Fatalf("create legacy prompts: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO prompts(id,key_zh,key_en,normalized_key_en,value,created_at,updated_at)
		VALUES('legacy-prompt','旧提示','legacy','legacy','旧内容',1,1)
	`).Error; err != nil {
		t.Fatalf("insert legacy prompt: %v", err)
	}
	if err := db.Exec(`INSERT INTO resource_tags(resource_type,resource_id,tag_id) VALUES('prompt','legacy-prompt','tag_prompt_other')`).Error; err != nil {
		t.Fatalf("insert legacy prompt tag: %v", err)
	}
	if err := db.Exec(`INSERT INTO resource_states(resource_type,resource_id,disabled,updated_at) VALUES('prompt','legacy-prompt',1,1)`).Error; err != nil {
		t.Fatalf("insert legacy prompt state: %v", err)
	}

	if err := applyMigrationFile(db, "0006_prompt_name_schema.sql"); err != nil {
		t.Fatalf("apply prompt name migration: %v", err)
	}
	cols := tableColumns(t, db, "prompts")
	for _, want := range []string{"id", "name", "normalized_name", "desc", "value", "created_at", "updated_at"} {
		if !cols[want] {
			t.Fatalf("prompts table missing column %q; got %v", want, cols)
		}
	}
	for table, query := range map[string]string{
		"prompts":         "SELECT COUNT(*) FROM prompts",
		"resource_tags":   "SELECT COUNT(*) FROM resource_tags WHERE resource_type = 'prompt'",
		"resource_states": "SELECT COUNT(*) FROM resource_states WHERE resource_type = 'prompt'",
	} {
		var count int64
		if err := db.Raw(query).Scan(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s legacy rows=%d err=%v", table, count, err)
		}
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
	var current strings.Builder
	inTrigger := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if current.Len() == 0 && (trimmed == "" || strings.HasPrefix(trimmed, "--")) {
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
		upper := strings.ToUpper(trimmed)
		if !inTrigger && strings.HasPrefix(upper, "CREATE TRIGGER") {
			inTrigger = true
		}
		if (!inTrigger && strings.HasSuffix(trimmed, ";")) || (inTrigger && upper == "END;") {
			stmt := strings.TrimSuffix(strings.TrimSpace(current.String()), ";")
			if err := db.Exec(stmt).Error; err != nil {
				return err
			}
			current.Reset()
			inTrigger = false
		}
	}
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
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
