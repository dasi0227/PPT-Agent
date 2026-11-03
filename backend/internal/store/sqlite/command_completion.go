package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// CompleteCommand commits the prepared business result, command status and
// public projection together. The returned status is the committed status:
// cancellation may have won while the model was producing the candidate.
func (s *Store) CompleteCommand(ctx context.Context, execution model.CommandExecution, effects sessionstate.CommandEffects) (model.CommandExecution, error) {
	if execution.Status != "completed" || !json.Valid(execution.Result) {
		return model.CommandExecution{}, sessionstate.ErrInvalidState
	}
	hash, err := commandHash(struct {
		Execution         model.CommandExecution
		Effects           sessionstate.CommandEffects
		OwnerInstanceID   string
		ExecutionRevision int64
	}{execution, effects, execution.OwnerInstanceID, execution.ExecutionRevision})
	if err != nil {
		return model.CommandExecution{}, err
	}
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: execution.ProjectID, ThreadID: execution.ThreadID, SceneRevision: execution.SceneRevision}, sessionstate.CommitIdentity{Key: "command-complete:" + execution.AttemptID + ":" + strconv.FormatInt(execution.ExecutionRevision, 10), RequestHash: hash}, func(w *sessionWrite) error {
		var row commandExecutionPO
		if err := w.tx.Where("id = ? AND command_id = ?", execution.AttemptID, execution.CommandID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if !activeCommandStatus(row.Status) || row.OwnerInstanceID != execution.OwnerInstanceID || row.ExecutionRevision != execution.ExecutionRevision || row.SceneRevision != execution.SceneRevision {
			return store.ErrCommandConflict
		}
		// Git has already performed an external effect. Its confirmed result
		// must be retained even when a cancellation arrived after dispatch.
		if row.Status == "cancel_requested" && execution.Kind != "commit" {
			execution.Status, execution.Result = "canceled", nil
		}
		if execution.Status == "completed" && execution.Kind == "suggest" {
			if err := checkSuggestCommand(w.tx, execution); err != nil {
				if !errors.Is(err, context.Canceled) {
					return err
				}
				execution.Status, execution.Result = "canceled", nil
			}
		}
		if execution.Status == "completed" {
			if (execution.Kind == "compact") != (effects.Context != nil && effects.Compaction != nil) ||
				(effects.Context != nil) != (effects.Compaction != nil) ||
				(execution.Kind == "handoff") != (effects.Briefing != nil) ||
				(execution.Kind == "rename") != (effects.Rename != nil) {
				return sessionstate.ErrInvalidState
			}
			if effects.Rename != nil {
				applied, err := w.completeRename(*effects.Rename)
				if err != nil {
					return err
				}
				if !applied {
					execution.Status, execution.Result = "canceled", nil
				}
			}
			if effects.Context != nil {
				if err := w.completeCompaction(execution, *effects.Context, *effects.Compaction); err != nil {
					return err
				}
			}
			if effects.Briefing != nil {
				if err := w.appendBriefing(*effects.Briefing); err != nil {
					return err
				}
			}
		}
		if err := w.writeCommand(row, execution); err != nil {
			return err
		}
		if err := w.tx.Where("id = ?", execution.AttemptID).Take(&row).Error; err != nil {
			return err
		}
		committed, err := row.toModel()
		if err != nil {
			return err
		}
		w.result, err = json.Marshal(committed)
		return err
	})
	if err != nil {
		return model.CommandExecution{}, err
	}
	// A receipt confirms a transaction, not continued ownership after recovery.
	// Recheck the exact attempt and return its private fence to the worker too.
	committed, err := s.commandAtExecution(ctx, execution)
	if err == nil && activeCommandStatus(committed.Status) {
		return model.CommandExecution{}, store.ErrCommandConflict
	}
	return committed, err
}
