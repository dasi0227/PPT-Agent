package sqlite

import (
	"context"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"gorm.io/gorm"
)

func (s *Store) CreateContextCompaction(ctx context.Context, compaction model.ContextCompaction) error {
	return s.db.WithContext(ctx).Create(contextCompactionToPO(compaction)).Error
}

func (s *Store) LatestThreadContextWindow(ctx context.Context, threadID string) (string, error) {
	var row struct {
		Payload string
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT e.payload
		FROM run_events e
		JOIN runs r ON r.id = e.run_id
		WHERE r.thread_id = ? AND e.type = ?
		ORDER BY e.created_at DESC, e.seq DESC
		LIMIT 1
	`, threadID, string(model.EventContextWindowUpdated)).Scan(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return row.Payload, err
}

func (s *Store) ListThreadContextCompactions(ctx context.Context, threadID string) ([]model.ContextCompaction, error) {
	var rows []contextCompactionPO
	if err := s.db.WithContext(ctx).
		Where("thread_id = ?", threadID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.ContextCompaction, len(rows))
	for index, row := range rows {
		out[index] = row.toModel()
	}
	return out, nil
}
