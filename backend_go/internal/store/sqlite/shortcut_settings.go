package sqlite

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/shortcuts"
)

func (s *Store) ReadShortcutSettings(ctx context.Context) (int64, map[string]shortcuts.Binding, error) {
	var row struct {
		Revision      int64
		OverridesJSON string `gorm:"column:overrides_json"`
	}
	if err := s.db.WithContext(ctx).Table("shortcut_settings").Where("id = 1").Take(&row).Error; err != nil {
		return 0, nil, err
	}
	var overrides map[string]shortcuts.Binding
	err := json.Unmarshal([]byte(row.OverridesJSON), &overrides)
	return row.Revision, overrides, err
}
func (s *Store) WriteShortcutSettings(ctx context.Context, revision int64, overrides map[string]shortcuts.Binding) error {
	raw, err := json.Marshal(overrides)
	if err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Exec("UPDATE shortcut_settings SET overrides_json = ?, revision = revision + 1 WHERE id = 1 AND revision = ?", string(raw), revision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return shortcuts.ErrConflict
	}
	return nil
}
