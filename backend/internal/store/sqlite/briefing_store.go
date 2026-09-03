package sqlite

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func (s *Store) AppendBriefingVersion(ctx context.Context, version model.BriefingVersion) error {
	return s.db.WithContext(ctx).Create(briefingVersionToPO(version)).Error
}

func (s *Store) ListThreadBriefings(ctx context.Context, threadID string) ([]model.Briefing, error) {
	var rows []briefingVersionPO
	if err := s.db.WithContext(ctx).
		Where("thread_id = ?", threadID).
		Order("created_at ASC, briefing_id ASC, version_no ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	briefings := make([]model.Briefing, 0)
	indexByID := make(map[string]int)
	for _, row := range rows {
		version := row.toModel()
		index, ok := indexByID[version.BriefingID]
		if !ok {
			index = len(briefings)
			indexByID[version.BriefingID] = index
			briefings = append(briefings, model.Briefing{
				BriefingID: version.BriefingID,
				ThreadID:   version.ThreadID,
				ProjectID:  version.ProjectID,
				Kind:       version.Kind,
				Versions:   []model.BriefingVersion{},
			})
		}
		briefings[index].Versions = append(briefings[index].Versions, version)
		if version.CreatedAt > briefings[index].UpdatedAt {
			briefings[index].UpdatedAt = version.CreatedAt
		}
	}
	return briefings, nil
}

func (s *Store) GetBriefingVersions(ctx context.Context, briefingID string, limit int) ([]model.BriefingVersion, error) {
	query := s.db.WithContext(ctx).
		Where("briefing_id = ?", briefingID).
		Order("version_no DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []briefingVersionPO
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.BriefingVersion, len(rows))
	for i := range rows {
		out[len(rows)-1-i] = rows[i].toModel()
	}
	return out, nil
}
