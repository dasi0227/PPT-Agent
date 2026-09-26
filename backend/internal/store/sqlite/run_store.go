package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// 编译期断言：sqlite.Store 满足 run.Store（会话日志事件持久化 + run 状态机所需）。
var _ run.Store = (*Store)(nil)

const interruptedIdempotencyResult = `{"code":"RUN_INTERRUPTED"}`

func (s *Store) CreateProject(ctx context.Context, m model.Project) error {
	return s.db.WithContext(ctx).Create(projectToPO(m)).Error
}

func (s *Store) GetProject(ctx context.Context, id string) (model.Project, error) {
	var po projectPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Project{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) ListProjects(ctx context.Context) ([]model.Project, error) {
	var pos []projectPO
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Project, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&projectPO{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return run.ErrRunNotFound
	}
	return nil
}

func (s *Store) UpdateProjectTitle(ctx context.Context, id, title string, updatedAt int64) error {
	err := s.db.WithContext(ctx).Model(&projectPO{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":      title,
		"updated_at": updatedAt,
	}).Error
	return mapErr(err)
}

func (s *Store) UpdateProjectTheme(ctx context.Context, id, theme string, updatedAt int64) error {
	return s.db.WithContext(ctx).Model(&projectPO{}).Where("id = ?", id).
		Updates(map[string]any{"theme": theme, "updated_at": updatedAt}).Error
}

func (s *Store) CreateThread(ctx context.Context, m model.Thread) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po := threadToPO(m)
		if err := tx.Create(&po).Error; err != nil {
			return err
		}
		raw, _ := json.Marshal(map[string]string{"title": m.Title})
		_, err := s.enqueueEvent(tx, m.ID, threadjournal.Event{Type: "thread.created", Payload: raw})
		return err
	})
	if err == nil {
		err = s.FlushThreadEvents(ctx, m.ID)
	}
	return err
}

func (s *Store) GetThread(ctx context.Context, id string) (model.Thread, error) {
	var po threadPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Thread{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) ListThreads(ctx context.Context, projectID string) ([]model.Thread, error) {
	var pos []threadPO
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Thread, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}

func (s *Store) DeleteThread(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&threadPO{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return run.ErrRunNotFound
	}
	return nil
}

// CreateRun commits the input reference and execution control together.
func (s *Store) CreateRun(ctx context.Context, r model.Run) error {
	raw, err := json.Marshal(runAcceptance{Command: r.Command, ClientRequestID: r.ClientRequestID})
	if err != nil {
		return err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.checkAcceptance(tx, r.ThreadID); err != nil {
			return err
		}
		event, err := s.enqueueEvent(tx, r.ThreadID, threadjournal.Event{Type: "run.accepted", RunID: r.ID, Payload: raw})
		if err != nil {
			return err
		}
		po := runToPO(r)
		po.StartEventSeq = event.Seq
		if err := tx.Create(&po).Error; err != nil {
			return mapProjectWriteErr(err)
		}
		receipt, _ := json.Marshal(map[string]string{"run_id": r.ID})
		if r.ClientRequestID == "" {
			return nil
		}
		return tx.Create(&idempotencyPO{Scope: "create_run", OwnerID: r.ThreadID, Key: r.ClientRequestID, RequestHash: r.RequestHash, Status: "completed", ResultJSON: string(receipt)}).Error
	})
	if err != nil {
		return err
	}
	if err := s.FlushThreadEvents(ctx, r.ThreadID); err != nil {
		recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, pauseErr := s.PauseRun(recoveryCtx, r.ID, r.OwnerInstanceID, "start_event_delivery_failed", time.Now().Unix())
		return errors.Join(err, pauseErr)
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, id string) (model.Run, error) {
	var po runPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Run{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) SetRunStatus(ctx context.Context, id string, status model.RunStatus) error {
	threadID := ""
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runStatusRow
		if err := tx.Table("runs").Select("id,thread_id,status,owner_instance_id,execution_revision").Where("id = ?", id).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		threadID = row.ThreadID
		return s.setRunStatusInTransaction(tx, row, status)
	})
	if err == nil && threadID != "" {
		err = s.FlushThreadEvents(ctx, threadID)
	}
	return err
}

type runStatusRow struct {
	ID                string
	ThreadID          string
	Status            string
	OwnerInstanceID   string
	ExecutionRevision int64
}

func (s *Store) setRunStatusInTransaction(tx *gorm.DB, row runStatusRow, status model.RunStatus) error {
	// The terminal event may already have committed this transition and released
	// ownership. The scheduler may acknowledge that same execution, but an old
	// execution must never change the state of a resumed run.
	sameTerminal := status.Terminal() && row.Status == string(status)
	if owner, execution, ok := workflow.CheckpointOwnership(tx.Statement.Context); ok {
		if row.ExecutionRevision != execution || (row.OwnerInstanceID != owner && !(sameTerminal && row.OwnerInstanceID == "")) {
			return run.ErrRunRevisionConflict
		}
	}
	if model.RunStatus(row.Status).Terminal() {
		if sameTerminal {
			return nil
		}
		return run.ErrRunNotRunning
	}
	// Enqueue while the worker still owns the run. Clearing ownership first
	// makes enqueueEvent reject our own event and roll back the transaction.
	raw, _ := json.Marshal(map[string]string{"status": string(status)})
	if _, err := s.enqueueEvent(tx, row.ThreadID, threadjournal.Event{Type: "run.state", RunID: row.ID, Payload: raw}); err != nil {
		return err
	}
	updates := map[string]any{"status": string(status), "updated_at": nowUnix()}
	if status.Terminal() {
		updates["owner_instance_id"] = ""
	}
	if status == model.RunRunning || status == model.RunWaiting {
		updates["pause_reason"] = ""
		updates["paused_at"] = nil
	}
	return tx.Model(&runPO{}).Where("id = ?", row.ID).Updates(updates).Error
}

func (s *Store) PauseNonTerminalRuns(ctx context.Context, reason string, pausedAt int64) ([]model.Run, error) {
	var out []model.Run
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []runPO
		statuses := []string{string(model.RunPending), string(model.RunRunning), string(model.RunWaiting), string(model.RunRecovering)}
		if err := tx.Where("status IN ?", statuses).Order("created_at ASC").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			if err := tx.Model(&runPO{}).Where("status IN ?", statuses).Updates(map[string]any{
				"status": string(model.RunPaused), "owner_instance_id": "", "pause_reason": reason,
				"paused_at": pausedAt, "updated_at": pausedAt,
			}).Error; err != nil {
				return err
			}
		}
		if err := interruptPausedRunIdempotency(tx, pausedAt); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		out = make([]model.Run, 0, len(rows))
		for _, row := range rows {
			runModel := row.toModel()
			runModel.Status = model.RunPaused
			runModel.OwnerInstanceID = ""
			runModel.PauseReason = reason
			runModel.PausedAt = pausedAt
			out = append(out, runModel)
		}
		return nil
	})
	return out, err
}

func (s *Store) PauseRun(ctx context.Context, id, ownerInstanceID, reason string, pausedAt int64) (model.Run, error) {
	var out model.Run
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runPO
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		status := model.RunStatus(row.Status)
		if status.Terminal() {
			out = row.toModel()
			return nil
		}
		if ownerInstanceID != "" && row.OwnerInstanceID != ownerInstanceID {
			return run.ErrRunNotRunning
		}
		if err := tx.Model(&runPO{}).Where("id = ?", id).Updates(map[string]any{
			"status": string(model.RunPaused), "owner_instance_id": "", "pause_reason": reason,
			"paused_at": pausedAt, "updated_at": pausedAt,
		}).Error; err != nil {
			return err
		}
		if err := interruptRunIdempotency(tx, []string{id}, pausedAt); err != nil {
			return err
		}
		out = row.toModel()
		out.Status, out.OwnerInstanceID, out.PauseReason, out.PausedAt = model.RunPaused, "", reason, pausedAt
		return nil
	})
	return out, err
}

func (s *Store) ClaimPausedRun(ctx context.Context, id, ownerInstanceID string) (model.Run, error) {
	now := nowUnix()
	result := s.db.WithContext(ctx).Model(&runPO{}).
		Where("id = ? AND status = ?", id, string(model.RunPaused)).
		Updates(map[string]any{
			"status": string(model.RunRecovering), "owner_instance_id": ownerInstanceID, "execution_revision": gorm.Expr("execution_revision + 1"),
			"pause_reason": "", "paused_at": nil, "updated_at": now,
		})
	if result.Error != nil {
		return model.Run{}, result.Error
	}
	if result.RowsAffected != 1 {
		return model.Run{}, run.ErrRunNotRunning
	}
	return s.GetRun(ctx, id)
}

func (s *Store) ReleaseRecoveringRun(ctx context.Context, id, ownerInstanceID, reason string, pausedAt int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&runPO{}).
			Where("id = ? AND status = ? AND owner_instance_id = ?", id, string(model.RunRecovering), ownerInstanceID).
			Updates(map[string]any{
				"status": string(model.RunPaused), "owner_instance_id": "", "pause_reason": reason,
				"paused_at": pausedAt, "updated_at": pausedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return run.ErrRunNotRunning
		}
		return interruptRunIdempotency(tx, []string{id}, pausedAt)
	})
}

func (s *Store) CancelPausedRun(ctx context.Context, id string, canceledAt int64) (model.Run, error) {
	result := s.db.WithContext(ctx).Model(&runPO{}).
		Where("id = ? AND status = ?", id, string(model.RunPaused)).
		Updates(map[string]any{
			"status": string(model.RunCanceled), "owner_instance_id": "", "pause_reason": "",
			"paused_at": nil, "cancel_requested_at": canceledAt, "updated_at": canceledAt,
		})
	if result.Error != nil {
		return model.Run{}, result.Error
	}
	if result.RowsAffected != 1 {
		return model.Run{}, run.ErrRunNotRunning
	}
	return s.GetRun(ctx, id)
}

func (s *Store) UpdateRunMode(ctx context.Context, id string, mode model.RunMode) error {
	return s.db.WithContext(ctx).Model(&runPO{}).Where("id = ?", id).Updates(map[string]any{"mode": string(mode), "updated_at": nowUnix()}).Error
}

func (s *Store) RequestRunCancel(ctx context.Context, id string, requestedAt int64) (model.Run, error) {
	var out model.Run
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po runPO
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		if model.RunStatus(po.Status).Terminal() || po.CancelRequestedAt != nil {
			out = po.toModel()
			return nil
		}
		if err := tx.Model(&runPO{}).Where("id = ? AND cancel_requested_at IS NULL", id).
			Updates(map[string]any{"cancel_requested_at": requestedAt, "updated_at": requestedAt}).Error; err != nil {
			return err
		}
		po.CancelRequestedAt = &requestedAt
		po.UpdatedAt = requestedAt
		out = po.toModel()
		return nil
	})
	return out, err
}

func (s *Store) AcquireIdempotency(ctx context.Context, record model.IdempotencyRecord) (model.IdempotencyRecord, bool, error) {
	var out model.IdempotencyRecord
	created := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.Status == "" {
			record.Status = "in_progress"
		}
		res := tx.Exec(`INSERT INTO idempotency_records
			(scope,owner_id,key,request_hash,status,result_json)
			VALUES (?,?,?,?,?,?) ON CONFLICT(scope,owner_id,key) DO NOTHING`,
			record.Scope, record.OwnerID, record.Key, record.RequestHash, record.Status,
			record.ResultJSON)
		if res.Error != nil {
			return res.Error
		}
		created = res.RowsAffected == 1
		var po idempotencyPO
		if err := tx.First(&po, "scope = ? AND owner_id = ? AND key = ?", record.Scope, record.OwnerID, record.Key).Error; err != nil {
			return err
		}
		if !created && po.Status == "failed" && po.ResultJSON == interruptedIdempotencyResult && po.RequestHash == record.RequestHash {
			reclaimed := tx.Model(&idempotencyPO{}).
				Where("scope = ? AND owner_id = ? AND key = ? AND status = ? AND result_json = ?",
					record.Scope, record.OwnerID, record.Key, "failed", interruptedIdempotencyResult).
				Updates(map[string]any{"status": "in_progress", "result_json": ""})
			if reclaimed.Error != nil {
				return reclaimed.Error
			}
			created = reclaimed.RowsAffected == 1
			if created {
				po.Status = "in_progress"
				po.ResultJSON = ""
			}
		}
		out = po.toModel()
		return nil
	})
	return out, created, err
}

func interruptPausedRunIdempotency(tx *gorm.DB, at int64) error {
	pausedRunIDs := tx.Model(&runPO{}).Select("id").Where("status = ?", string(model.RunPaused))
	return tx.Model(&idempotencyPO{}).
		Where("scope IN ? AND status = ? AND owner_id IN (?)", []string{"tool_call", "commit"}, "in_progress", pausedRunIDs).
		Updates(map[string]any{"status": "failed", "result_json": interruptedIdempotencyResult}).Error
}

func interruptRunIdempotency(tx *gorm.DB, runIDs []string, at int64) error {
	if len(runIDs) == 0 {
		return nil
	}
	return tx.Model(&idempotencyPO{}).
		Where("scope IN ? AND status = ? AND owner_id IN ?", []string{"tool_call", "commit"}, "in_progress", runIDs).
		Updates(map[string]any{"status": "failed", "result_json": interruptedIdempotencyResult}).Error
}

func (s *Store) CompleteIdempotency(ctx context.Context, scope, ownerID, key, status, resultJSON string) error {
	res := s.db.WithContext(ctx).Model(&idempotencyPO{}).
		Where("scope = ? AND owner_id = ? AND key = ? AND status != ?", scope, ownerID, key, "completed").
		Updates(map[string]any{"status": status, "result_json": resultJSON})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return run.ErrRunNotFound
	}
	return nil
}

func (s *Store) GetIdempotency(ctx context.Context, scope, ownerID, key string) (model.IdempotencyRecord, error) {
	var po idempotencyPO
	if err := s.db.WithContext(ctx).First(&po, "scope = ? AND owner_id = ? AND key = ?", scope, ownerID, key).Error; err != nil {
		return model.IdempotencyRecord{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) CreateSteering(ctx context.Context, message model.SteeringMessage) (model.SteeringMessage, bool, error) {
	var out model.SteeringMessage
	created := false
	err := workflow.WithExternalCheckpointWrite(ctx, func() (workflow.RuntimeCheckpoint, error) {
		var committed workflow.RuntimeCheckpoint
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var existing steeringPO
			err := tx.First(&existing, "thread_id = ? AND client_message_id = ?", message.ThreadID, message.ClientMessageID).Error
			if err == nil {
				out = existing.toModel()
				committed, err = readCheckpoint(tx, message.RunID)
				if errors.Is(err, run.ErrRunNotFound) {
					var row runPO
					if err := tx.First(&row, "id = ?", message.RunID).Error; err != nil {
						return mapErr(err)
					}
					committed = workflow.RuntimeCheckpoint{OwnerInstanceID: row.OwnerInstanceID, ExecutionRevision: row.ExecutionRevision, CheckpointRevision: row.CheckpointRevision, Scope: row.Command.Scope}
					return nil
				}
				return err
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := s.checkAcceptance(tx, message.ThreadID); err != nil {
				return err
			}
			var row runPO
			if err := tx.First(&row, "id = ? AND thread_id = ?", message.RunID, message.ThreadID).Error; err != nil {
				return mapErr(err)
			}
			if (row.Status != string(model.RunPending) && row.Status != string(model.RunRunning)) || row.CancelRequestedAt != nil {
				return run.ErrRunNotRunning
			}
			current := row.Command.Scope
			if message.Scope.Source.Kind != "" {
				if message.Scope.Revision != current.Revision && message.Scope.Revision != current.Revision+1 {
					return run.ErrRunRevisionConflict
				}
				if message.Scope.Revision == current.Revision && !sameRunScope(current, message.Scope) {
					return run.ErrRunRevisionConflict
				}
				cp, err := readCheckpoint(tx, row.ID)
				if errors.Is(err, run.ErrRunNotFound) {
					phase := workflow.PhaseExecuting
					if row.Command.Mode == model.ModePlan {
						phase = workflow.PhasePlanning
					} else if row.Command.Mode == model.ModeChat || row.Command.Mode == model.ModeGrill {
						phase = workflow.PhaseChat
					}
					cp = workflow.RuntimeCheckpoint{RunID: row.ID, LoopID: "loop_steering_" + row.ID, Phase: phase, Mode: row.Command.Mode, Scope: current, DOMSelections: row.Command.DOMSelections}
				} else if err != nil {
					return err
				}
				cp.Scope = message.Scope
				cp.DOMSelections = append(cp.DOMSelections, message.DOMSelections...)
				cp.Boundary = "steering_accepted"
				cp.OwnerInstanceID = row.OwnerInstanceID
				cp.ExecutionRevision = row.ExecutionRevision
				cp.CheckpointRevision = row.CheckpointRevision
				scope, _ := json.Marshal(message.Scope)
				if err := writeCheckpoint(tx, cp, current.Revision, map[string]any{"scope_json": string(scope), "scope_revision": message.Scope.Revision}); err != nil {
					return err
				}
				cp.CheckpointRevision++
				committed = cp
			}
			if message.Scope.Source.Kind == "" {
				committed, err = readCheckpoint(tx, row.ID)
				if err != nil && !errors.Is(err, run.ErrRunNotFound) {
					return err
				}
				if errors.Is(err, run.ErrRunNotFound) {
					committed = workflow.RuntimeCheckpoint{OwnerInstanceID: row.OwnerInstanceID, ExecutionRevision: row.ExecutionRevision, CheckpointRevision: row.CheckpointRevision, Scope: current}
				}
			}
			message.Status = model.SteeringAccepted
			raw, err := json.Marshal(message)
			if err != nil {
				return err
			}
			event, err := s.enqueueEvent(tx, message.ThreadID, threadjournal.Event{Type: "steering.accepted", RunID: message.RunID, Payload: raw})
			if err != nil {
				return err
			}
			po := steeringPO{RunID: message.RunID, ThreadID: message.ThreadID, ClientMessageID: message.ClientMessageID, RequestHash: message.RequestHash, InputEventSeq: event.Seq, Status: string(message.Status)}
			if err := tx.Create(&po).Error; err != nil {
				return err
			}
			out = message
			created = true
			return nil
		})
		return committed, err
	})
	if err == nil {
		err = s.FlushThreadEvents(ctx, message.ThreadID)
	}
	return out, created, err
}

func sameRunScope(left, right model.RunScope) bool {
	return left.Revision == right.Revision &&
		left.IncludeRunCreatedSlides == right.IncludeRunCreatedSlides &&
		left.Source.Kind == right.Source.Kind &&
		slices.Equal(left.Source.SectionIDs, right.Source.SectionIDs) &&
		slices.Equal(left.SlideIDs, right.SlideIDs)
}

func (s *Store) ListPendingSteering(ctx context.Context, runID string) ([]model.SteeringMessage, error) {
	var rows []steeringPO
	if err := s.db.WithContext(ctx).Where("run_id = ? AND status = ?", runID, string(model.SteeringAccepted)).
		Order("input_event_seq ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.SteeringMessage, len(rows))
	for i := range rows {
		out[i] = rows[i].toModel()
	}
	return out, nil
}

func (s *Store) ListThreadSteering(ctx context.Context, threadID string) ([]model.SteeringMessage, error) {
	var rows []steeringPO
	if err := s.db.WithContext(ctx).Where("thread_id = ?", threadID).
		Order("input_event_seq ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.SteeringMessage, len(rows))
	for i := range rows {
		out[i] = rows[i].toModel()
	}
	return out, nil
}

func (s *Store) MarkSteering(ctx context.Context, runID string, ids []string, status model.SteeringStatus, at int64, rejectionCode string) error {
	if len(ids) == 0 {
		return nil
	}
	threadID := ""
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []steeringPO
		if err := tx.Where("run_id = ? AND client_message_id IN ? AND status = ?", runID, ids, string(model.SteeringAccepted)).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			threadID = row.ThreadID
			raw, _ := json.Marshal(map[string]any{"client_message_id": row.ClientMessageID, "input_event_seq": row.InputEventSeq, "rejection_code": rejectionCode})
			event, err := s.enqueueEvent(tx, threadID, threadjournal.Event{Type: "steering." + string(status), RunID: runID, TS: at / 1_000_000, Payload: raw})
			if err != nil {
				return err
			}
			if err := tx.Model(&steeringPO{}).Where("thread_id = ? AND client_message_id = ?", threadID, row.ClientMessageID).Updates(map[string]any{"status": string(status), "result_event_seq": event.Seq}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil && threadID != "" {
		err = s.FlushThreadEvents(ctx, threadID)
	}
	return err
}

// HasActiveRun 报告某 project 是否有非终态 run（包括 paused/recovering），供手动写操作互斥判定。
func (s *Store) HasActiveRun(ctx context.Context, projectID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&runPO{}).
		Where("project_id = ? AND status NOT IN ?", projectID, []string{"done", "failed", "canceled"}).
		Count(&n).Error
	return n > 0, err
}

// GetActiveRunForThread returns the durable non-terminal Run for UI recovery.
func (s *Store) GetActiveRunForThread(ctx context.Context, threadID string) (model.Run, error) {
	var po runPO
	if err := s.db.WithContext(ctx).
		Where("thread_id = ? AND status NOT IN ?", threadID, []string{
			string(model.RunDone), string(model.RunFailed), string(model.RunCanceled),
		}).
		Order("created_at DESC").First(&po).Error; err != nil {
		return model.Run{}, mapErr(err)
	}
	return po.toModel(), nil
}

// AppendEvent assigns the conversation sequence before broadcasting.
func (s *Store) AppendEvent(ctx context.Context, e *model.Event) error {
	var threadID string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runStatusRow
		if err := tx.Table("runs").Select("id,thread_id,status,owner_instance_id,execution_revision").Where("id = ?", e.RunID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if _, execution, ok := workflow.CheckpointOwnership(ctx); ok && execution != row.ExecutionRevision {
			return run.ErrRunRevisionConflict
		}
		threadID = row.ThreadID
		event, err := s.enqueueEvent(tx, threadID, threadjournal.Event{Type: string(e.Type), RunID: e.RunID, Payload: json.RawMessage(e.Payload)})
		if err != nil {
			return err
		}
		// A visible terminal event and the scheduler's project lock must agree,
		// including when the process stops before Engine.finish can run.
		var terminalStatus model.RunStatus
		switch e.Type {
		case model.EventRunCompleted:
			terminalStatus = model.RunDone
		case model.EventRunFailed, model.EventRunError:
			terminalStatus = model.RunFailed
		case model.EventRunCanceled:
			terminalStatus = model.RunCanceled
		}
		if terminalStatus != "" {
			if err := s.setRunStatusInTransaction(tx, row, terminalStatus); err != nil {
				return err
			}
		}
		e.Seq = event.Seq
		e.CreatedAt = event.TS
		return nil
	})
	if err != nil {
		return err
	}
	return s.FlushThreadEvents(ctx, threadID)
}
func (s *Store) EventsSince(ctx context.Context, runID string, afterSeq int64) ([]model.Event, error) {
	var row struct{ ThreadID string }
	if err := s.db.WithContext(ctx).Table("runs").Select("thread_id").Where("id = ?", runID).Take(&row).Error; err != nil {
		return nil, mapErr(err)
	}
	events, err := s.ListThreadEvents(ctx, row.ThreadID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Event, 0)
	for _, e := range events {
		if e.RunID == runID && e.Seq > afterSeq {
			out = append(out, e)
		}
	}
	return out, nil
}
func (s *Store) ListThreadEvents(ctx context.Context, threadID string) ([]model.Event, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return nil, err
	}
	out := make([]model.Event, 0)
	for _, e := range events {
		var payload map[string]any
		if e.RunID == "" || json.Unmarshal(e.Payload, &payload) != nil || model.ValidatePublicEvent(model.EventType(e.Type), payload) != nil {
			continue
		}
		out = append(out, model.Event{RunID: e.RunID, Seq: e.Seq, Type: model.EventType(e.Type), Payload: string(e.Payload), CreatedAt: e.TS})
	}
	return out, nil
}

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return run.ErrRunNotFound
	}
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
