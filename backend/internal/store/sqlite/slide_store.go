package sqlite

import (
	"context"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReplaceSlides 用新大纲整体替换某 project 的 slides（幂等：先删后插，同一事务）。
// 大纲每次提交都是一份完整 slide 集合（DATA-MODEL：slides.idx 在 project 内唯一）。
func (s *Store) ReplaceSlides(ctx context.Context, projectID string, slides []model.Slide) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", projectID).Delete(&slidePO{}).Error; err != nil {
			return err
		}
		for _, sl := range slides {
			if err := tx.Create(slideToPO(sl)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListSlides 返回某 project 的全部 slides（按 idx 升序）。
func (s *Store) ListSlides(ctx context.Context, projectID string) ([]model.Slide, error) {
	var pos []slidePO
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("idx ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Slide, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}

// SetProjectStatus 更新 project 状态游标（draft/generating/ready）。
func (s *Store) SetProjectStatus(ctx context.Context, id, status string) error {
	return s.db.WithContext(ctx).Model(&projectPO{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": nowUnix()}).Error
}

// NextVersionNo 返回 (target_type,target_id) 维度下一个版本号（单调递增，不复用 DATA-VERSION-002）。
func (s *Store) NextVersionNo(ctx context.Context, targetType, targetID string) (int, error) {
	var maxNo *int
	if err := s.db.WithContext(ctx).Model(&versionPO{}).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Select("MAX(version_no)").Scan(&maxNo).Error; err != nil {
		return 0, err
	}
	if maxNo == nil {
		return 0, nil
	}
	return *maxNo + 1, nil
}

// CreateVersion 登记一条版本快照记录（DATA-VERSION-003）。
func (s *Store) CreateVersion(ctx context.Context, v model.Version) error {
	return s.db.WithContext(ctx).Create(versionToPO(v)).Error
}

// ListVersions 返回某目标的全部版本（按 version_no 升序）。
func (s *Store) ListVersions(ctx context.Context, targetType, targetID string) ([]model.Version, error) {
	var pos []versionPO
	if err := s.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("version_no ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Version, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}
