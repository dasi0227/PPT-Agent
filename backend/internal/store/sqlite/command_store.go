package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

func (s *Store) AcceptCommand(ctx context.Context, threadID string, request model.CommandRequest, sceneRevision int64) (model.CommandExecution, bool, error) {
	var accepted model.CommandExecution
	created := false
	source := request.Source
	if source == "" {
		source = "user"
	}
	if source != "user" && source != "automatic" {
		return accepted, false, errors.New("invalid command source")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return accepted, false, err
	}
	hash := sha256.Sum256(raw)
	requestHash := hex.EncodeToString(hash[:])
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt idempotencyPO
		err := tx.First(&receipt, "scope = ? AND owner_id = ? AND key = ?", "command", threadID, request.RequestKey).Error
		if err == nil {
			if receipt.RequestHash != requestHash {
				return model.NewAgentError("IDEMPOTENCY_KEY_REUSED", "command", nil)
			}
			return json.Unmarshal([]byte(receipt.ResultJSON), &accepted)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := s.checkAcceptance(tx, threadID); err != nil {
			return err
		}
		var thread threadPO
		if err := tx.First(&thread, "id = ?", threadID).Error; err != nil {
			return mapErr(err)
		}
		commandID := request.CommandID
		if commandID == "" {
			commandID = model.MustShortID("cmd")
		}
		attempt := 1
		var previous commandExecutionPO
		err = tx.Where("command_id = ?", commandID).Order("attempt_no DESC").First(&previous).Error
		if err == nil {
			if previous.ThreadID != threadID || previous.Kind != request.Kind || activeCommandStatus(previous.Status) {
				return store.ErrCommandConflict
			}
			attempt = previous.AttemptNo + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if request.BaseAttemptID != "" {
			var base commandExecutionPO
			if err := tx.First(&base, "id = ? AND command_id = ? AND status = ?", request.BaseAttemptID, commandID, "completed").Error; err != nil {
				return store.ErrCommandConflict
			}
		}
		now := time.Now().UnixMilli()
		accepted = model.CommandExecution{PreviousTitle: thread.Title, OwnerInstanceID: s.instanceID, ExecutionRevision: 1, SceneRevision: sceneRevision, CommandID: commandID, AttemptID: model.MustShortID("attempt"), AttemptNo: attempt, ThreadID: threadID, ProjectID: thread.ProjectID, Kind: request.Kind, Source: source, Status: "accepted", Phase: -1, Input: request.Input, BaseAttemptID: request.BaseAttemptID, Feedback: request.Feedback, CreatedAt: now, UpdatedAt: now}
		payload, err := json.Marshal(accepted)
		if err != nil {
			return err
		}
		event, err := s.enqueueEvent(tx, threadID, threadjournal.Event{Type: "command.accepted", CommandID: commandID, AttemptID: accepted.AttemptID, Payload: payload})
		if err != nil {
			return err
		}
		row := commandExecutionPO{ID: accepted.AttemptID, CommandID: commandID, AttemptNo: attempt, ThreadID: threadID, ProjectID: thread.ProjectID, Kind: request.Kind, Source: source, Status: "accepted", Phase: "-1", OwnerInstanceID: s.instanceID, ExecutionRevision: 1, SceneRevision: sceneRevision, StartEventSeq: event.Seq, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return mapProjectWriteErr(err)
		}
		receiptPayload, _ := json.Marshal(map[string]string{"command_id": commandID, "attempt_id": accepted.AttemptID})
		if err := tx.Create(&idempotencyPO{Scope: "command", OwnerID: threadID, Key: request.RequestKey, RequestHash: requestHash, Status: "completed", ResultJSON: string(receiptPayload)}).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	if err == nil {
		err = s.FlushThreadEvents(ctx, threadID)
	}
	if err == nil && !created {
		var row commandExecutionPO
		err = s.db.WithContext(ctx).First(&row, "id = ? AND command_id = ?", accepted.AttemptID, accepted.CommandID).Error
		if err == nil {
			accepted, err = s.commandProjection(ctx, row)
		}
	}
	return accepted, created, err
}
func (s *Store) commandProjection(ctx context.Context, row commandExecutionPO) (model.CommandExecution, error) {
	event, err := eventInTransaction(s.db.WithContext(ctx), row.ThreadID, row.StartEventSeq)
	if err != nil {
		return model.CommandExecution{}, err
	}
	var out model.CommandExecution
	if err := json.Unmarshal(event.Payload, &out); err != nil {
		return out, err
	}
	if row.TerminalEventSeq != nil {
		terminal, err := eventInTransaction(s.db.WithContext(ctx), row.ThreadID, *row.TerminalEventSeq)
		if err != nil {
			return out, err
		}
		var result model.CommandExecution
		if err := json.Unmarshal(terminal.Payload, &result); err != nil {
			return out, err
		}
		out.Result = result.Result
		out.Error = result.Error
	}
	out.OwnerInstanceID = row.OwnerInstanceID
	out.ExecutionRevision = row.ExecutionRevision
	out.SceneRevision = row.SceneRevision
	out.CommandID = row.CommandID
	out.AttemptID = row.ID
	out.Status = row.Status
	out.UpdatedAt = row.UpdatedAt
	out.Phase, _ = strconv.Atoi(row.Phase)
	return out, nil
}
func (s *Store) GetCommand(ctx context.Context, id string) (model.CommandExecution, error) {
	var row commandExecutionPO
	if err := s.db.WithContext(ctx).Where("command_id = ?", id).Order("attempt_no DESC").First(&row).Error; err != nil {
		return model.CommandExecution{}, mapErr(err)
	}
	out, err := s.commandProjection(ctx, row)
	if err != nil {
		return out, err
	}
	var success commandExecutionPO
	err = s.db.WithContext(ctx).Where("command_id = ? AND status = ?", id, "completed").Order("attempt_no DESC").First(&success).Error
	if err == nil {
		last, err := s.commandProjection(ctx, success)
		if err != nil {
			return out, err
		}
		out.LatestSuccess = &last
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	return out, nil
}
func (s *Store) SaveCommandExecution(ctx context.Context, execution model.CommandExecution) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return s.saveCommandExecution(tx, execution) })
	if err == nil {
		err = s.FlushThreadEvents(ctx, execution.ThreadID)
	}
	if err != nil && !errors.Is(err, store.ErrCommandConflict) && !errors.Is(err, context.Canceled) {
		s.deliveryFailures.Store(execution.ThreadID, err)
	}
	return err
}

func (s *Store) HasActiveCommand(ctx context.Context, projectID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&commandExecutionPO{}).Where("project_id = ? AND status IN ?", projectID, []string{"accepted", "running", "cancel_requested"}).Count(&n).Error
	return n > 0, err
}

func (s *Store) saveCommandExecution(tx *gorm.DB, execution model.CommandExecution) error {
	var row commandExecutionPO
	if err := tx.First(&row, "id = ? AND command_id = ?", execution.AttemptID, execution.CommandID).Error; err != nil {
		return mapErr(err)
	}
	if !activeCommandStatus(row.Status) || row.OwnerInstanceID != execution.OwnerInstanceID || row.ExecutionRevision != execution.ExecutionRevision || row.SceneRevision != execution.SceneRevision {
		return store.ErrCommandConflict
	}
	if row.Status == "cancel_requested" && execution.Status == "running" {
		return context.Canceled
	}
	execution.UpdatedAt = time.Now().UnixMilli()
	raw, err := json.Marshal(execution)
	if err != nil {
		return err
	}
	event, err := s.enqueueEvent(tx, row.ThreadID, threadjournal.Event{Type: "command." + execution.Status, CommandID: row.CommandID, AttemptID: row.ID, Payload: raw})
	if err != nil {
		return err
	}
	updates := map[string]any{"status": execution.Status, "phase": strconv.Itoa(execution.Phase), "updated_at": execution.UpdatedAt}
	if !activeCommandStatus(execution.Status) {
		updates["terminal_event_seq"] = event.Seq
	}
	return tx.Model(&commandExecutionPO{}).Where("id = ?", row.ID).Updates(updates).Error
}
func (s *Store) RequestCommandCancel(ctx context.Context, id, attempt, key string) (model.CommandExecution, error) {
	var out model.CommandExecution
	var threadID string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row commandExecutionPO
		if err := tx.Where("command_id = ?", id).Order("attempt_no DESC").First(&row).Error; err != nil {
			return mapErr(err)
		}
		threadID = row.ThreadID
		if row.ID != attempt {
			return store.ErrCommandConflict
		}
		var receipt idempotencyPO
		err := tx.First(&receipt, "scope = ? AND owner_id = ? AND key = ?", "cancel_command", row.ThreadID, key).Error
		hash := id + ":" + attempt
		if err == nil && receipt.RequestHash != hash {
			return model.NewAgentError("IDEMPOTENCY_KEY_REUSED", "cancel_command", nil)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		event, err := eventInTransaction(tx, row.ThreadID, row.StartEventSeq)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(event.Payload, &out); err != nil {
			return err
		}
		out.CommandID = row.CommandID
		out.AttemptID = row.ID
		out.ThreadID = row.ThreadID
		out.Status = row.Status
		out.OwnerInstanceID = row.OwnerInstanceID
		out.ExecutionRevision = row.ExecutionRevision
		out.SceneRevision = row.SceneRevision
		if activeCommandStatus(row.Status) && row.Status != "cancel_requested" {
			out.Status = "cancel_requested"
			if err := s.saveCommandExecution(tx, out); err != nil {
				return err
			}
		}
		if receipt.Key == "" {
			return tx.Create(&idempotencyPO{Scope: "cancel_command", OwnerID: row.ThreadID, Key: key, RequestHash: hash, Status: "completed", ResultJSON: `{}`}).Error
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	if err := s.FlushThreadEvents(ctx, threadID); err != nil {
		return out, err
	}
	return s.GetCommand(ctx, id)
}
