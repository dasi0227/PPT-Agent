package sqlite

import (
	"context"
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/fileopen"
)

func (s *Store) ReadFileSettings(ctx context.Context) (fileopen.Settings, error) {
	var row struct {
		DefaultOpen string
		JSONOpen    string `gorm:"column:json_open"`
		HTMLOpen    string `gorm:"column:html_open"`
		CustomApps  string
		Revision    int64
	}
	if err := s.db.WithContext(ctx).Table("file_settings").Where("id = 1").Take(&row).Error; err != nil {
		return fileopen.Settings{}, err
	}
	value := fileopen.Settings{Revision: row.Revision}
	for _, entry := range []struct {
		raw    string
		target any
	}{{row.DefaultOpen, &value.Default}, {row.JSONOpen, &value.JSON}, {row.HTMLOpen, &value.HTML}, {row.CustomApps, &value.CustomApps}} {
		if err := json.Unmarshal([]byte(entry.raw), entry.target); err != nil {
			return fileopen.Settings{}, err
		}
	}
	return value, nil
}
func (s *Store) WriteFileSettings(ctx context.Context, value fileopen.Settings) error {
	if value.CustomApps == nil {
		value.CustomApps = []fileopen.Application{}
	}
	args := make([]any, 0, 5)
	for _, entry := range []any{value.Default, value.JSON, value.HTML, value.CustomApps} {
		raw, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		args = append(args, string(raw))
	}
	args = append(args, value.Revision)
	result := s.db.WithContext(ctx).Exec("UPDATE file_settings SET default_open = ?, json_open = ?, html_open = ?, custom_apps = ?, revision = revision + 1 WHERE id = 1 AND revision = ?", args...)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fileopen.ErrConflict
	}
	return nil
}
