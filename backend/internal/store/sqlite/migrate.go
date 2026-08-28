package sqlite

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/migrations"
)

// Migrate 按序执行 migrations/ 中尚未记录的 SQL（schema 权威源）。
func Migrate(db *gorm.DB, log *zap.Logger) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	files, err := migrations.Files()
	if err != nil {
		return err
	}
	for _, name := range files {
		var applied int64
		if err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE name = ?", name).Scan(&applied).Error; err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		foreignKeysDisabled := strings.Contains(string(content), "PRAGMA foreign_keys = OFF")
		if foreignKeysDisabled {
			if err := db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				return err
			}
		}
		applyErr := db.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range splitStatements(string(content)) {
				if strings.HasPrefix(strings.ToUpper(stmt), "PRAGMA FOREIGN_KEYS") {
					continue
				}
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("apply %s: %w", name, err)
				}
			}
			return tx.Exec("INSERT INTO schema_migrations(name, applied_at) VALUES (?, ?)", name, nowUnix()).Error
		})
		if foreignKeysDisabled {
			if err := db.Exec("PRAGMA foreign_keys = ON").Error; applyErr == nil && err != nil {
				applyErr = err
			}
		}
		if applyErr != nil {
			return applyErr
		}
		log.Info("migration applied", zap.String("file", name))
	}
	var incompatible int64
	if err := db.Raw("SELECT COUNT(*) FROM projects WHERE layout_version <> ?", currentProjectLayoutVersion).Scan(&incompatible).Error; err != nil {
		return fmt.Errorf("existing development database is not canonical v%d; remove it instead of migrating it: %w", currentProjectLayoutVersion, err)
	}
	if incompatible > 0 {
		return fmt.Errorf("existing development database is not canonical v%d; remove it instead of migrating it", currentProjectLayoutVersion)
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
