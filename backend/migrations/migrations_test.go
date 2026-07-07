package migrations

import (
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

// TestSlidesHasOrderAndDirtyColumns 验证 slides 表含 order/outline_dirty 列（页身份重构地基）。
func TestSlidesHasOrderAndDirtyColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	if err := applyAllMigrations(t, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	cols := tableColumns(t, db, "slides")
	for _, want := range []string{"order", "outline_dirty"} {
		if !cols[want] {
			t.Fatalf("slides table missing column %q; got %v", want, cols)
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
