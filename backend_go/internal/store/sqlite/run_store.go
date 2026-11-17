package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// 编译期断言：sqlite.Store 满足 run.Store（会话日志事件持久化 + run 状态机所需）。
var _ run.Store = (*Store)(nil)
var _ run.InputReplyStore = (*Store)(nil)

func (s *Store) CreateProject(ctx context.Context, m model.Project) error {
	if m.Decorations == (decoration.Positions{}) {
		m.Decorations = decoration.DefaultPositions()
	}
	if err := m.Decorations.Validate(); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := rejectDeleted(tx, "project", m.ID); err != nil {
			return err
		}
		row := projectToPO(m)
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Table("project_history_state").Create(map[string]any{"project_id": m.ID}).Error
	})
}

func (s *Store) UpdateProjectDecorations(ctx context.Context, id string, positions decoration.Positions, updatedAt int64) error {
	if err := positions.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(positions)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&projectPO{}).Where("id = ?", id).
		Updates(map[string]any{"decorations_json": string(raw), "updated_at": updatedAt}).Error
}

func (s *Store) UpdateProjectLogo(ctx context.Context, id string, logo *decoration.LogoMetadata, updatedAt int64) error {
	var value any
	if logo != nil {
		raw, err := json.Marshal(logo)
		if err != nil {
			return err
		}
		value = string(raw)
	}
	return s.db.WithContext(ctx).Model(&projectPO{}).Where("id = ?", id).
		Updates(map[string]any{"logo_json": value, "updated_at": updatedAt}).Error
}

func (s *Store) GetProject(ctx context.Context, id string) (model.Project, error) {
	var po projectPO
	if err := s.db.WithContext(ctx).Where("id NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'project')").First(&po, "id = ?", id).Error; err != nil {
		return model.Project{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) ListProjects(ctx context.Context) ([]model.Project, error) {
	var pos []projectPO
	if err := s.db.WithContext(ctx).Where("id NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'project')").Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Project, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	return s.deleteTarget(ctx, "project", id)
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
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := rejectDeleted(tx, "project", m.ProjectID); err != nil {
			return err
		}
		if err := rejectDeleted(tx, "thread", m.ID); err != nil {
			return err
		}
		po := threadToPO(m)
		if err := tx.Create(&po).Error; err != nil {
			return err
		}
		return tx.Table("thread_context_versions").Create(map[string]any{
			"thread_id": m.ID, "revision": 0, "message_ids_json": "[]", "created_seq": 0,
			"created_at": time.Now().UnixMilli(), "reason": "append",
		}).Error
	})
}

func (s *Store) GetThread(ctx context.Context, id string) (model.Thread, error) {
	var po threadPO
	if err := s.db.WithContext(ctx).Where("id NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'thread') AND project_id NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'project')").First(&po, "id = ?", id).Error; err != nil {
		return model.Thread{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) ListThreads(ctx context.Context, projectID string) ([]model.Thread, error) {
	var pos []threadPO
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Where("id NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'thread') AND project_id NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'project')").
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
	return s.deleteTarget(ctx, "thread", id)
}

// CreateRun commits the original input, task identity and public view together.
func (s *Store) CreateRun(ctx context.Context, r model.Run) error {
	if r.SceneRevision < 1 {
		return sessionstate.ErrSceneChanged
	}
	handoff, transferring := run.SteeringHandoffFromContext(ctx)
	hash := r.RequestHash
	if hash == "" {
		var err error
		hash, err = commandHash(r.Command)
		if err != nil {
			return err
		}
	}
	_, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: r.ProjectID, ThreadID: r.ThreadID, SceneRevision: r.SceneRevision}, sessionstate.CommitIdentity{Key: "create-run:" + r.ClientRequestID, RequestHash: hash}, func(w *sessionWrite) error {
		r.InputID = model.MustShortID("input")
		if transferring {
			if len(handoff.Messages) == 0 {
				return sessionstate.ErrInvalidState
			}
			source, err := findRunInput(w.tx, r.ThreadID, handoff.SourceRunID, handoff.Messages[0].ClientMessageID)
			if err != nil {
				return err
			}
			r.InputID = source.ID
		}
		r.ContextRevision, r.AcceptedSeq, r.UpdatedSeq = w.head.ContextRevision, w.seq, w.seq
		r.CreatedAt, r.UpdatedAt = w.now, w.now
		po := runToPO(r)
		if err := w.tx.Create(&po).Error; err != nil {
			return mapProjectWriteErr(err)
		}
		if err := w.publishRun(r.ID); err != nil {
			return err
		}
		if transferring {
			if err := w.transferSteering(r, handoff); err != nil {
				return err
			}
		} else {
			if _, err := w.acceptInput(r.InputID, "initial:"+r.ClientRequestID, r.ID, hash, r.Command); err != nil {
				return err
			}
		}
		if err := w.tx.Table("threads").Where("id = ?", r.ThreadID).Updates(map[string]any{"suggest_version": gorm.Expr("suggest_version + 1"), "next_input_suggestions_json": nil}).Error; err != nil {
			return err
		}
		if err := w.revokeProjectContinuations(r.ProjectID, r.ID); err != nil {
			return err
		}
		receipt, err := json.Marshal(map[string]string{"run_id": r.ID})
		if err != nil {
			return err
		}
		w.result = receipt
		if err := w.tx.Create(&idempotencyPO{Scope: "create_run:" + fmt.Sprint(r.SceneRevision), OwnerID: r.ThreadID, Key: r.ClientRequestID, RequestHash: hash, Status: "completed", ResultJSON: string(receipt)}).Error; err != nil {
			return err
		}
		return w.publishThread()
	})
	return err
}

func (w *sessionWrite) revokeProjectContinuations(projectID, currentRunID string) error {
	var previous []struct{ ID, ThreadID string }
	if err := w.tx.Table("runs").Select("id,thread_id").Where("project_id = ? AND id <> ? AND continuation_allowed = 1", projectID, currentRunID).Order("thread_id,id").Find(&previous).Error; err != nil {
		return err
	}
	for _, row := range previous {
		other, err := w.forThread(row.ThreadID)
		if err != nil {
			return err
		}
		if err := w.tx.Table("runs").Where("id = ?", row.ID).Updates(map[string]any{"continuation_allowed": 0, "checkpoint_json": gorm.Expr("json_set(checkpoint_json, '$.continuation_allowed', json('false'))")}).Error; err != nil {
			return err
		}
		if err := other.publishRun(row.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *sessionWrite) transferSteering(next model.Run, handoff run.SteeringHandoff) error {
	var previous runPO
	if err := w.tx.Where("id = ? AND thread_id = ?", handoff.SourceRunID, next.ThreadID).Take(&previous).Error; err != nil {
		return mapErr(err)
	}
	if previous.Status != string(model.RunDone) {
		return run.ErrRunRevisionConflict
	}
	var rows []steeringPO
	if err := w.tx.Where("run_id = ?", previous.ID).Where(pendingSteering).Order("accepted_seq ASC").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(handoff.Messages) {
		return run.ErrRunRevisionConflict
	}
	if _, err := w.openConversationRound(next.ID); err != nil {
		return err
	}
	for index, row := range rows {
		if row.ClientMessageID != handoff.Messages[index].ClientMessageID {
			return run.ErrRunRevisionConflict
		}
		var input inputPO
		if err := w.tx.Where("id = ?", row.InputID).Take(&input).Error; err != nil {
			return err
		}
		input.SourceRunID, input.RunID, input.DeliveryState, input.UpdatedAt = stringPointer(previous.ID), stringPointer(next.ID), "transferred", w.now
		var command model.RunCommand
		if err := json.Unmarshal([]byte(input.ReferenceJSON), &command); err != nil {
			return err
		}
		command.Scope = next.Command.Scope
		raw, err := json.Marshal(command)
		if err != nil {
			return err
		}
		input.ReferenceJSON = string(raw)
		// Input acceptance and its strict historical cutoff never change on handoff.
		if err := w.tx.Save(&input).Error; err != nil {
			return err
		}
		status := string(model.SteeringAccepted)
		if err := w.tx.Model(&steeringPO{}).Where("thread_id = ? AND client_message_id = ?", row.ThreadID, row.ClientMessageID).Updates(map[string]any{"run_id": next.ID, "status": status, "rejection_code": ""}).Error; err != nil {
			return err
		}
		if err := w.moveInput(input, command, "run:"+next.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CompletedRunsWithPendingSteering(ctx context.Context) ([]model.Run, error) {
	var rows []runPO
	if err := s.reader.WithContext(ctx).Where(settledHistoryProjects).Where("status = ? AND EXISTS (SELECT 1 FROM steering_inbox WHERE steering_inbox.run_id = runs.id AND "+pendingSteering+")", string(model.RunDone)).
		Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	completed := make([]model.Run, len(rows))
	for index, row := range rows {
		completed[index] = row.toModel()
	}
	return completed, nil
}

func (s *Store) GetRun(ctx context.Context, id string) (model.Run, error) {
	var value model.Run
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runPO
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, row.ThreadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		value = row.toModel()
		allowed, err := canContinueRun(tx, row)
		if err != nil {
			return err
		}
		value.CanContinue = allowed
		var routing struct{ ModelRouteJSON *string }
		if err := tx.Table("runs").Select("json_extract(checkpoint_json,'$.model_route') AS model_route_json").Where("id=?", id).Scan(&routing).Error; err != nil {
			return err
		}
		if routeJSON := routing.ModelRouteJSON; routeJSON != nil && *routeJSON != "null" {
			var route struct {
				Active       string `json:"active"`
				Provider     string `json:"provider"`
				Model        string `json:"model"`
				FallbackUsed bool   `json:"fallback_used"`
			}
			if err := json.Unmarshal([]byte(*routeJSON), &route); err != nil {
				return err
			}
			value.ExecutionModel = &model.ActiveModelSelection{Profile: route.Active, Provider: route.Provider, Model: route.Model, FallbackUsed: route.FallbackUsed}
		}
		return nil
	})
	return value, err
}

func (s *Store) GetIdempotency(ctx context.Context, scope, ownerID, key string) (model.IdempotencyRecord, error) {
	var po idempotencyPO
	if err := s.db.WithContext(ctx).First(&po, "scope = ? AND owner_id = ? AND key = ?", scope, ownerID, key).Error; err != nil {
		return model.IdempotencyRecord{}, mapErr(err)
	}
	return po.toModel(), nil
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

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return run.ErrRunNotFound
	}
	return err
}
