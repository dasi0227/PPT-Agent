package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"

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
			"updated_at": nowUnix(),
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
					"project_id", "generation_inputs_json",
				}),
			}).Create(&po).Error; err != nil {
				return err
			}
		}
		if commit.RunID != "" && commit.OperationID != "" {
			receipt := idempotencyPO{
				Scope: "artifact_commit", OwnerID: commit.RunID, Key: commit.OperationID,
				RequestHash: commit.RequestHash, Status: "completed", ResultJSON: "{}",
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
				result := tx.Model(&idempotencyPO{}).
					Where("scope = ? AND owner_id = ? AND key = ?", "tool_call", commit.RunID, commit.OperationID).
					Updates(map[string]any{"status": "completed", "result_json": commit.ToolResultJSON})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return errors.New("tool replay receipt missing before artifact commit")
				}
			}
			var owner struct{ ThreadID string }
			if err := tx.Table("runs").Select("thread_id").Where("id = ?", commit.RunID).Take(&owner).Error; err != nil {
				return mapErr(err)
			}
			raw, _ := json.Marshal(map[string]string{"operation_id": commit.OperationID, "request_hash": commit.RequestHash, "result_scope": "tool_call"})
			if _, err := s.enqueueEvent(tx, owner.ThreadID, threadjournal.Event{Type: "artifact.committed", RunID: commit.RunID, Payload: raw}); err != nil {
				return err
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
		return tx.Where("id = ?", slideID).Delete(&slidePO{}).Error
	})
}

// IsSlideDeleted rejects every reference outside the current project membership.
func (s *Store) IsSlideDeleted(ctx context.Context, projectID, slideID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Table("slides").Where("project_id = ? AND id = ?", projectID, slideID).Count(&count).Error
	return count == 0, err
}
