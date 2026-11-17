package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

type toolExecutionPO struct {
	ID                string `gorm:"primaryKey"`
	RunID             string
	CallID            string
	AttemptNo         int
	ThreadID          string
	SceneRevision     int64
	ExecutionRevision int64
	ToolName          string
	ArgumentsJSON     string
	ArgumentsHash     string
	Status            string
	RecoveryPolicy    string
	OperationID       *string
	ResultJSON        *string
	ResultMessageID   *string
	StartedAt         *int64
	FinishedAt        *int64
	EntityRevision    int64
	UpdatedSeq        int64
}

func (toolExecutionPO) TableName() string { return "tool_executions" }
func (p toolExecutionPO) value() sessionstate.ToolExecution {
	return sessionstate.ToolExecution{ID: p.ID, RunID: p.RunID, CallID: p.CallID, AttemptNo: p.AttemptNo, Name: p.ToolName, Arguments: json.RawMessage(p.ArgumentsJSON), ArgumentsHash: p.ArgumentsHash, Status: p.Status, RecoveryPolicy: sessionstate.ToolRecoveryPolicy(p.RecoveryPolicy), OperationID: textOrEmpty(p.OperationID), ResultJSON: rawOrNull(p.ResultJSON), ResultMessageID: textOrEmpty(p.ResultMessageID), EntityRevision: p.EntityRevision}
}

func (s *Store) GetToolExecution(ctx context.Context, runID, callID string) (sessionstate.ToolExecution, error) {
	var value sessionstate.ToolExecution
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row toolExecutionPO
		if err := tx.Where("run_id = ? AND call_id = ?", runID, callID).Order("attempt_no DESC").Take(&row).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, row.ThreadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		value = row.value()
		_, err = value.Call()
		return err
	})
	return value, err
}

func (w *sessionWrite) checkpointTools(cp workflow.RuntimeCheckpoint) error {
	for _, next := range cp.ToolChanges {
		if cp.PendingToolBatch == nil || !slices.Contains(cp.PendingToolBatch.CallIDs, next.CallID) {
			return sessionstate.ErrInvalidState
		}
		if next.RunID != cp.RunID || next.AttemptNo < 1 || next.ID != sessionstate.ToolAttemptID(cp.RunID, next.CallID, next.AttemptNo) {
			return sessionstate.ErrInvalidState
		}
		if _, err := next.Call(); err != nil {
			return err
		}
		switch next.RecoveryPolicy {
		case sessionstate.ToolReadOnly, sessionstate.ToolIdempotent, sessionstate.ToolReconcile, sessionstate.ToolUnknown:
		default:
			return sessionstate.ErrInvalidState
		}
		var previous toolExecutionPO
		err := w.tx.Where("run_id = ? AND call_id = ?", cp.RunID, next.CallID).Order("attempt_no DESC").Take(&previous).Error
		create := errors.Is(err, gorm.ErrRecordNotFound) || err == nil && next.AttemptNo == previous.AttemptNo+1
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if create {
			if next.Status != "prepared" || next.EntityRevision != 0 || len(next.ResultJSON) != 0 || next.ResultMessageID != "" || cp.PendingToolBatch == nil || !slices.Contains(cp.PendingToolBatch.CallIDs, next.CallID) {
				return sessionstate.ErrInvalidState
			}
			if previous.ID == "" && next.AttemptNo != 1 {
				return sessionstate.ErrInvalidState
			}
			if previous.ID != "" && (previous.Status != "outcome_unknown" || previous.RecoveryPolicy != string(sessionstate.ToolReadOnly) || previous.ArgumentsHash != next.ArgumentsHash || previous.ToolName != next.Name || previous.RecoveryPolicy != string(next.RecoveryPolicy)) {
				return sessionstate.ErrVersionConflict
			}
			row := toolExecutionPO{ID: next.ID, RunID: cp.RunID, CallID: next.CallID, AttemptNo: next.AttemptNo, ThreadID: w.head.ID, SceneRevision: w.head.SceneRevision, ExecutionRevision: cp.ExecutionRevision, ToolName: next.Name, ArgumentsJSON: string(next.Arguments), ArgumentsHash: next.ArgumentsHash, Status: "prepared", RecoveryPolicy: string(next.RecoveryPolicy), OperationID: stringPointer(next.OperationID), EntityRevision: 1, UpdatedSeq: w.seq}
			if err := w.tx.Create(&row).Error; err != nil {
				return err
			}
			continue
		}
		if previous.ID != next.ID || previous.ThreadID != w.head.ID || previous.EntityRevision != next.EntityRevision || previous.ArgumentsHash != next.ArgumentsHash || previous.ToolName != next.Name || previous.RecoveryPolicy != string(next.RecoveryPolicy) || textOrEmpty(previous.OperationID) != next.OperationID {
			return sessionstate.ErrVersionConflict
		}
		if previous.value().Terminal() {
			return sessionstate.ErrVersionConflict
		}
		updates := map[string]any{"status": next.Status, "entity_revision": previous.EntityRevision + 1, "updated_seq": w.seq, "scene_revision": w.head.SceneRevision, "execution_revision": cp.ExecutionRevision}
		switch next.Status {
		case "executing":
			if previous.Status != "prepared" && previous.Status != "outcome_unknown" {
				return sessionstate.ErrVersionConflict
			}
			// An unknown mutation may only resume a persisted approval candidate.
			if previous.Status == "outcome_unknown" && (cp.PendingCommand == nil || cp.PendingCommand.CallID != next.CallID) && (cp.PendingResourceApproval == nil || cp.PendingResourceApproval.CallID != next.CallID) {
				return sessionstate.ErrVersionConflict
			}
			var run struct{ CancelRequestedAt *int64 }
			if err := w.tx.Table("runs").Select("cancel_requested_at").Where("id = ?", cp.RunID).Take(&run).Error; err != nil {
				return err
			}
			if run.CancelRequestedAt != nil {
				return context.Canceled
			}
			updates["started_at"] = w.now
		case "outcome_unknown":
			if previous.Status != "executing" && previous.Status != "outcome_unknown" {
				return sessionstate.ErrVersionConflict
			}
		case "succeeded", "failed", "canceled":
			if len(next.ResultJSON) == 0 || !json.Valid(next.ResultJSON) || next.ResultMessageID == "" {
				return sessionstate.ErrInvalidState
			}
			var body struct {
				Result struct {
					OK      *bool   `json:"ok"`
					Applied *bool   `json:"applied"`
					Summary *string `json:"summary"`
				} `json:"result"`
			}
			if json.Unmarshal(next.ResultJSON, &body) != nil || body.Result.OK == nil || body.Result.Applied == nil || body.Result.Summary == nil || (*body.Result.OK != (next.Status == "succeeded")) {
				return sessionstate.ErrInvalidState
			}
			message, err := checkpointNativeMessage(w.tx, w.head.ID, w.head.ContextRevision, next.ResultMessageID)
			if err != nil {
				return err
			}
			if message.Role != llm.RoleTool || message.ToolCallID != next.CallID {
				return sessionstate.ErrInvalidState
			}
			if previous.Status == "outcome_unknown" {
				var receipt idempotencyPO
				if previous.RecoveryPolicy != string(sessionstate.ToolReconcile) || next.OperationID == "" {
					return sessionstate.ErrInvalidState
				}
				if err := w.tx.Where("scope='artifact_commit' AND owner_id=? AND key=? AND status='completed'", cp.RunID, next.OperationID).Take(&receipt).Error; err != nil {
					return err
				}
				var recorded, candidate any
				if json.Unmarshal([]byte(receipt.ResultJSON), &recorded) != nil || json.Unmarshal(next.ResultJSON, &candidate) != nil {
					return sessionstate.ErrInvalidState
				}
				// Observation binding can add derived fields, but confirmed effects
				// cannot be replaced by a failure or a different result body.
				recordedResult, _ := recorded.(map[string]any)
				candidateResult, _ := candidate.(map[string]any)
				left, _ := json.Marshal(recordedResult["result"])
				right, _ := json.Marshal(candidateResult["result"])
				if string(left) != string(right) {
					return sessionstate.ErrIdempotencyConflict
				}
			}
			updates["result_json"], updates["result_message_id"], updates["finished_at"] = string(next.ResultJSON), next.ResultMessageID, w.now
		default:
			return sessionstate.ErrInvalidState
		}
		if err := w.tx.Model(&previous).Updates(updates).Error; err != nil {
			return err
		}
	}
	if cp.PendingToolBatch != nil {
		return validateCheckpointToolBatch(w.tx, w.head.ID, cp)
	}
	var unfinished int64
	if err := w.tx.Table("tool_executions AS t").Where("run_id = ? AND status IN ('prepared','executing','outcome_unknown') AND NOT EXISTS (SELECT 1 FROM tool_executions AS n WHERE n.run_id = t.run_id AND n.call_id = t.call_id AND n.attempt_no > t.attempt_no)", cp.RunID).Count(&unfinished).Error; err != nil {
		return err
	}
	if unfinished != 0 {
		return sessionstate.ErrInvalidState
	}
	return nil
}

func checkpointNativeMessage(tx *gorm.DB, thread string, revision int64, id string) (llm.Message, error) {
	var row struct{ NativeJSON string }
	if err := tx.Raw(`SELECT m.native_json FROM thread_context_versions v JOIN json_each(v.message_ids_json) ids JOIN conversation_messages m ON m.id=ids.value AND m.thread_id=v.thread_id WHERE v.thread_id=? AND v.revision=? AND m.id=?`, thread, revision, id).Scan(&row).Error; err != nil {
		return llm.Message{}, err
	}
	if row.NativeJSON == "" {
		return llm.Message{}, sessionstate.ErrInvalidState
	}
	message, err := sessionstate.DecodeMessage([]byte(row.NativeJSON))
	message.PersistentID = id
	return message, err
}

func validateCheckpointToolBatch(tx *gorm.DB, thread string, cp workflow.RuntimeCheckpoint) error {
	pending := cp.PendingToolBatch
	if len(pending.CallIDs) == 0 || pending.AssistantMessageID == "" {
		return sessionstate.ErrInvalidState
	}
	message, err := checkpointNativeMessage(tx, thread, cp.ContextRevision, pending.AssistantMessageID)
	if err != nil {
		return err
	}
	if message.Role != llm.RoleAssistant || len(message.ToolCalls) != len(pending.CallIDs) {
		return sessionstate.ErrInvalidState
	}
	for i, call := range message.ToolCalls {
		if call.ID != pending.CallIDs[i] {
			return sessionstate.ErrInvalidState
		}
		var row toolExecutionPO
		if err := tx.Where("run_id=? AND thread_id=? AND call_id=?", cp.RunID, thread, call.ID).Order("attempt_no DESC").Take(&row).Error; err != nil {
			return err
		}
		hash, _, err := sessionstate.ToolArgumentsHash(call.Name, call.Args)
		if err != nil {
			return err
		}
		if hash != row.ArgumentsHash || call.Name != row.ToolName {
			return sessionstate.ErrInvalidState
		}
		if row.value().Terminal() && (row.ResultJSON == nil || row.ResultMessageID == nil) {
			return sessionstate.ErrInvalidState
		}
		if row.ResultMessageID != nil {
			var result struct{ NativeJSON string }
			if err := tx.Table("conversation_messages").Select("native_json").Where("id=? AND thread_id=? AND run_id=?", *row.ResultMessageID, thread, cp.RunID).Take(&result).Error; err != nil {
				return err
			}
			native, err := sessionstate.DecodeMessage([]byte(result.NativeJSON))
			if err != nil {
				return err
			}
			if native.Role != llm.RoleTool || native.ToolCallID != call.ID {
				return sessionstate.ErrInvalidState
			}
		}
	}
	return nil
}

func hydrateCheckpointTools(tx *gorm.DB, cp *workflow.RuntimeCheckpoint) error {
	if cp.PendingToolBatch == nil {
		return nil
	}
	var run struct{ ThreadID string }
	if err := tx.Table("runs").Select("thread_id").Where("id=?", cp.RunID).Take(&run).Error; err != nil {
		return err
	}
	if err := validateCheckpointToolBatch(tx, run.ThreadID, *cp); err != nil {
		return err
	}
	message, err := checkpointNativeMessage(tx, run.ThreadID, cp.ContextRevision, cp.PendingToolBatch.AssistantMessageID)
	if err != nil {
		return err
	}
	cp.PendingToolBatch.Calls, cp.PendingToolBatch.AssistantText = message.ToolCalls, message.Text()
	for _, id := range cp.PendingToolBatch.CallIDs {
		var row toolExecutionPO
		if err := tx.Where("run_id=? AND call_id=?", cp.RunID, id).Order("attempt_no DESC").Take(&row).Error; err != nil {
			return err
		}
		cp.ToolExecutions = append(cp.ToolExecutions, row.value())
	}
	return nil
}
