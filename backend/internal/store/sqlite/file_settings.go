package sqlite

import (
	"context"
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/fileopen"
)

func (s *Store) ReadFileSettings(ctx context.Context) (fileopen.Settings, error) {
	var row struct {
		OpenWith      string
		CustomAppPath string
		CustomApps    string
		Revision      int64
	}
	if err := s.db.WithContext(ctx).Table("file_settings").Where("id = 1").Take(&row).Error; err != nil {
		return fileopen.Settings{}, err
	}
	value := fileopen.Settings{OpenWith: row.OpenWith, CustomAppPath: row.CustomAppPath, Revision: row.Revision}
	if err := json.Unmarshal([]byte(row.CustomApps), &value.CustomApps); err != nil {
		return fileopen.Settings{}, err
	}
	return value, nil
}
func (s *Store) WriteFileSettings(ctx context.Context, value fileopen.Settings) error {
	if value.CustomApps == nil {
		value.CustomApps = []fileopen.Application{}
	}
	apps, err := json.Marshal(value.CustomApps)
	if err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Exec("UPDATE file_settings SET open_with = ?, custom_app_path = ?, custom_apps = ?, revision = revision + 1 WHERE id = 1 AND revision = ?", value.OpenWith, value.CustomAppPath, string(apps), value.Revision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fileopen.ErrConflict
	}
	return nil
}
