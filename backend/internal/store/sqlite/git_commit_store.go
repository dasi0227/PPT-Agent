package sqlite

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func (s *Store) CreateGitCommitOperation(ctx context.Context, operation model.GitCommitOperation) error {
	return mapProjectWriteErr(s.db.WithContext(ctx).Create(gitCommitOperationToPO(operation)).Error)
}

func (s *Store) GetGitCommitOperation(ctx context.Context, id string) (model.GitCommitOperation, error) {
	var row gitCommitOperationPO
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return model.GitCommitOperation{}, mapErr(err)
	}
	return row.toModel(), nil
}

func (s *Store) GetGitCommitOperationByRequest(ctx context.Context, threadID, clientRequestID string) (model.GitCommitOperation, error) {
	var row gitCommitOperationPO
	if err := s.db.WithContext(ctx).
		First(&row, "thread_id = ? AND client_request_id = ?", threadID, clientRequestID).Error; err != nil {
		return model.GitCommitOperation{}, mapErr(err)
	}
	return row.toModel(), nil
}

func (s *Store) UpdateGitCommitOperation(ctx context.Context, operation model.GitCommitOperation) error {
	return mapProjectWriteErr(s.db.WithContext(ctx).Model(&gitCommitOperationPO{}).
		Where("id = ?", operation.ID).
		Updates(map[string]any{
			"status":      string(operation.Status),
			"phase":       string(operation.Phase),
			"result_json": operation.ResultJSON,
			"error_json":  operation.ErrorJSON,
			"updated_at":  operation.UpdatedAt,
		}).Error)
}

func (s *Store) HasActiveGitCommit(ctx context.Context, projectID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&gitCommitOperationPO{}).
		Where("project_id = ? AND status IN ?", projectID, []string{
			string(model.GitCommitAccepted), string(model.GitCommitRunning),
		}).
		Count(&count).Error
	return count > 0, err
}

func (s *Store) ListActiveGitCommits(ctx context.Context) ([]model.GitCommitOperation, error) {
	var rows []gitCommitOperationPO
	if err := s.db.WithContext(ctx).
		Where("status IN ?", []string{string(model.GitCommitAccepted), string(model.GitCommitRunning)}).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.GitCommitOperation, len(rows))
	for i := range rows {
		out[i] = rows[i].toModel()
	}
	return out, nil
}

func (s *Store) AppendGitCommitEvent(ctx context.Context, event model.GitCommitEvent) error {
	return s.db.WithContext(ctx).Create(gitCommitEventToPO(event)).Error
}

func (s *Store) GitCommitEventsSince(ctx context.Context, operationID string, afterSeq int64) ([]model.GitCommitEvent, error) {
	var rows []gitCommitEventPO
	if err := s.db.WithContext(ctx).
		Where("operation_id = ? AND seq > ?", operationID, afterSeq).
		Order("seq ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.GitCommitEvent, len(rows))
	for i := range rows {
		out[i] = rows[i].toModel()
	}
	return out, nil
}

func (s *Store) ListThreadGitCommits(ctx context.Context, threadID string) ([]model.GitCommitOperation, error) {
	var rows []gitCommitOperationPO
	if err := s.db.WithContext(ctx).
		Where("thread_id = ? AND status IN ?", threadID, []string{
			string(model.GitCommitCompleted), string(model.GitCommitFailed),
		}).
		Order("updated_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.GitCommitOperation, len(rows))
	for i := range rows {
		out[i] = rows[i].toModel()
	}
	return out, nil
}
