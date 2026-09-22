package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReplaceSlides 用新大纲整体替换某 project 的 slides（幂等：先删后插，同一事务）。
// 大纲每次提交都是一份完整 slide 集合（DATA-MODEL：slides.idx 在 project 内唯一）。
func (s *Store) ReplaceSlides(ctx context.Context, projectID string, slides []model.Slide) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous []slidePO
		if err := tx.Where("project_id = ?", projectID).Find(&previous).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(previous))
		for _, slide := range previous {
			ids = append(ids, slide.ID)
		}
		if err := rememberDeletedSlides(tx, projectID, ids); err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&slidePO{}).Error; err != nil {
			return err
		}
		for _, sl := range slides {
			if err := forgetDeletedSlide(tx, projectID, sl.ID); err != nil {
				return err
			}
			if err := tx.Create(slideToPO(sl)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) CommitWorkflow(ctx context.Context, commit model.ArtifactCommit) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if commit.RunID != "" && commit.OperationID != "" {
			var receipt idempotencyPO
			err := tx.First(&receipt, "scope = ? AND owner_id = ? AND key = ?", "artifact_commit", commit.RunID, commit.OperationID).Error
			if err == nil {
				if receipt.RequestHash != commit.RequestHash || receipt.Status != "completed" {
					return fmt.Errorf("artifact commit receipt conflicts with operation %s", commit.OperationID)
				}
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := tx.Model(&projectPO{}).Where("id = ?", commit.ProjectID).Updates(map[string]any{
			"layout_version": currentProjectLayoutVersion, "updated_at": nowUnix(),
		}).Error; err != nil {
			return err
		}
		if len(commit.DeletedSlideIDs) > 0 {
			if err := rememberDeletedSlides(tx, commit.ProjectID, commit.DeletedSlideIDs); err != nil {
				return err
			}
			if err := tx.Where("project_id = ? AND id IN ?", commit.ProjectID, commit.DeletedSlideIDs).
				Delete(&slidePO{}).Error; err != nil {
				return err
			}
		}
		for _, slide := range commit.Slides {
			if err := forgetDeletedSlide(tx, slide.ProjectID, slide.ID); err != nil {
				return err
			}
			po := slideToPO(slide)
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"project_id", "current_version", "last_export_at",
				}),
			}).Create(&po).Error; err != nil {
				return err
			}
		}
		if commit.RunID != "" && commit.OperationID != "" {
			now := time.Now().UnixNano()
			receipt := idempotencyPO{
				Scope: "artifact_commit", OwnerID: commit.RunID, Key: commit.OperationID,
				RequestHash: commit.RequestHash, Status: "completed", ResultJSON: commit.ToolResultJSON,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "scope"}, {Name: "owner_id"}, {Name: "key"}},
				DoNothing: true,
			}).Create(&receipt).Error; err != nil {
				return err
			}
			var stored idempotencyPO
			if err := tx.First(&stored, "scope = ? AND owner_id = ? AND key = ?", "artifact_commit", commit.RunID, commit.OperationID).Error; err != nil {
				return err
			}
			if stored.RequestHash != commit.RequestHash || stored.Status != "completed" {
				return fmt.Errorf("artifact commit receipt conflicts with operation %s", commit.OperationID)
			}
			if commit.ToolResultJSON != "" {
				if err := tx.Model(&idempotencyPO{}).
					Where("scope = ? AND owner_id = ? AND key = ?", "tool_call", commit.RunID, commit.OperationID).
					Updates(map[string]any{"status": "completed", "result_json": commit.ToolResultJSON, "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// ListSlides 返回某 project 的全部 slides。DB 不再存顺序（position 已退出），此处按 id
// 稳定返回；页面顺序由读路径依据 outline.json 的树形节点投影。
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

// GetSlide reads the current slide identity.
func (s *Store) GetSlide(ctx context.Context, id string) (model.Slide, error) {
	var po slidePO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Slide{}, err
	}
	return po.toModel(), nil
}

// InsertSlide 单页插入（不清空其它页，结构操作加页用）。
func (s *Store) InsertSlide(ctx context.Context, sl model.Slide) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := forgetDeletedSlide(tx, sl.ProjectID, sl.ID); err != nil {
			return err
		}
		return tx.Create(slideToPO(sl)).Error
	})
}

// DeleteSlideByID 按 id 删除单页 DB 行（无回收站，结构操作删页用）。
func (s *Store) DeleteSlideByID(ctx context.Context, slideID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var slide slidePO
		if err := tx.First(&slide, "id = ?", slideID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		if err := rememberDeletedSlides(tx, slide.ProjectID, []string{slideID}); err != nil {
			return err
		}
		return tx.Where("id = ?", slideID).Delete(&slidePO{}).Error
	})
}

// SetProjectStatus 更新 project 状态游标（draft/generating/ready）。
func (s *Store) SetProjectStatus(ctx context.Context, id, status string) error {
	return s.db.WithContext(ctx).Model(&projectPO{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": nowUnix()}).Error
}

// Tombstones contain identity only; checkpoints restore them with slide membership.
func rememberDeletedSlides(tx *gorm.DB, projectID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Exec(`INSERT OR IGNORE INTO deleted_slides(project_id, slide_id)
 SELECT project_id, id FROM slides WHERE project_id = ? AND id IN ?`, projectID, ids).Error
}

func forgetDeletedSlide(tx *gorm.DB, projectID, slideID string) error {
	return tx.Exec("DELETE FROM deleted_slides WHERE project_id = ? AND slide_id = ?", projectID, slideID).Error
}

func (s *Store) IsSlideDeleted(ctx context.Context, projectID, slideID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Table("deleted_slides").Where("project_id = ? AND slide_id = ?", projectID, slideID).Count(&count).Error
	return count > 0, err
}
