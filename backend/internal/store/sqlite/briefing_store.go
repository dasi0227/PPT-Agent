package sqlite

import (
	"context"
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func (s *Store) AppendBriefingVersion(ctx context.Context, version model.BriefingVersion) error {
	raw, err := json.Marshal(version)
	if err != nil {
		return err
	}
	_, err = s.AppendThreadEvent(ctx, version.ThreadID, threadjournal.Event{Type: "briefing.result", Payload: raw})
	return err
}

func (s *Store) ListThreadBriefings(ctx context.Context, threadID string) ([]model.Briefing, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return nil, err
	}
	rows := []model.BriefingVersion{}
	for _, e := range events {
		if e.Type == "briefing.result" {
			var v model.BriefingVersion
			if err := json.Unmarshal(e.Payload, &v); err != nil {
				return nil, err
			}
			rows = append(rows, v)
		}
	}
	briefings := make([]model.Briefing, 0)
	indexByID := make(map[string]int)
	for _, row := range rows {
		version := row
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
	var threads []journalThreadPO
	if err := s.db.WithContext(ctx).Find(&threads).Error; err != nil {
		return nil, err
	}
	for _, thread := range threads {
		briefings, err := s.ListThreadBriefings(ctx, thread.ID)
		if err != nil {
			return nil, err
		}
		for _, b := range briefings {
			if b.BriefingID == briefingID {
				versions := b.Versions
				if limit > 0 && len(versions) > limit {
					versions = versions[len(versions)-limit:]
				}
				return versions, nil
			}
		}
	}
	return []model.BriefingVersion{}, nil
}
