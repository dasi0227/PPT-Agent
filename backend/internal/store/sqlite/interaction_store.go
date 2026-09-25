package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

func (s *Store) AcceptInteractionAnswer(ctx context.Context, runID, kind, id string, answer json.RawMessage) error {
	hash := sha256.Sum256(answer)
	digest := hex.EncodeToString(hash[:])
	threadID := ""
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			ThreadID string
			Status   string
		}
		if err := tx.Table("runs").Select("thread_id,status").Where("id = ?", runID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		threadID = row.ThreadID
		var receipt idempotencyPO
		err := tx.First(&receipt, "scope = ? AND owner_id = ? AND key = ?", "interaction_answer", runID, kind+":"+id).Error
		if err == nil {
			if receipt.RequestHash != digest {
				return run.ErrReplyMismatch
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if model.RunStatus(row.Status).Terminal() {
			return run.ErrRunNotRunning
		}
		raw, _ := json.Marshal(map[string]any{"kind": kind, "interaction_id": id, "answer": answer})
		event, err := s.enqueueEvent(tx, threadID, threadjournal.Event{Type: "interaction.answer_accepted", RunID: runID, Payload: raw})
		if err != nil {
			return err
		}
		reference, _ := json.Marshal(map[string]int64{"event_seq": event.Seq})
		return tx.Create(&idempotencyPO{Scope: "interaction_answer", OwnerID: runID, Key: kind + ":" + id, RequestHash: digest, Status: "completed", ResultJSON: string(reference)}).Error
	})
	if err == nil {
		err = s.FlushThreadEvents(ctx, threadID)
	}
	return err
}
func (s *Store) InteractionAnswer(ctx context.Context, runID, kind, id string) (json.RawMessage, error) {
	receipt, err := s.GetIdempotency(ctx, "interaction_answer", runID, kind+":"+id)
	if err != nil {
		return nil, err
	}
	var ref struct {
		Seq int64 `json:"event_seq"`
	}
	if err := json.Unmarshal([]byte(receipt.ResultJSON), &ref); err != nil {
		return nil, err
	}
	var row struct{ ThreadID string }
	if err := s.db.WithContext(ctx).Table("runs").Select("thread_id").Where("id = ?", runID).Take(&row).Error; err != nil {
		return nil, mapErr(err)
	}
	event, err := eventInTransaction(s.db.WithContext(ctx), row.ThreadID, ref.Seq)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Answer json.RawMessage `json:"answer"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.Answer, nil
}
