package sqlite

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

func (s *Store) CommitWorkflow(ctx context.Context, commit model.ArtifactCommit) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&projectPO{}).Where("id = ?", commit.ProjectID).Updates(map[string]any{
			"outline_revision": commit.OutlineRevision, "design_revision": commit.DesignRevision,
			"layout_version": 2, "updated_at": nowUnix(),
		}).Error; err != nil {
			return err
		}
		if len(commit.DeletedSlideIDs) > 0 {
			if err := tx.Where("project_id = ? AND id IN ?", commit.ProjectID, commit.DeletedSlideIDs).
				Delete(&slidePO{}).Error; err != nil {
				return err
			}
		}
		for _, slide := range commit.Slides {
			po := slideToPO(slide)
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"project_id", "current_version", "spec_revision", "html_revision",
					"source_outline_revision", "source_spec_revision", "source_design_revision",
				}),
			}).Create(&po).Error; err != nil {
				return err
			}
		}
		for _, version := range commit.Versions {
			po := versionToPO(version)
			if err := tx.Create(&po).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListSlides 返回某 project 的全部 slides。DB 不再存顺序（position 已退出），此处按 id
// 稳定返回；页面顺序由读路径依据 outline.json 的 slide_order 投影。
func (s *Store) ListSlides(ctx context.Context, projectID string) ([]model.Slide, error) {
	var pos []slidePO
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Slide, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}

// GetSlide 按 slide id 读取单页元数据（rollback 用 id 反查 idx/project）。
func (s *Store) GetSlide(ctx context.Context, id string) (model.Slide, error) {
	var po slidePO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Slide{}, err
	}
	return po.toModel(), nil
}

// InsertSlide 单页插入（不清空其它页，结构操作加页用）。
func (s *Store) InsertSlide(ctx context.Context, sl model.Slide) error {
	po := slideToPO(sl)
	return s.db.WithContext(ctx).Create(&po).Error
}

// DeleteSlideByID 按 id 删除单页 DB 行（无回收站，结构操作删页用）。
func (s *Store) DeleteSlideByID(ctx context.Context, slideID string) error {
	return s.db.WithContext(ctx).Where("id = ?", slideID).Delete(&slidePO{}).Error
}

// SetSlidesOrder is a no-op cursor: slide order now lives solely in
// outline.json's slide_order. Callers persist order by rewriting the outline.
func (s *Store) SetSlidesOrder(ctx context.Context, projectID string, orderByID map[string]int) error {
	_ = ctx
	_ = projectID
	_ = orderByID
	return nil
}

// SetProjectStatus 更新 project 状态游标（draft/generating/ready）。
func (s *Store) SetProjectStatus(ctx context.Context, id, status string) error {
	return s.db.WithContext(ctx).Model(&projectPO{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": nowUnix()}).Error
}

// UpdateSlideMeta is a no-op cursor: title/layout now live solely in spec.json.
// Callers persist them by rewriting the slide spec.
func (s *Store) UpdateSlideMeta(ctx context.Context, slideID, title, layout string) error {
	_ = ctx
	_ = slideID
	_ = title
	_ = layout
	return nil
}

func (s *Store) UpdateSlideRevisions(ctx context.Context, slideID string, specRevision, htmlRevision, sourceOutlineRevision, sourceSpecRevision, sourceDesignRevision int) error {
	return s.db.WithContext(ctx).Model(&slidePO{}).Where("id = ?", slideID).
		Updates(map[string]any{
			"spec_revision": specRevision, "html_revision": htmlRevision,
			"source_outline_revision": sourceOutlineRevision, "source_spec_revision": sourceSpecRevision,
			"source_design_revision": sourceDesignRevision,
		}).Error
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
	po := versionToPO(v)
	return s.db.WithContext(ctx).Create(&po).Error
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

// DeleteVersion 删除一条版本登记，用于复合写失败补偿。
func (s *Store) DeleteVersion(ctx context.Context, targetType, targetID string, versionNo int) error {
	return s.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ? AND version_no = ?", targetType, targetID, versionNo).
		Delete(&versionPO{}).Error
}
