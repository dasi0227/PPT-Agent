package sqlite

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// 编译期断言：sqlite.Store 满足 run.Store（run_events 持久化 + run 状态机所需）。
var _ run.Store = (*Store)(nil)

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

func (s *Store) UpdateProjectRevisions(ctx context.Context, id string, outlineRevision, designRevision int) error {
	return s.db.WithContext(ctx).Model(&projectPO{}).Where("id = ?", id).
		Updates(map[string]any{
			"outline_revision": outlineRevision, "design_revision": designRevision,
			"layout_version": 2,
		}).Error
}

func (s *Store) UpdateThreadTitle(ctx context.Context, id, title string, updatedAt int64) error {
	err := s.db.WithContext(ctx).Model(&threadPO{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":      title,
		"updated_at": updatedAt,
	}).Error
	return mapErr(err)
}

func (s *Store) CreateThread(ctx context.Context, m model.Thread) error {
	return s.db.WithContext(ctx).Create(threadToPO(m)).Error
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

// CreateRun writes the normalized WorkSpec without an action/scope projection.
func (s *Store) CreateRun(ctx context.Context, r model.Run) error {
	po := runToPO(r)
	return s.db.WithContext(ctx).Exec(
		`INSERT INTO runs (id, thread_id, project_id,
		 target_artifact, target_level, target_slide_id, interaction_intent, work_spec_json,
		 client_request_id, model_profile_name, model_provider, model_name, model_url,
		 cancel_requested_at, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		po.ID, po.ThreadID, po.ProjectID, po.TargetArtifact, po.TargetLevel,
		nullIfEmpty(po.TargetSlideID), po.InteractionIntent, po.WorkSpecJSON,
		nullIfEmpty(po.ClientRequestID), nullIfEmpty(po.ModelProfileName),
		nullIfEmpty(po.ModelProvider), nullIfEmpty(po.ModelName), nullIfEmpty(po.ModelURL),
		po.CancelRequestedAt, po.Status, po.CreatedAt, po.UpdatedAt,
	).Error
}

func (s *Store) GetRun(ctx context.Context, id string) (model.Run, error) {
	var po runPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Run{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) SetRunStatus(ctx context.Context, id string, status model.RunStatus) error {
	return s.db.WithContext(ctx).Model(&runPO{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": string(status), "updated_at": nowUnix()}).Error
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
		now := time.Now().UnixNano()
		if record.CreatedAt == 0 {
			record.CreatedAt = now
		}
		record.UpdatedAt = now
		if record.Status == "" {
			record.Status = "in_progress"
		}
		res := tx.Exec(`INSERT INTO idempotency_records
			(scope,owner_id,key,request_hash,status,result_json,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(scope,owner_id,key) DO NOTHING`,
			record.Scope, record.OwnerID, record.Key, record.RequestHash, record.Status,
			record.ResultJSON, record.CreatedAt, record.UpdatedAt)
		if res.Error != nil {
			return res.Error
		}
		created = res.RowsAffected == 1
		var po idempotencyPO
		if err := tx.First(&po, "scope = ? AND owner_id = ? AND key = ?", record.Scope, record.OwnerID, record.Key).Error; err != nil {
			return err
		}
		out = po.toModel()
		return nil
	})
	return out, created, err
}

func (s *Store) CompleteIdempotency(ctx context.Context, scope, ownerID, key, status, resultJSON string) error {
	res := s.db.WithContext(ctx).Model(&idempotencyPO{}).
		Where("scope = ? AND owner_id = ? AND key = ?", scope, ownerID, key).
		Updates(map[string]any{"status": status, "result_json": resultJSON, "updated_at": time.Now().UnixNano()})
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
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`INSERT INTO steering_inbox
			(run_id,thread_id,client_message_id,request_hash,content,status,accepted_at,injected_at,rejection_code)
			VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(thread_id,client_message_id) DO NOTHING`,
			message.RunID, message.ThreadID, message.ClientMessageID, message.RequestHash,
			message.Content, message.Status, message.AcceptedAt, nil, message.RejectionCode)
		if res.Error != nil {
			return res.Error
		}
		created = res.RowsAffected == 1
		var po steeringPO
		if err := tx.First(&po, "thread_id = ? AND client_message_id = ?", message.ThreadID, message.ClientMessageID).Error; err != nil {
			return err
		}
		out = po.toModel()
		return nil
	})
	return out, created, err
}

func (s *Store) ListPendingSteering(ctx context.Context, runID string) ([]model.SteeringMessage, error) {
	var rows []steeringPO
	if err := s.db.WithContext(ctx).Where("run_id = ? AND status = ?", runID, string(model.SteeringAccepted)).
		Order("accepted_at ASC, client_message_id ASC").Find(&rows).Error; err != nil {
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
		Order("accepted_at ASC, client_message_id ASC").Find(&rows).Error; err != nil {
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
	updates := map[string]any{"status": string(status), "rejection_code": rejectionCode}
	if status == model.SteeringInjected {
		updates["injected_at"] = at
	}
	return s.db.WithContext(ctx).Model(&steeringPO{}).
		Where("run_id = ? AND client_message_id IN ? AND status = ?", runID, ids, string(model.SteeringAccepted)).
		Updates(updates).Error
}

// HasActiveRun 报告某 project 是否有非终态 run（pending/running/waiting），供手动写操作互斥判定。
func (s *Store) HasActiveRun(ctx context.Context, projectID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&runPO{}).
		Where("project_id = ? AND status NOT IN ?", projectID, []string{"done", "failed", "canceled"}).
		Count(&n).Error
	return n > 0, err
}

// AppendEvent 持久化一条 run 事件（run_id+seq 主键，seq 单调连续 API-SSE-001）。
func (s *Store) AppendEvent(ctx context.Context, e model.Event) error {
	return s.db.WithContext(ctx).Create(eventToPO(e)).Error
}

// EventsSince 返回 seq > afterSeq 的事件（升序），用于 Last-Event-ID 续传（API-SSE-003）。
func (s *Store) EventsSince(ctx context.Context, runID string, afterSeq int64) ([]model.Event, error) {
	var pos []runEventPO
	if err := s.db.WithContext(ctx).
		Where("run_id = ? AND seq > ?", runID, afterSeq).
		Order("seq ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]model.Event, len(pos))
	for i, po := range pos {
		out[i] = po.toModel()
	}
	return out, nil
}

func (s *Store) SaveRunContext(ctx context.Context, m model.RunContext) error {
	return s.db.WithContext(ctx).Create(contextToPO(m)).Error
}

func (s *Store) GetRunContext(ctx context.Context, runID string) (model.RunContext, error) {
	var po runContextPO
	if err := s.db.WithContext(ctx).First(&po, "run_id = ?", runID).Error; err != nil {
		return model.RunContext{}, mapErr(err)
	}
	return po.toModel(), nil
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
