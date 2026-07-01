package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 迁移文件 0001_init.sql MUST 与 docs 权威 schema 的 DDL 保持一致（DEV-RULES R1）。
// 比较时忽略注释与空白，只对齐可执行 SQL 语句集合。
func TestMigrationMatchesDocsSchema(t *testing.T) {
	migration, err := FS.ReadFile("0001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	docsPath := filepath.Join("..", "..", "docs", "30-data-model", "sqlite-schema.sql")
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
