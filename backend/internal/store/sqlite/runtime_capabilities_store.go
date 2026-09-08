package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) SaveCheckpoint(ctx context.Context, checkpoint workflow.RuntimeCheckpoint) error {
	if checkpoint.RunID == "" || checkpoint.LoopID == "" {
		return errors.New("checkpoint requires run_id and loop_id")
	}
	if checkpoint.CreatedAt == 0 {
		checkpoint.CreatedAt = time.Now().UnixNano()
	}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var seq int64
		if err := tx.Raw("SELECT COALESCE(MAX(seq), 0) + 1 FROM run_checkpoints WHERE run_id = ?", checkpoint.RunID).Scan(&seq).Error; err != nil {
			return err
		}
		id := "ckpt_" + workflowHash(checkpoint.RunID, checkpoint.LoopID, seq, checkpoint.CreatedAt)
		return tx.Create(&runCheckpointPO{
			ID: id, RunID: checkpoint.RunID, LoopID: checkpoint.LoopID, Seq: seq,
			Phase: string(checkpoint.Phase), CheckpointJSON: string(raw), CreatedAt: checkpoint.CreatedAt,
		}).Error
	})
}

// CommitPlanApproval is the durable commit point for the in-place plan -> execute
// transition. The Run command, refreshed ContextManifest, and complete checkpoint
// become visible together or not at all.
func (s *Store) CommitPlanApproval(
	ctx context.Context,
	runID string,
	mode model.RunMode,
	runContext model.RunContext,
	checkpoint workflow.RuntimeCheckpoint,
) error {
	if runID == "" || mode != model.ModeExecute || checkpoint.RunID != runID || checkpoint.Mode != mode {
		return errors.New("invalid plan approval transition")
	}
	if checkpoint.CreatedAt == 0 {
		checkpoint.CreatedAt = time.Now().UnixNano()
	}
	rawCheckpoint, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run runPO
		if err := tx.First(&run, "id = ?", runID).Error; err != nil {
			return mapErr(err)
		}
		if model.RunMode(run.Mode) != model.ModePlan {
			return errors.New("run is not in plan mode")
		}
		var command model.RunCommand
		if err := json.Unmarshal([]byte(run.RunCommandJSON), &command); err != nil {
			return err
		}
		command.Mode = mode
		rawCommand, err := json.Marshal(command)
		if err != nil {
			return err
		}
		result := tx.Model(&runPO{}).Where("id = ? AND mode = ?", runID, string(model.ModePlan)).Updates(map[string]any{
			"mode": string(mode), "run_command_json": string(rawCommand), "status": string(model.RunRunning), "updated_at": nowUnix(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("run mode changed during plan approval")
		}
		runContext.RunID = runID
		if runContext.CreatedAt == 0 {
			runContext.CreatedAt = time.Now().Unix()
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "run_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"context_id", "profile", "pack_hash", "estimated_tokens", "budget_tokens", "manifest_json", "created_at",
			}),
		}).Create(contextToPO(runContext)).Error; err != nil {
			return err
		}
		var seq int64
		if err := tx.Raw("SELECT COALESCE(MAX(seq), 0) + 1 FROM run_checkpoints WHERE run_id = ?", runID).Scan(&seq).Error; err != nil {
			return err
		}
		id := "ckpt_" + workflowHash(runID, checkpoint.LoopID, seq, checkpoint.CreatedAt)
		return tx.Create(&runCheckpointPO{
			ID: id, RunID: runID, LoopID: checkpoint.LoopID, Seq: seq,
			Phase: string(checkpoint.Phase), CheckpointJSON: string(rawCheckpoint), CreatedAt: checkpoint.CreatedAt,
		}).Error
	})
}

// CommitScopeExpansion atomically advances the canonical Run scope and the
// checkpoint that resumes execution under that exact revision.
func (s *Store) CommitScopeExpansion(ctx context.Context, runID string, scope model.RunScope, checkpoint workflow.RuntimeCheckpoint) error {
	if runID == "" || checkpoint.RunID != runID || checkpoint.Scope.Revision != scope.Revision {
		return errors.New("invalid scope expansion transition")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if checkpoint.CreatedAt == 0 {
		checkpoint.CreatedAt = time.Now().UnixNano()
	}
	rawCheckpoint, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	slideIDs, _ := json.Marshal(scope.SlideIDs)
	source, _ := json.Marshal(scope.Source)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runPO
		if err := tx.First(&row, "id = ?", runID).Error; err != nil {
			return mapErr(err)
		}
		var command model.RunCommand
		if err := json.Unmarshal([]byte(row.RunCommandJSON), &command); err != nil {
			return err
		}
		if command.Scope.Revision+1 != scope.Revision {
			return errors.New("scope revision changed during approval")
		}
		command.Scope = scope
		rawCommand, err := json.Marshal(command)
		if err != nil {
			return err
		}
		result := tx.Model(&runPO{}).Where("id = ? AND scope_revision = ?", runID, command.Scope.Revision-1).Updates(map[string]any{
			"scope_object": string(scope.Object), "scope_slide_ids_json": string(slideIDs), "scope_source_json": string(source),
			"scope_include_run_created_slides": boolInt(scope.IncludeRunCreatedSlides), "scope_revision": scope.Revision,
			"run_command_json": string(rawCommand), "status": string(model.RunRunning), "updated_at": nowUnix(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("scope revision changed during approval")
		}
		var seq int64
		if err := tx.Raw("SELECT COALESCE(MAX(seq), 0) + 1 FROM run_checkpoints WHERE run_id = ?", runID).Scan(&seq).Error; err != nil {
			return err
		}
		id := "ckpt_" + workflowHash(runID, checkpoint.LoopID, seq, checkpoint.CreatedAt)
		return tx.Create(&runCheckpointPO{ID: id, RunID: runID, LoopID: checkpoint.LoopID, Seq: seq, Phase: string(checkpoint.Phase), CheckpointJSON: string(rawCheckpoint), CreatedAt: checkpoint.CreatedAt}).Error
	})
}

func (s *Store) LatestCheckpoint(ctx context.Context, runID string) (workflow.RuntimeCheckpoint, error) {
	var po runCheckpointPO
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).
		Order("seq DESC").First(&po).Error; err != nil {
		return workflow.RuntimeCheckpoint{}, mapErr(err)
	}
	var checkpoint workflow.RuntimeCheckpoint
	if err := json.Unmarshal([]byte(po.CheckpointJSON), &checkpoint); err != nil {
		return workflow.RuntimeCheckpoint{}, err
	}
	return checkpoint, nil
}

func (s *Store) ListCheckpoints(ctx context.Context, runID string, limit int) ([]workflow.RuntimeCheckpoint, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []runCheckpointPO
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).
		Order("seq DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]workflow.RuntimeCheckpoint, 0, len(rows))
	for _, row := range rows {
		var checkpoint workflow.RuntimeCheckpoint
		if err := json.Unmarshal([]byte(row.CheckpointJSON), &checkpoint); err != nil {
			return nil, err
		}
		out = append(out, checkpoint)
	}
	return out, nil
}

func (s *Store) SaveContextIndex(ctx context.Context, index workflow.ContextIndex) (string, error) {
	if index.RunID == "" {
		return "", errors.New("context index requires run_id")
	}
	if index.BuiltAt == 0 {
		index.BuiltAt = time.Now().UnixNano()
	}
	if index.ID == "" {
		index.ID = workflow.ContextIndexSnapshotID(index)
	}
	contentHash := workflow.ContextIndexContentHash(index)
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := json.Marshal(index)
		if err != nil {
			return "", err
		}
		result := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).Create(&contextIndexSnapshotPO{
			ID: index.ID, RunID: index.RunID, PackHash: index.PackHash,
			IndexJSON: string(raw), CreatedAt: index.BuiltAt,
		})
		if result.Error != nil {
			return "", result.Error
		}
		if result.RowsAffected == 1 {
			return index.ID, nil
		}

		existing, err := s.GetContextIndex(ctx, index.ID)
		if err != nil {
			return "", err
		}
		if workflow.ContextIndexContentHash(existing) == contentHash {
			return existing.ID, nil
		}
		versionID := workflow.ContextIndexSnapshotID(index)
		if versionID == index.ID {
			return "", errors.New("context index id collision")
		}
		index.ID = versionID
	}
	return "", errors.New("context index version collision")
}

func (s *Store) GetContextIndex(ctx context.Context, id string) (workflow.ContextIndex, error) {
	var po contextIndexSnapshotPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return workflow.ContextIndex{}, mapErr(err)
	}
	var index workflow.ContextIndex
	if err := json.Unmarshal([]byte(po.IndexJSON), &index); err != nil {
		return workflow.ContextIndex{}, err
	}
	return index, nil
}

func (s *Store) LatestContextIndex(ctx context.Context, runID string) (workflow.ContextIndex, error) {
	var po contextIndexSnapshotPO
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).
		Order("created_at DESC").First(&po).Error; err != nil {
		return workflow.ContextIndex{}, mapErr(err)
	}
	var index workflow.ContextIndex
	if err := json.Unmarshal([]byte(po.IndexJSON), &index); err != nil {
		return workflow.ContextIndex{}, err
	}
	return index, nil
}

func (s *Store) SaveSemanticReview(ctx context.Context, review workflow.StoredSemanticReview) error {
	if review.CreatedAt == 0 {
		review.CreatedAt = time.Now().UnixNano()
	}
	accepted := 0
	if review.Accepted {
		accepted = 1
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&semanticReviewPO{
		ID: review.ID, RunID: review.RunID, FinishCallID: review.FinishCallID,
		Accepted: accepted, Confidence: review.Confidence, InputHash: review.InputHash,
		OutputJSON: review.OutputJSON, PromptManifestJSON: review.PromptManifestJSON,
		CreatedAt: review.CreatedAt,
	}).Error
}

func workflowHash(parts ...any) string {
	raw, _ := json.Marshal(parts)
	hash := workflowHashBytes(raw)
	if len(hash) > 24 {
		return hash[:24]
	}
	return hash
}

func workflowHashBytes(raw []byte) string {
	const table = "0123456789abcdef"
	var h uint64 = 1469598103934665603
	for _, b := range raw {
		h ^= uint64(b)
		h *= 1099511628211
	}
	out := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		out[i] = table[h&0xf]
		h >>= 4
	}
	return string(out)
}

var _ run.Store = (*Store)(nil)
