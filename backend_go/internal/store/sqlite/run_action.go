package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

func resumeIdentity(id string, action sessionstate.RunAction) (sessionstate.CommitIdentity, error) {
	if !action.Valid() {
		return sessionstate.CommitIdentity{}, sessionstate.ErrInvalidState
	}
	hash, err := commandHash(struct {
		RunID  string
		Action sessionstate.RunAction
	}{id, action})
	return sessionstate.CommitIdentity{Key: "resume-run:" + action.RequestKey, RequestHash: hash}, err
}

// RunResumeReceipt acknowledges an already accepted action before expensive
// recovery preparation. It never grants execution ownership a second time.
func (s *Store) RunResumeReceipt(ctx context.Context, id string, action sessionstate.RunAction) (sessionstate.RunMutation, bool, error) {
	identity, err := resumeIdentity(id, action)
	if err != nil {
		return sessionstate.RunMutation{}, false, err
	}
	var receipt sessionstate.Receipt
	found := false
	err = s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ ThreadID string }
		if err := tx.Table("runs").Select("thread_id").Where("id = ?", id).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, row.ThreadID)
		if err != nil {
			return err
		}
		if head.SceneRevision != action.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		var stored idempotencyPO
		err = tx.Where("scope = ? AND owner_id = ? AND key = ?", "session:"+strconv.FormatInt(action.SceneRevision, 10), row.ThreadID, identity.Key).Take(&stored).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if stored.RequestHash != identity.RequestHash {
			return sessionstate.ErrIdempotencyConflict
		}
		if stored.Status != "completed" {
			return sessionstate.ErrInvalidState
		}
		found = true
		return json.Unmarshal([]byte(stored.ResultJSON), &receipt)
	})
	if err != nil || !found {
		return sessionstate.RunMutation{}, false, err
	}
	mutation, err := s.runMutation(ctx, id, receipt)
	return mutation, true, err
}

// ClaimRunForResume commits ownership, the current view and the user's receipt
// together. Recovery validation and all file/model work happen before this call.
func (s *Store) ClaimRunForResume(ctx context.Context, expected model.Run, owner string, action sessionstate.RunAction) (sessionstate.RunMutation, bool, error) {
	identity, err := resumeIdentity(expected.ID, action)
	if err != nil || owner == "" {
		if err == nil {
			err = sessionstate.ErrInvalidState
		}
		return sessionstate.RunMutation{}, false, err
	}
	claimed := false
	var claimedRun runPO
	receipt, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: expected.ProjectID, ThreadID: expected.ThreadID, SceneRevision: action.SceneRevision}, identity, func(w *sessionWrite) error {
		var row runPO
		if err := w.tx.Where("id = ? AND thread_id = ?", expected.ID, w.head.ID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.SceneRevision != action.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		if row.EntityRevision != action.EntityRevision || row.ExecutionRevision != expected.ExecutionRevision || row.CheckpointRevision != expected.CheckpointRevision {
			return sessionstate.ErrVersionConflict
		}
		eligible, err := canContinueRun(w.tx, row)
		if err != nil {
			return err
		}
		if !eligible {
			return run.ErrRunNotContinuable
		}
		if err := w.tx.Table("runs").Where("id = ?", row.ID).Updates(map[string]any{
			"status": string(model.RunRecovering), "owner_instance_id": owner, "execution_revision": row.ExecutionRevision + 1,
			"cancel_requested_at": nil, "cancel_request_id": nil, "cancel_reason": nil, "pause_reason": "", "paused_at": nil,
			"terminal_json": nil, "terminal_ref": nil, "final_message_id": nil, "progress_json": nil, "continuation_allowed": 0,
		}).Error; err != nil {
			return err
		}
		if err := w.tx.Table("threads").Where("id = ?", w.head.ID).Updates(map[string]any{"suggest_version": gorm.Expr("suggest_version + 1"), "next_input_suggestions_json": nil}).Error; err != nil {
			return err
		}
		if err := w.publishRun(row.ID); err != nil {
			return err
		}
		if err := w.tx.Where("id = ?", row.ID).Take(&claimedRun).Error; err != nil {
			return err
		}
		w.result, err = json.Marshal(map[string]string{"run_id": row.ID, "request_key": action.RequestKey})
		claimed = err == nil
		return err
	})
	if err != nil {
		return sessionstate.RunMutation{}, false, err
	}
	if claimed {
		return sessionstate.RunMutation{Run: claimedRun.view(), Current: claimedRun.toModel(), Receipt: receipt}, true, nil
	}
	mutation, err := s.runMutation(ctx, expected.ID, receipt)
	return mutation, claimed, err
}

func (s *Store) runMutation(ctx context.Context, id string, receipt sessionstate.Receipt) (sessionstate.RunMutation, error) {
	result := sessionstate.RunMutation{Receipt: receipt}
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runPO
		if err := tx.Where("id = ?", id).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, row.ThreadID)
		if err != nil {
			return err
		}
		if head.SceneRevision != receipt.SceneRevision || row.SceneRevision != receipt.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		result.Current, result.Run = row.toModel(), row.view()
		allowed, err := canContinueRun(tx, row)
		result.Current.CanContinue, result.Run.CanContinue = allowed, allowed
		return err
	})
	return result, err
}

// RequestRunCancel commits the intent, reason, public projection and receipt in
// one transaction. A replay reaches the receipt before checking entity CAS.
func (s *Store) RequestRunCancel(ctx context.Context, id string, reason model.RunCancelReason, action sessionstate.RunAction) (sessionstate.RunMutation, error) {
	if !action.Valid() || (reason != model.RunCancelUserRequested && reason != model.RunCancelSuperseded) {
		return sessionstate.RunMutation{}, sessionstate.ErrInvalidState
	}
	var expected runPO
	if err := s.reader.WithContext(ctx).Where("id = ?", id).Take(&expected).Error; err != nil {
		return sessionstate.RunMutation{}, mapErr(err)
	}
	hash, err := commandHash(struct {
		RunID  string
		Reason model.RunCancelReason
		Action sessionstate.RunAction
	}{id, reason, action})
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	receipt, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: expected.ProjectID, ThreadID: expected.ThreadID, SceneRevision: action.SceneRevision}, sessionstate.CommitIdentity{Key: "cancel-run:" + action.RequestKey, RequestHash: hash}, func(w *sessionWrite) error {
		var row runPO
		if err := w.tx.Where("id = ? AND thread_id = ?", id, w.head.ID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.SceneRevision != action.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		if row.EntityRevision != action.EntityRevision {
			return sessionstate.ErrVersionConflict
		}
		if !model.RunStatus(row.Status).Terminal() {
			if row.CancelRequestedAt == nil {
				row.CancelRequestedAt = &w.now
				row.CancelRequestID, row.CancelReason = stringPointer(action.RequestKey), stringPointer(string(reason))
				if err := w.tx.Table("runs").Where("id = ?", id).Updates(map[string]any{
					"cancel_requested_at": w.now, "cancel_request_id": action.RequestKey, "cancel_reason": string(reason),
				}).Error; err != nil {
					return err
				}
			}
			if row.Status == string(model.RunPaused) {
				// There is no live worker. Tool outcomes stay in their durable rows;
				// ending the Run does not fabricate canceled tool observations.
				payload := model.NewRunTerminalPayload(id, row.ActiveDurationMS, nil, nil)
				payload.Reason = model.RunCancelReason(textOrEmpty(row.CancelReason))
				raw, err := json.Marshal(payload)
				if err != nil {
					return err
				}
				if err := w.applyRunTerminal(row, model.EventRunCanceled, raw); err != nil {
					return err
				}
				if err := w.tx.Table("runs").Where("id = ?", id).Updates(map[string]any{"pause_reason": "", "paused_at": nil}).Error; err != nil {
					return err
				}
			} else {
				progress, err := json.Marshal(model.RunProgressPayload{PublicEventBase: model.NewPublicEventBase(id), Activity: model.ActivityRunCanceling})
				if err != nil {
					return err
				}
				if err := w.tx.Table("runs").Where("id = ?", id).Update("progress_json", string(progress)).Error; err != nil {
					return err
				}
			}
			if err := w.publishRun(id); err != nil {
				return err
			}
		}
		w.result, err = json.Marshal(map[string]string{"run_id": id, "request_key": action.RequestKey})
		return err
	})
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	return s.runMutation(ctx, id, receipt)
}
