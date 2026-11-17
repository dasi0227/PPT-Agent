package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

// Chat steering is acknowledged by its saved decision and is never injected.
const pendingSteering = `steering_inbox.status = 'accepted'
 AND steering_inbox.input_id <> (SELECT input_id FROM runs WHERE id = steering_inbox.run_id)
 AND NOT EXISTS (
 SELECT 1 FROM thread_inputs AS input WHERE input.id = steering_inbox.input_id AND input.disposition = 'chat')`

func readSteering(query *gorm.DB) ([]model.SteeringMessage, error) {
	var rows []struct {
		RunID           string
		ThreadID        string
		ClientMessageID string
		RequestHash     string
		InputID         string
		AcceptedSeq     int64
		Status          string
		RejectionCode   string
		ReferenceJSON   string
	}
	err := query.Table("steering_inbox").
		Select("steering_inbox.run_id, steering_inbox.thread_id, steering_inbox.client_message_id, steering_inbox.request_hash, steering_inbox.input_id, steering_inbox.accepted_seq, steering_inbox.status, steering_inbox.rejection_code, input.reference_json").
		Joins("JOIN thread_inputs AS input ON input.id = steering_inbox.input_id AND input.thread_id = steering_inbox.thread_id").
		Order("steering_inbox.accepted_seq ASC, steering_inbox.client_message_id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]model.SteeringMessage, 0, len(rows))
	for _, row := range rows {
		var command model.RunCommand
		if err := json.Unmarshal([]byte(row.ReferenceJSON), &command); err != nil {
			return nil, err
		}
		out = append(out, model.SteeringMessage{InputID: row.InputID, AcceptedSeq: row.AcceptedSeq, RunID: row.RunID, ThreadID: row.ThreadID, ClientMessageID: row.ClientMessageID, RequestHash: row.RequestHash,
			Content: command.Instruction, Attachments: command.Attachments, DOMSelections: command.DOMSelections, ReferenceOrder: command.ReferenceOrder, Scope: command.Scope, Status: model.SteeringStatus(row.Status), RejectionCode: row.RejectionCode})
	}
	return out, nil
}

func (s *Store) CreateSteering(ctx context.Context, message model.SteeringMessage) (model.SteeringMessage, bool, error) {
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return model.SteeringMessage{}, false, sessionstate.ErrSceneChanged
	}
	var row runPO
	if err := s.reader.WithContext(ctx).Where("id = ? AND thread_id = ?", message.RunID, message.ThreadID).Take(&row).Error; err != nil {
		return model.SteeringMessage{}, false, mapErr(err)
	}
	if message.ClientMessageID == "" || message.RequestHash == "" {
		return model.SteeringMessage{}, false, sessionstate.ErrInvalidState
	}
	created := false
	_, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: row.ProjectID, ThreadID: row.ThreadID, SceneRevision: scene}, sessionstate.CommitIdentity{Key: "accept-steering:" + message.ClientMessageID, RequestHash: message.RequestHash}, func(w *sessionWrite) error {
		if err := w.tx.Where("id = ?", message.RunID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.SceneRevision != scene {
			return sessionstate.ErrSceneChanged
		}
		if (row.Status != string(model.RunPending) && row.Status != string(model.RunRunning) && row.Status != string(model.RunWaiting)) || row.CancelRequestedAt != nil {
			return run.ErrRunNotRunning
		}
		command := row.Command
		command.Instruction, command.Attachments, command.DOMSelections, command.ReferenceOrder, command.Scope = message.Content, message.Attachments, message.DOMSelections, message.ReferenceOrder, message.Scope
		input, err := w.acceptInput(model.MustShortID("input"), message.ClientMessageID, row.ID, message.RequestHash, command)
		if err != nil {
			return err
		}
		entry := steeringPO{RunID: row.ID, ThreadID: row.ThreadID, ClientMessageID: message.ClientMessageID, RequestHash: message.RequestHash, InputID: input.ID, AcceptedSeq: w.seq, Status: string(model.SteeringAccepted)}
		if err := w.tx.Create(&entry).Error; err != nil {
			return err
		}
		if err := w.tx.Table("threads").Where("id = ?", row.ThreadID).Updates(map[string]any{"suggest_version": gorm.Expr("suggest_version + 1"), "next_input_suggestions_json": nil}).Error; err != nil {
			return err
		}
		w.result, err = json.Marshal(map[string]string{"input_id": input.ID, "run_id": row.ID})
		if err != nil {
			return err
		}
		created = true
		return w.publishThread()
	})
	if err != nil {
		return model.SteeringMessage{}, false, err
	}
	out, err := readSteering(s.reader.WithContext(ctx).Where("steering_inbox.thread_id = ? AND steering_inbox.client_message_id = ?", message.ThreadID, message.ClientMessageID))
	if err != nil {
		return model.SteeringMessage{}, false, err
	}
	if len(out) != 1 {
		return model.SteeringMessage{}, false, run.ErrRunNotFound
	}
	return out[0], created, nil
}

func (s *Store) ListPendingSteering(ctx context.Context, runID string) ([]model.SteeringMessage, error) {
	return readSteering(s.reader.WithContext(ctx).Where("steering_inbox.run_id = ?", runID).Where(pendingSteering))
}

// Injection is only committed together with the checkpoint that contains the
// native instruction. This standalone entry point is for terminal rejection.
func (s *Store) MarkSteering(ctx context.Context, runID string, ids []string, status model.SteeringStatus, _ int64, rejectionCode string) error {
	if len(ids) == 0 {
		return nil
	}
	if status != model.SteeringRejected {
		return sessionstate.ErrInvalidState
	}
	var row runPO
	if err := s.reader.WithContext(ctx).Where("id = ?", runID).Take(&row).Error; err != nil {
		return mapErr(err)
	}
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok || scene != row.SceneRevision {
		return sessionstate.ErrSceneChanged
	}
	ids = slices.Clone(ids)
	slices.Sort(ids)
	hash, err := commandHash(struct {
		IDs    []string
		Status model.SteeringStatus
		Code   string
	}{ids, status, rejectionCode})
	if err != nil {
		return err
	}
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: row.ProjectID, ThreadID: row.ThreadID, SceneRevision: scene}, sessionstate.CommitIdentity{Key: fmt.Sprintf("reject-steering:%s:%d:%s", runID, row.ExecutionRevision, hash), RequestHash: hash}, func(w *sessionWrite) error {
		var current runPO
		if err := w.tx.Where("id = ?", runID).Take(&current).Error; err != nil {
			return mapErr(err)
		}
		if owner, execution, ok := workflow.CheckpointOwnership(ctx); ok && (current.OwnerInstanceID != owner || current.ExecutionRevision != execution) {
			return run.ErrRunRevisionConflict
		}
		if current.ExecutionRevision != row.ExecutionRevision {
			return run.ErrRunRevisionConflict
		}
		return w.markSteering(runID, ids, status, rejectionCode)
	})
	return err
}

func (w *sessionWrite) markSteering(runID string, ids []string, status model.SteeringStatus, rejectionCode string) error {
	if len(ids) == 0 {
		return nil
	}
	var rows []steeringPO
	if err := w.tx.Where("run_id = ? AND client_message_id IN ?", runID, ids).Where(pendingSteering).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		var input inputPO
		if err := w.tx.Where("id = ? AND thread_id = ?", row.InputID, w.head.ID).Take(&input).Error; err != nil {
			return err
		}
		if err := w.tx.Model(&row).Updates(map[string]any{"status": string(status), "rejection_code": rejectionCode}).Error; err != nil {
			return err
		}
		input.DeliveryState, input.UpdatedAt = string(status), w.now
		if err := w.tx.Save(&input).Error; err != nil {
			return err
		}
		var command model.RunCommand
		if err := json.Unmarshal([]byte(input.ReferenceJSON), &command); err != nil {
			return err
		}
		if err := w.projectInput(input, command, "run:"+runID); err != nil {
			return err
		}
	}
	return nil
}
