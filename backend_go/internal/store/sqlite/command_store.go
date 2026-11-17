package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

func commandHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func (s *Store) AcceptCommand(ctx context.Context, threadID string, request model.CommandRequest, sceneRevision int64) (sessionstate.CommandMutation, bool, error) {
	var accepted model.CommandExecution
	if sceneRevision < 1 || request.RequestKey == "" || !json.Valid(request.Input) {
		return sessionstate.CommandMutation{Command: accepted}, false, sessionstate.ErrInvalidState
	}
	source := request.Source
	if source == "" {
		source = "user"
	}
	if source != "user" && source != "automatic" {
		return sessionstate.CommandMutation{Command: accepted}, false, sessionstate.ErrInvalidState
	}
	head, err := readSessionHead(s.reader.WithContext(ctx), threadID)
	if err != nil {
		return sessionstate.CommandMutation{Command: accepted}, false, err
	}
	hash, err := commandHash(struct {
		Request       model.CommandRequest
		Source, RunID string
	}{request, source, request.RunID})
	if err != nil {
		return sessionstate.CommandMutation{Command: accepted}, false, err
	}
	created := false
	receipt, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: head.ProjectID, ThreadID: threadID, SceneRevision: sceneRevision}, sessionstate.CommitIdentity{Key: "command:" + request.RequestKey, RequestHash: hash}, func(w *sessionWrite) error {
		var thread threadPO
		if err := w.tx.Where("id = ?", threadID).Take(&thread).Error; err != nil {
			return err
		}
		commandID := request.CommandID
		if commandID == "" {
			commandID = model.MustShortID("cmd")
		}
		attempt := 1
		entityRevision := int64(1)
		var previous commandExecutionPO
		err := w.tx.Where("command_id = ?", commandID).Order("attempt_no DESC").Take(&previous).Error
		if err == nil {
			if previous.EntityRevision != request.ExpectedRevision {
				return sessionstate.ErrVersionConflict
			}
			if previous.ThreadID != threadID || previous.Kind != request.Kind || activeCommandStatus(previous.Status) {
				return store.ErrCommandConflict
			}
			attempt = previous.AttemptNo + 1
			entityRevision = previous.EntityRevision + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else if request.ExpectedRevision != 0 {
			return sessionstate.ErrVersionConflict
		}
		if request.BaseAttemptID != "" {
			var base commandExecutionPO
			if err := w.tx.Where("id = ? AND command_id = ? AND status = 'completed'", request.BaseAttemptID, commandID).Take(&base).Error; err != nil {
				return store.ErrCommandConflict
			}
		}
		accepted = model.CommandExecution{RunID: request.RunID, PreviousTitle: thread.Title, OwnerInstanceID: s.instanceID, ExecutionRevision: 1, SceneRevision: sceneRevision, CommandID: commandID, AttemptID: model.MustShortID("attempt"), AttemptNo: attempt, ThreadID: threadID, ProjectID: thread.ProjectID, Kind: request.Kind, Source: source, Status: "accepted", Phase: -1, Input: request.Input, BaseAttemptID: request.BaseAttemptID, Feedback: request.Feedback, CreatedAt: w.now, UpdatedAt: w.now, EntityRevision: entityRevision, UpdatedSeq: w.seq}
		if accepted.Kind == "suggest" {
			if err := prepareSuggestCommand(w.tx, &accepted, thread); err != nil {
				return err
			}
		}
		raw, err := json.Marshal(accepted)
		if err != nil {
			return err
		}
		row := commandExecutionPO{ID: accepted.AttemptID, CommandID: commandID, AttemptNo: attempt, ThreadID: threadID, ProjectID: thread.ProjectID, Kind: request.Kind, Source: source, Status: "accepted", Phase: "-1", OwnerInstanceID: s.instanceID, ExecutionRevision: 1, SceneRevision: sceneRevision, RequestJSON: string(raw), PreviousTitle: accepted.PreviousTitle, RunID: stringPointer(accepted.RunID), EntityRevision: entityRevision, UpdatedSeq: w.seq, CreatedAt: w.now, UpdatedAt: w.now}
		if err := w.tx.Create(&row).Error; err != nil {
			return mapProjectWriteErr(err)
		}
		if err := w.projectCommand(accepted); err != nil {
			return err
		}
		w.result, err = json.Marshal(map[string]string{"command_id": commandID, "attempt_id": row.ID})
		if err != nil {
			return err
		}
		created = true
		return w.publishThread()
	})
	if err != nil {
		return sessionstate.CommandMutation{}, false, err
	}
	if !created {
		var identity struct {
			CommandID string `json:"command_id"`
			AttemptID string `json:"attempt_id"`
		}
		if err := json.Unmarshal(receipt.Result, &identity); err != nil {
			return sessionstate.CommandMutation{Command: accepted}, false, err
		}
		var row commandExecutionPO
		if err := s.reader.WithContext(ctx).Where("id = ? AND command_id = ?", identity.AttemptID, identity.CommandID).Take(&row).Error; err != nil {
			return sessionstate.CommandMutation{Command: accepted}, false, mapErr(err)
		}
		accepted, err = row.toModel()
	}
	return sessionstate.CommandMutation{Command: accepted, Receipt: receipt}, created, err
}

func (s *Store) GetCommand(ctx context.Context, id string) (model.CommandExecution, error) {
	var out model.CommandExecution
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row commandExecutionPO
		if err := tx.Where("command_id = ?", id).Order("attempt_no DESC").Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if _, err := readSessionHead(tx, row.ThreadID); err != nil {
			return err
		}
		var err error
		out, err = row.toModel()
		if err != nil {
			return err
		}
		var success commandExecutionPO
		err = tx.Where("command_id = ? AND status = 'completed'", id).Order("attempt_no DESC").Take(&success).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		last, err := success.toModel()
		if err != nil {
			return err
		}
		out.LatestSuccess = &last
		return nil
	})
	return out, err
}

func (s *Store) SaveCommandExecution(ctx context.Context, execution model.CommandExecution) error {
	// Successful candidates must use CompleteCommand so their business rows
	// cannot be omitted from the terminal transaction.
	if execution.Status == "completed" && (execution.Kind == "compact" || execution.Kind == "handoff" || execution.Kind == "rename") {
		return sessionstate.ErrInvalidState
	}
	hash, err := commandHash(struct {
		Execution         model.CommandExecution
		OwnerInstanceID   string
		ExecutionRevision int64
	}{execution, execution.OwnerInstanceID, execution.ExecutionRevision})
	if err != nil {
		return err
	}
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: execution.ProjectID, ThreadID: execution.ThreadID, SceneRevision: execution.SceneRevision}, sessionstate.CommitIdentity{Key: "command-write:" + execution.AttemptID + ":" + hash, RequestHash: hash}, func(w *sessionWrite) error {
		var row commandExecutionPO
		if err := w.tx.Where("id = ? AND command_id = ?", execution.AttemptID, execution.CommandID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if !activeCommandStatus(row.Status) || row.OwnerInstanceID != execution.OwnerInstanceID || row.ExecutionRevision != execution.ExecutionRevision || row.SceneRevision != execution.SceneRevision {
			return store.ErrCommandConflict
		}
		if row.Status == "cancel_requested" && execution.Status == "running" {
			return context.Canceled
		}
		if execution.Kind == "suggest" && execution.Status == "completed" {
			err := checkSuggestCommand(w.tx, execution)
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			if row.Status == "cancel_requested" || errors.Is(err, context.Canceled) {
				execution.Status, execution.Result = "canceled", nil
			}
		}
		return w.writeCommand(row, execution)
	})
	if err != nil {
		return err
	}
	if !activeCommandStatus(execution.Status) {
		current, err := s.commandAtExecution(ctx, execution)
		if err != nil {
			return err
		}
		if activeCommandStatus(current.Status) {
			return store.ErrCommandConflict
		}
	}
	return nil
}

// Terminal receipts must not acknowledge a different recovery generation.
func (s *Store) commandAtExecution(ctx context.Context, execution model.CommandExecution) (model.CommandExecution, error) {
	var current model.CommandExecution
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, execution.ThreadID)
		if err != nil {
			return err
		}
		if head.ProjectID != execution.ProjectID || head.SceneRevision != execution.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		var row commandExecutionPO
		if err := tx.Where("id = ? AND command_id = ?", execution.AttemptID, execution.CommandID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.OwnerInstanceID != execution.OwnerInstanceID || row.ExecutionRevision != execution.ExecutionRevision || row.SceneRevision != execution.SceneRevision {
			return store.ErrCommandConflict
		}
		current, err = row.toModel()
		return err
	})
	return current, err
}

func (w *sessionWrite) writeCommand(row commandExecutionPO, execution model.CommandExecution) error {
	execution.EntityRevision = row.EntityRevision + 1
	execution.UpdatedSeq, execution.UpdatedAt = w.seq, w.now
	updates := map[string]any{"status": execution.Status, "phase": strconv.Itoa(execution.Phase), "result_json": jsonColumn(execution.Result), "error_json": jsonColumn(execution.Error), "owner_instance_id": execution.OwnerInstanceID, "execution_revision": execution.ExecutionRevision, "entity_revision": execution.EntityRevision, "updated_seq": w.seq, "updated_at": w.now}
	result := w.tx.Model(&commandExecutionPO{}).Where("id = ? AND entity_revision = ?", row.ID, row.EntityRevision).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return sessionstate.ErrVersionConflict
	}
	if execution.Kind == "suggest" && execution.Status == "completed" {
		if err := w.tx.Table("threads").Where("id = ?", w.head.ID).Update("next_input_suggestions_json", string(execution.Result)).Error; err != nil {
			return err
		}
		if err := w.publishThread(); err != nil {
			return err
		}
	}
	return w.projectCommand(execution)
}

func jsonColumn(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func (w *sessionWrite) projectCommand(execution model.CommandExecution) error {
	// Metadata-only commands synchronize their execution state without occupying
	// visible history, page budgets, or the conversation's nonempty state.
	if execution.Kind == "rename" || execution.Kind == "suggest" {
		return w.upsert("command", execution.CommandID, execution)
	}
	groupID, itemID := "command:"+execution.CommandID, "command:"+execution.CommandID
	summary, err := json.Marshal(map[string]any{"kind": execution.Kind, "status": execution.Status, "phase": execution.Phase})
	if err != nil {
		return err
	}
	if err := w.putGroup(sessionstate.TimelineGroup{ID: groupID, ThreadID: w.head.ID, Kind: "command", CommandID: execution.CommandID, Status: execution.Status, Summary: summary}); err != nil {
		return err
	}
	raw, err := json.Marshal(execution)
	if err != nil {
		return err
	}
	var contentRef string
	if len(raw) > 32*1024 {
		var row timelineItemPO
		err := w.tx.Select("entity_revision").Where("id = ?", itemID).Take(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		detailID := model.MustShortID("detail")
		if err := w.tx.Table("timeline_details").Create(map[string]any{"id": detailID, "thread_id": w.head.ID, "public_json": string(raw), "created_seq": w.seq}).Error; err != nil {
			return err
		}
		contentRef = sessionstate.ContentRef{ThreadID: w.head.ID, ItemID: itemID, SceneRevision: w.head.SceneRevision, ItemRevision: row.EntityRevision + 1, SourceType: "detail", SourceID: detailID}.Encode()
		execution.Input, execution.Result, execution.Error = nil, nil, nil
		execution.Feedback = ""
		execution.ContentRef = contentRef
		raw, err = json.Marshal(execution)
		if err != nil {
			return err
		}
	}
	if err := w.putItem(sessionstate.TimelineItem{ID: itemID, ThreadID: w.head.ID, GroupID: groupID, Kind: "command", AttemptID: execution.AttemptID, Summary: raw, DetailRef: contentRef}); err != nil {
		return err
	}
	return w.upsert("command", execution.CommandID, execution)
}

func (s *Store) HasBlockingCommand(ctx context.Context, projectID string) (bool, error) {
	var count int64
	err := s.reader.WithContext(ctx).Model(&commandExecutionPO{}).Where("project_id = ? AND status IN ?", projectID, []string{"accepted", "running", "cancel_requested"}).Where("NOT (kind = 'rename' AND source = 'automatic')").Count(&count).Error
	return count > 0, err
}

func (s *Store) RequestCommandCancel(ctx context.Context, id, attempt, key string) (sessionstate.CommandMutation, error) {
	var row commandExecutionPO
	if err := s.reader.WithContext(ctx).Where("command_id = ?", id).Order("attempt_no DESC").Take(&row).Error; err != nil {
		return sessionstate.CommandMutation{}, mapErr(err)
	}
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return sessionstate.CommandMutation{}, sessionstate.ErrSceneChanged
	}
	revision, ok := sessionstate.EntityRevision(ctx)
	if !ok {
		return sessionstate.CommandMutation{}, sessionstate.ErrVersionConflict
	}
	hash, err := commandHash(struct {
		ID, Attempt string
		Revision    int64
	}{id, attempt, revision})
	if err != nil {
		return sessionstate.CommandMutation{}, err
	}
	receipt, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: row.ProjectID, ThreadID: row.ThreadID, SceneRevision: scene}, sessionstate.CommitIdentity{Key: "cancel-command:" + key, RequestHash: hash}, func(w *sessionWrite) error {
		var current commandExecutionPO
		if err := w.tx.Where("command_id = ?", id).Order("attempt_no DESC").Take(&current).Error; err != nil {
			return mapErr(err)
		}
		if current.ID != attempt || current.EntityRevision != revision {
			return sessionstate.ErrVersionConflict
		}
		execution, err := current.toModel()
		if err != nil {
			return err
		}
		if activeCommandStatus(current.Status) && current.Status != "cancel_requested" {
			execution.Status = "cancel_requested"
			if err := w.writeCommand(current, execution); err != nil {
				return err
			}
			// The response version is the committed version, not the worker copy.
			if err := w.tx.Where("id = ?", attempt).Take(&current).Error; err != nil {
				return err
			}
			execution, err = current.toModel()
			if err != nil {
				return err
			}
		}
		w.result, err = json.Marshal(execution)
		return err
	})
	if err != nil {
		return sessionstate.CommandMutation{}, err
	}
	var execution model.CommandExecution
	if err := json.Unmarshal(receipt.Result, &execution); err != nil {
		return sessionstate.CommandMutation{}, err
	}
	return sessionstate.CommandMutation{Command: execution, Receipt: receipt}, nil
}
