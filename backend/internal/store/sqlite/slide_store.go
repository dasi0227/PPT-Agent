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
			"deck_revision": commit.DeckRevision, "design_revision": commit.DesignRevision,
			"deck_path": "deck.json", "updated_at": nowUnix(),
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
					"project_id", "position", "layout", "title", "json_path", "html_path",
					"current_version", "blueprint_revision", "presentation_revision",
					"source_deck_revision", "source_blueprint_revision", "source_design_revision",
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

// ListSlides 返回某 project 的全部 slides（按 position 升序）。
func (s *Store) ListSlides(ctx context.Context, projectID string) ([]model.Slide, error) {
	var pos []slidePO
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("position ASC").Find(&pos).Error; err != nil {
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

// SetSlidesOrder uses a temporary negative position space so the unique
// (project_id, position) constraint is never violated mid-reorder.
func (s *Store) SetSlidesOrder(ctx context.Context, projectID string, orderByID map[string]int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		temporary := -1
		for id := range orderByID {
			if err := tx.Model(&slidePO{}).
				Where("id = ? AND project_id = ?", id, projectID).
				Update("position", temporary).Error; err != nil {
				return err
			}
			temporary--
		}
		for id, position := range orderByID {
			if err := tx.Model(&slidePO{}).
				Where("id = ? AND project_id = ?", id, projectID).
				Update("position", position).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SetProjectStatus 更新 project 状态游标（draft/generating/ready）。
func (s *Store) SetProjectStatus(ctx context.Context, id, status string) error {
	return s.db.WithContext(ctx).Model(&projectPO{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": nowUnix()}).Error
}

// UpdateSlideMeta 同步某页元数据 title/layout（手动 PATCH 改这两项时保持 DB 与 slide.json 一致）。
func (s *Store) UpdateSlideMeta(ctx context.Context, slideID, title, layout string) error {
	return s.db.WithContext(ctx).Model(&slidePO{}).
		Where("id = ?", slideID).
		Updates(map[string]any{"title": title, "layout": layout}).Error
}

func (s *Store) UpdateSlideRevisions(ctx context.Context, slideID string, blueprintRevision, presentationRevision, sourceDeckRevision, sourceBlueprintRevision, sourceDesignRevision int) error {
	return s.db.WithContext(ctx).Model(&slidePO{}).Where("id = ?", slideID).
		Updates(map[string]any{
			"blueprint_revision": blueprintRevision, "presentation_revision": presentationRevision,
			"source_deck_revision": sourceDeckRevision, "source_blueprint_revision": sourceBlueprintRevision,
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
