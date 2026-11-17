package sqlite

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

type modelRequestPO struct {
	ID              string `gorm:"primaryKey"`
	ThreadID        string
	RunID           string
	SceneRevision   int64
	ContextRevision int64
	Provider        string
	Model           string
	RequestJSON     string
	ResponseJSON    *string
	CreatedSeq      int64
	CreatedAt       int64
}

func (modelRequestPO) TableName() string { return "model_request_snapshots" }

func (w *sessionWrite) checkpointModelCall(cp workflow.RuntimeCheckpoint) error {
	pending := cp.PendingModelCall
	if pending == nil {
		if cp.NewModelRequest != nil || len(cp.ModelResponseJSON) != 0 {
			return sessionstate.ErrInvalidState
		}
		return nil
	}
	if pending.ID == "" || pending.Provider == "" || pending.Model == "" || pending.ContextRevision != w.head.ContextRevision {
		return sessionstate.ErrInvalidState
	}
	if candidate := cp.NewModelRequest; candidate != nil {
		if candidate.ID != pending.ID || pending.Stage != "prepared" || candidate.SceneRevision != w.head.SceneRevision || candidate.Provider != pending.Provider || candidate.Model != pending.Model || candidate.ContextRevision != pending.ContextRevision || len(candidate.ResponseJSON) != 0 {
			return sessionstate.ErrInvalidState
		}
		if _, err := sessionstate.DecodeModelRequest(candidate.RequestJSON, llm.GenerateRequest{}); err != nil {
			return err
		}
		row := modelRequestPO{ID: pending.ID, ThreadID: w.head.ID, RunID: cp.RunID, SceneRevision: w.head.SceneRevision, ContextRevision: w.head.ContextRevision, Provider: pending.Provider, Model: pending.Model, RequestJSON: string(candidate.RequestJSON), CreatedSeq: w.seq, CreatedAt: w.now}
		if err := w.tx.Create(&row).Error; err != nil {
			return err
		}
	}
	var row modelRequestPO
	if err := w.tx.Where("id = ? AND run_id = ? AND thread_id = ?", pending.ID, cp.RunID, w.head.ID).Take(&row).Error; err != nil {
		return err
	}
	if row.Provider != pending.Provider || row.Model != pending.Model || row.ContextRevision != pending.ContextRevision {
		return sessionstate.ErrInvalidState
	}
	var prior struct {
		ID                *string
		Stage             *string
		CancelRequestedAt *int64
	}
	if err := w.tx.Table("runs").Select("json_extract(checkpoint_json, '$.pending_model_call.id') AS id,json_extract(checkpoint_json, '$.pending_model_call.stage') AS stage,cancel_requested_at").Where("id = ?", cp.RunID).Take(&prior).Error; err != nil {
		return err
	}
	previous := textOrEmpty(prior.Stage)
	if textOrEmpty(prior.ID) != pending.ID {
		if cp.NewModelRequest == nil || pending.Stage != "prepared" {
			return sessionstate.ErrVersionConflict
		}
		previous = ""
	}
	switch pending.Stage {
	case "prepared":
		if previous != "" && previous != "prepared" {
			return sessionstate.ErrVersionConflict
		}
	case "dispatched":
		if previous != "prepared" && previous != "dispatched" && previous != "outcome_unknown" {
			return sessionstate.ErrVersionConflict
		}
		if prior.CancelRequestedAt != nil {
			return context.Canceled
		}
	case "outcome_unknown":
		if previous != "dispatched" && previous != "outcome_unknown" {
			return sessionstate.ErrVersionConflict
		}
	case "responded":
		if previous != "dispatched" && previous != "responded" {
			return sessionstate.ErrVersionConflict
		}
	default:
		return sessionstate.ErrInvalidState
	}
	if len(cp.ModelResponseJSON) > 0 {
		if pending.Stage != "responded" {
			return sessionstate.ErrInvalidState
		}
		if _, err := sessionstate.DecodeModelResponse(cp.ModelResponseJSON); err != nil {
			return err
		}
		if row.ResponseJSON != nil {
			if *row.ResponseJSON != string(cp.ModelResponseJSON) {
				return sessionstate.ErrIdempotencyConflict
			}
		} else if err := w.tx.Model(&row).Update("response_json", string(cp.ModelResponseJSON)).Error; err != nil {
			return err
		}
	} else if pending.Stage == "responded" && row.ResponseJSON == nil {
		return sessionstate.ErrInvalidState
	}
	return nil
}

func hydrateCheckpointModelCall(tx *gorm.DB, cp *workflow.RuntimeCheckpoint) error {
	if cp.PendingModelCall == nil {
		return nil
	}
	pending := cp.PendingModelCall
	switch pending.Stage {
	case "prepared", "dispatched", "outcome_unknown", "responded":
	default:
		return sessionstate.ErrInvalidState
	}
	var row modelRequestPO
	if err := tx.Where("id = ? AND run_id = ?", pending.ID, cp.RunID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sessionstate.ErrInvalidState
		}
		return err
	}
	if row.ContextRevision != pending.ContextRevision || row.ContextRevision != cp.ContextRevision || row.Provider != pending.Provider || row.Model != pending.Model {
		return sessionstate.ErrInvalidState
	}
	if _, err := sessionstate.DecodeModelRequest(json.RawMessage(row.RequestJSON), llm.GenerateRequest{}); err != nil {
		return err
	}
	if (pending.Stage == "responded") != (row.ResponseJSON != nil) {
		return sessionstate.ErrInvalidState
	}
	if row.ResponseJSON != nil {
		if _, err := sessionstate.DecodeModelResponse(json.RawMessage(*row.ResponseJSON)); err != nil {
			return err
		}
	}
	cp.ModelRequestSnapshot = &sessionstate.ModelRequestSnapshot{ID: row.ID, SceneRevision: row.SceneRevision, ContextRevision: row.ContextRevision, Provider: row.Provider, Model: row.Model, RequestJSON: json.RawMessage(row.RequestJSON), ResponseJSON: rawOrNull(row.ResponseJSON)}
	return nil
}
