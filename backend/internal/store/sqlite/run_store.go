package sqlite

import (
	"context"
	"errors"

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

func (s *Store) UpdateProjectRevisions(ctx context.Context, id string, deckRevision, designRevision int) error {
	return s.db.WithContext(ctx).Model(&projectPO{}).Where("id = ?", id).
		Updates(map[string]any{"deck_revision": deckRevision, "design_revision": designRevision, "deck_path": "deck.json"}).Error
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
		 status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		po.ID, po.ThreadID, po.ProjectID, po.TargetArtifact, po.TargetLevel,
		nullIfEmpty(po.TargetSlideID), po.InteractionIntent, po.WorkSpecJSON,
		po.Status, po.CreatedAt, po.UpdatedAt,
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
