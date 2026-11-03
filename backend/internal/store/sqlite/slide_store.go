package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"

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
		// Artifact receipts cannot authorize a stale worker after a scene switch.
		// This check replaces the fence formerly hidden inside journal enqueue.
		var worker runPO
		if err := tx.Where("id = ? AND project_id = ?", commit.RunID, commit.ProjectID).Take(&worker).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, worker.ThreadID)
		if err != nil {
			return err
		}
		if head.SceneRevision != worker.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		if scene, ok := sessionstate.RequestScene(ctx); !ok || scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		owner, execution, ok := workflow.CheckpointOwnership(ctx)
		if !ok || owner != worker.OwnerInstanceID || execution != worker.ExecutionRevision {
			return sessionstate.ErrVersionConflict
		}
		switch model.RunStatus(worker.Status) {
		case model.RunRunning, model.RunWaiting, model.RunRecovering:
		default:
			return sessionstate.ErrVersionConflict
		}
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
			if commit.ToolResultJSON != "" {
				var execution toolExecutionPO
				if err := tx.Where("run_id = ? AND operation_id = ?", commit.RunID, commit.OperationID).Order("attempt_no DESC").Take(&execution).Error; err != nil {
					return err
				}
				if execution.Status != "executing" || execution.SceneRevision != head.SceneRevision || execution.ExecutionRevision != worker.ExecutionRevision {
					return sessionstate.ErrVersionConflict
				}
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
					"project_id",
				}),
			}).Create(&po).Error; err != nil {
				return err
			}
		}
		if commit.RunID != "" && commit.OperationID != "" {
			resultJSON := commit.ToolResultJSON
			if resultJSON == "" {
				resultJSON = "{}"
			}
			receipt := idempotencyPO{
				Scope: "artifact_commit", OwnerID: commit.RunID, Key: commit.OperationID,
				RequestHash: commit.RequestHash, Status: "completed", ResultJSON: resultJSON,
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
