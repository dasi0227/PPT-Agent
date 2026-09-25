package sqlite

import (
	"context"
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func (s *Store) CreateContextCompaction(ctx context.Context, c model.ContextCompaction) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = s.AppendThreadEvent(ctx, c.ThreadID, threadjournal.Event{Type: "context.compaction_result", RunID: c.RunID, Payload: raw})
	return err
}
func (s *Store) LatestThreadContextWindow(ctx context.Context, threadID string) (string, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return "", err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == string(model.EventContextWindowUpdated) {
			return string(events[i].Payload), nil
		}
	}
	return "", nil
}
func (s *Store) ListThreadContextCompactions(ctx context.Context, threadID string) ([]model.ContextCompaction, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return nil, err
	}
	out := []model.ContextCompaction{}
	for _, e := range events {
		if e.Type == "context.compaction_result" {
			var c model.ContextCompaction
			if err := json.Unmarshal(e.Payload, &c); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
	}
	return out, nil
}
