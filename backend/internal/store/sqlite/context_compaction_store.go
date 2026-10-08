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
