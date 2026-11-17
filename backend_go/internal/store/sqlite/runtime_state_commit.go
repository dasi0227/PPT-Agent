package sqlite

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// CommitRuntimeState publishes the native context and the exact control-flow
// position that consumes it in one transaction. A failure publishes neither.
func (s *Store) CommitRuntimeState(ctx context.Context, fence sessionstate.Fence, identity sessionstate.CommitIdentity, update *sessionstate.ContextUpdate, checkpoint workflow.RuntimeCheckpoint) (sessionstate.Receipt, error) {
	if fence.RunID == "" || checkpoint.RunID != fence.RunID || checkpoint.OwnerInstanceID != fence.OwnerInstanceID || checkpoint.ExecutionRevision != fence.ExecutionRevision {
		return sessionstate.Receipt{}, sessionstate.ErrVersionConflict
	}
	return s.commitSession(ctx, fence, identity, func(w *sessionWrite) error {
		if update != nil {
			candidate := *update
			candidate.NewMessages = slices.Clone(update.NewMessages)
			if err := attachPublicMessages(&candidate, checkpoint.PublicUpdates); err != nil {
				return err
			}
			if err := w.bindContextInputs(fence.RunID, &candidate); err != nil {
				return err
			}
			if err := w.replaceContext(candidate); err != nil {
				return err
			}
		} else if checkpoint.ContextRevision != w.head.ContextRevision {
			return sessionstate.ErrVersionConflict
		}
		checkpoint.ContextRevision = w.head.ContextRevision
		if err := w.checkpointModelCall(checkpoint); err != nil {
			return err
		}
		if err := w.checkpointTools(checkpoint); err != nil {
			return err
		}
		for _, fact := range checkpoint.PublicUpdates {
			if fact.Type != model.EventContextCompacted {
				continue
			}
			if update == nil || checkpoint.Boundary != "context_compacted" {
				return sessionstate.ErrInvalidState
			}
			var value model.ContextCompactedPayload
			if err := json.Unmarshal(fact.Payload, &value); err != nil {
				return err
			}
			if value.Compaction.RunID != fence.RunID || value.Compaction.Trigger != model.ContextCompactionAuto {
				return sessionstate.ErrInvalidState
			}
			if err := w.recordCompaction(value.Compaction, update.ExpectedRevision); err != nil {
				return err
			}
		}
		if err := w.checkpointInteractions(checkpoint); err != nil {
			return err
		}
		expectedScope := checkpoint.Scope.Revision
		changes := map[string]any{"context_revision": w.head.ContextRevision}
		if checkpoint.Phase == workflow.PhaseWaitingInput {
			changes["status"] = string(model.RunWaiting)
		} else {
			changes["status"] = string(model.RunRunning)
		}
		if checkpoint.ScopeTransition {
			if expectedScope < 2 {
				return sessionstate.ErrVersionConflict
			}
			expectedScope--
			changes["scope_revision"] = checkpoint.Scope.Revision
			changes["status"] = string(model.RunRunning)
		}
		if checkpoint.ModeTransition {
			var mode string
			if err := w.tx.Table("runs").Select("mode").Where("id = ?", fence.RunID).Scan(&mode).Error; err != nil {
				return err
			}
			if mode != string(model.ModePlan) || checkpoint.Mode != model.ModeCreate {
				return sessionstate.ErrVersionConflict
			}
			changes["mode"] = string(checkpoint.Mode)
			changes["status"] = string(model.RunRunning)
		}
		if err := writeCheckpoint(w.tx, checkpoint, expectedScope, changes); err != nil {
			return err
		}
		w.checkpointRevision = checkpoint.CheckpointRevision + 1
		for _, update := range checkpoint.PublicUpdates {
			var row runPO
			if err := w.tx.Where("id = ?", fence.RunID).Take(&row).Error; err != nil {
				return err
			}
			if err := w.applyRunUpdate(row, update); err != nil {
				return err
			}
		}
		return w.publishRun(fence.RunID)
	})
}

// Public final content and its native message are formed together, so immutable
// message rows never need to be rewritten after their context is committed.
func attachPublicMessages(update *sessionstate.ContextUpdate, facts []sessionstate.RunUpdate) error {
	for _, fact := range facts {
		if fact.Type != model.EventMessageFinal {
			continue
		}
		var value model.MessageFinalPayload
		if err := json.Unmarshal(fact.Payload, &value); err != nil {
			return err
		}
		found := false
		for i := range update.NewMessages {
			message := &update.NewMessages[i]
			if message.ID == value.MessageID {
				if message.Native.Text() != value.Text {
					return sessionstate.ErrInvalidState
				}
				message.Public = fact.Payload
				found = true
				break
			}
		}
		if !found {
			return sessionstate.ErrInvalidState
		}
	}
	return nil
}

// The injected marker and its native instruction are one durable fact. The
// public user message stays immutable and is not duplicated in the timeline.
func (w *sessionWrite) bindContextInputs(runID string, update *sessionstate.ContextUpdate) error {
	rounds := map[string]string{}
	for i := range update.NewMessages {
		message := &update.NewMessages[i]
		if message.RoundID == message.RunID {
			id, ok := rounds[message.RunID]
			if !ok {
				var row conversationRoundPO
				if err := w.tx.Where("thread_id = ? AND run_id = ?", w.head.ID, message.RunID).Order("opened_seq DESC").Take(&row).Error; err != nil {
					return err
				}
				id = row.ID
				rounds[message.RunID] = id
			}
			message.RoundID = id
		}
		metadata := message.Native.Metadata
		if metadata == nil || metadata.RunID != runID || metadata.Origin != "user" || (metadata.Kind != "instruction" && metadata.Kind != "steering") {
			continue
		}
		var input inputPO
		query := w.tx.Where("thread_id = ?", w.head.ID)
		if metadata.Kind == "instruction" {
			query = query.Where("id = (SELECT input_id FROM runs WHERE id = ?)", runID)
		} else {
			query = query.Where("run_id = ? AND client_message_id = ?", runID, metadata.Key)
		}
		if err := query.Take(&input).Error; err != nil {
			return err
		}
		if input.Disposition != model.InputTask {
			return sessionstate.ErrInvalidState
		}
		if err := w.tx.Model(&conversationRoundPO{}).Where("thread_id = ? AND run_id = ? AND closed_seq IS NULL", w.head.ID, runID).Update("has_task_input", 1).Error; err != nil {
			return err
		}
		var origin struct{ RoundID string }
		if err := w.tx.Table("conversation_messages").Select("round_id").Where("id = ? AND thread_id = ?", input.MessageID, w.head.ID).Take(&origin).Error; err != nil {
			return err
		}
		message.InputID, message.RoundID = input.ID, origin.RoundID
		if input.DeliveryState == "injected" {
			continue
		}
		input.DeliveryState, input.UpdatedAt = "injected", w.now
		if err := w.tx.Save(&input).Error; err != nil {
			return err
		}
		if err := w.tx.Model(&steeringPO{}).Where("run_id = ? AND input_id = ?", runID, input.ID).Updates(map[string]any{"status": "injected", "rejection_code": ""}).Error; err != nil {
			return err
		}
		var command model.RunCommand
		if err := json.Unmarshal([]byte(input.ReferenceJSON), &command); err != nil {
			return err
		}
		if err := w.projectInput(input, command, "run:"+runID); err != nil {
			return err
		}
		if err := w.tx.Table("threads").Where("id = ?", w.head.ID).Update("last_context_run_id", runID).Error; err != nil {
			return err
		}
	}
	return nil
}
