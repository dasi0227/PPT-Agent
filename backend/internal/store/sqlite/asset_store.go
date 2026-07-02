package sqlite

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// UpsertAsset 按 (name,kind) 幂等写入资产元数据（seeding 与 /repo 共用）。
// 已存在则更新可变字段；用于冷启动幂等（DS-SEED-005）。
func (s *Store) UpsertAsset(ctx context.Context, a model.Asset) error {
	po := assetToPO(a)
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}, {Name: "kind"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"version", "source", "description", "tags", "manifest_path", "dir", "updated_at",
		}),
	}).Create(&po).Error
}

// ListAssets 列出资产元数据；kind 非空则按 kind 过滤（AC-SEED-001 / search 基础）。
func (s *Store) ListAssets(ctx context.Context, kind string) ([]model.Asset, error) {
	var pos []assetPO
	q := s.db.WithContext(ctx).Order("kind ASC, name ASC")
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if err := q.Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Asset, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}

// CountAssets 返回资产总数（seeding 幂等判断辅助）。
func (s *Store) CountAssets(ctx context.Context) (int, error) {
	var n int64
	if err := s.db.WithContext(ctx).Model(&assetPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// GetAsset 按 id 读取资产元数据。
func (s *Store) GetAsset(ctx context.Context, id string) (model.Asset, error) {
	var po assetPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Asset{}, err
	}
	return po.toModel(), nil
}

// SetSlideVersion 更新某页当前版本号（单页生成/重生成落版本后同步 slides.current_version）。
func (s *Store) SetSlideVersion(ctx context.Context, projectID string, idx, versionNo int) error {
	return s.db.WithContext(ctx).Model(&slidePO{}).
		Where("project_id = ? AND idx = ?", projectID, idx).
		Update("current_version", versionNo).Error
}
