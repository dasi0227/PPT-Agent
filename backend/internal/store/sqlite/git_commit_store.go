package sqlite

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func (s *Store) HasActiveGitCommit(ctx context.Context, projectID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&commandExecutionPO{}).Where("project_id = ? AND kind = ? AND status IN ?", projectID, "commit", []string{"accepted", "running", "cancel_requested"}).Count(&n).Error
	return n > 0, err
}
func (s *Store) ListActiveCommitCommands(ctx context.Context) ([]model.CommandExecution, error) {
	var rows []commandExecutionPO
	if err := s.db.WithContext(ctx).Where("kind = ? AND status IN ?", "commit", []string{"accepted", "running", "cancel_requested"}).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.CommandExecution, 0, len(rows))
	for _, row := range rows {
		command, err := s.commandProjection(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, command)
	}
	return out, nil
}
