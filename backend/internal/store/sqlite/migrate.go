package sqlite

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/migrations"
)

// Migrate 按序执行 migrations/ 中的 SQL（schema 权威源，对应 sqlite-schema.sql）。
// 语句均为幂等 CREATE ... IF NOT EXISTS，可重复执行。
func Migrate(db *gorm.DB, log *zap.Logger) error {
	files, err := migrations.Files()
	if err != nil {
		return err
	}
	for _, name := range files {
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		for _, stmt := range splitStatements(string(content)) {
			if err := db.Exec(stmt).Error; err != nil {
				// SQLite lacks ADD COLUMN IF NOT EXISTS. Re-running additive migrations
				// is safe when the column already exists.
				if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
					continue
				}
				return fmt.Errorf("apply %s: %w", name, err)
			}
		}
		log.Info("migration applied", zap.String("file", name))
	}
	return nil
}

// splitStatements 按 ; 切分脚本，跳过仅含空白/注释的片段。
// schema 内的语句体不含分号，故简单切分是安全的。
func splitStatements(script string) []string {
	var out []string
	for _, chunk := range strings.Split(script, ";") {
		if containsSQL(chunk) {
			out = append(out, strings.TrimSpace(chunk))
		}
	}
	return out
}

func containsSQL(chunk string) bool {
	for _, line := range strings.Split(chunk, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		return true
	}
	return false
}
