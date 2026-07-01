package sqlite

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// Open 用 GORM + modernc.org/sqlite（纯 Go，无 cgo，经 glebarez 适配）打开数据库。
// DSN 开启 WAL、busy_timeout 与外键（ADR-0012 / ARCH-BACKEND-004）。
func Open(cfg *config.Config, log *zap.Logger) (*gorm.DB, func(), error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create db dir: %w", err)
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
		cfg.DBPath,
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve sql.DB: %w", err)
	}
	// 单连接即单一串行写通道，天然规避 SQLite 写竞态（ARCH-BACKEND-004）。
	sqlDB.SetMaxOpenConns(1)

	cleanup := func() { _ = sqlDB.Close() }
	return db, cleanup, nil
}
