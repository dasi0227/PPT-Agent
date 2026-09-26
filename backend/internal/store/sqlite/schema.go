package sqlite

import (
	_ "embed"
	"fmt"
	"slices"

	"gorm.io/gorm"
)

//go:embed schema.sql
var schemaSQL string

const applicationID = 0x50505441
const schemaVersion = 3

func initializeSchema(db *gorm.DB) error {
	var appID, version int
	var tables []string
	if err := db.Raw("PRAGMA application_id").Scan(&appID).Error; err != nil {
		return err
	}
	if err := db.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		return err
	}
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&tables).Error; err != nil {
		return err
	}
	if len(tables) > 0 {
		if appID != applicationID || version != schemaVersion {
			return fmt.Errorf("incompatible development database; initialize a new work directory (expected format %d)", schemaVersion)
		}
		expected := []string{"command_executions", "file_settings", "idempotency_records", "projects", "resource_tags", "resources", "runs", "shortcut_settings", "slides", "steering_inbox", "tags", "thread_event_outbox", "threads"}
		if !slices.Equal(tables, expected) {
			return fmt.Errorf("database schema inventory is invalid; use a new work directory")
		}
		return nil
	}
	if appID != 0 || version != 0 {
		return fmt.Errorf("database format markers exist without schema; use a new work directory")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(schemaSQL).Error; err != nil {
			return err
		}
		if err := tx.Exec(fmt.Sprintf("PRAGMA application_id = %d", applicationID)).Error; err != nil {
			return err
		}
		return tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)).Error
	})
}
