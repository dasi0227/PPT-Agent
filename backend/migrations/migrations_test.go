package migrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 迁移文件 0001_init.sql MUST 与 docs 权威 schema 的 DDL 保持一致（DEV-RULES R1）。
// 比较时忽略注释与空白，只对齐可执行 SQL 语句集合。
func TestMigrationMatchesDocsSchema(t *testing.T) {
	migration, err := FS.ReadFile("0001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	docsPath := filepath.Join("..", "..", "docs", "v1", "30-data-model", "sqlite-schema.sql")
	docs, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read docs schema %s: %v", docsPath, err)
	}

	got := normalize(string(migration))
	want := normalize(string(docs))
	if got != want {
		t.Errorf("migration DDL diverged from docs schema (source of truth).\nmigration:\n%s\n\ndocs:\n%s", got, want)
	}
}

// normalize 去除注释、空行并压缩空白，得到可比较的 DDL 规范形。
func normalize(sql string) string {
	var b strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		if i := strings.Index(t, "--"); i >= 0 {
			t = strings.TrimSpace(t[:i])
		}
		b.WriteString(strings.Join(strings.Fields(t), " "))
		b.WriteString("\n")
	}
	return b.String()
}

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
	`, "layout-v5", "Deck", "/tmp/layout-v5", "default", "draft", 5, 1, 1).Error; err != nil {
		t.Fatalf("layout version 5 is not accepted: %v", err)
	}
}

func TestRunCommandMigrationConvertsPersistedRuns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if name == "0011_run_command_contract.sql" {
			break
		}
		if err := applyMigrationFile(db, name); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	if err := db.Exec(`
		INSERT INTO projects(id,title,work_dir,theme,status,layout_version,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)
	`, "p1", "Deck", "/tmp/p1", "default", "draft", 5, 1, 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO threads(id,project_id,title,history_path,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?)
	`, "t1", "p1", "Thread", "threads/t1.jsonl", "active", 1, 1).Error; err != nil {
		t.Fatal(err)
	}
	legacy := `{"target":{"artifact":"presentation","level":"deck"},"interaction":{"intent":"execute"},"instruction":"build","options":{"language":"en-US","theme_id":"legacy","desired_slide_count":12}}`
	if err := db.Exec(`
		INSERT INTO runs(
			id,thread_id,project_id,target_artifact,target_level,target_slide_id,
			interaction_intent,work_spec_json,client_request_id,status,created_at,updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
	`, "r1", "t1", "p1", "presentation", "deck", nil, "execute", legacy, "req1", "done", 1, 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := applyMigrationFile(db, "0011_run_command_contract.sql"); err != nil {
		t.Fatalf("apply command migration: %v", err)
	}

	columns := tableColumns(t, db, "runs")
	for _, want := range []string{"scope_artifact", "scope_level", "scope_slide_id", "intent", "run_command_json"} {
		if !columns[want] {
			t.Fatalf("runs table missing %q: %v", want, columns)
		}
	}
	for _, removed := range []string{"target_artifact", "target_level", "target_slide_id", "interaction_intent", "work_spec_json"} {
		if columns[removed] {
			t.Fatalf("legacy runs column %q still exists", removed)
		}
	}
	var row struct {
		Artifact string `gorm:"column:scope_artifact"`
		Intent   string `gorm:"column:intent"`
		Command  string `gorm:"column:run_command_json"`
	}
	if err := db.Raw(`SELECT scope_artifact,intent,run_command_json FROM runs WHERE id = ?`, "r1").Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Artifact != "ppt" || row.Intent != "execute" {
		t.Fatalf("projection not migrated: %+v", row)
	}
	var command map[string]any
	if err := json.Unmarshal([]byte(row.Command), &command); err != nil {
		t.Fatalf("invalid command JSON: %v\n%s", err, row.Command)
	}
	scope, _ := command["scope"].(map[string]any)
	options, _ := command["options"].(map[string]any)
	if scope["artifact"] != "ppt" || scope["level"] != "deck" ||
		command["intent"] != "execute" || command["instruction"] != "build" ||
		options["language"] != "en-US" || options["range"] != "9-15" {
		t.Fatalf("command not migrated: %+v", command)
	}
	if _, exists := options["theme_id"]; exists {
		t.Fatalf("legacy theme_id survived: %+v", options)
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
