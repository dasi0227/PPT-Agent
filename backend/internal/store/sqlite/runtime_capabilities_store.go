package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
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
		index.ID = "ctxidx_" + workflowHash(index.RunID, index.PackHash, index.BuiltAt)
	}
	raw, err := json.Marshal(index)
	if err != nil {
		return "", err
	}
	err = s.db.WithContext(ctx).Create(&contextIndexSnapshotPO{
		ID: index.ID, RunID: index.RunID, PackHash: index.PackHash,
		IndexJSON: string(raw), CreatedAt: index.BuiltAt,
	}).Error
	return index.ID, err
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
	return s.db.WithContext(ctx).Create(&semanticReviewPO{
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
