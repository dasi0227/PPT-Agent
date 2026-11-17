package sessionstate

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// RunUpdate is a typed public fact submitted with a business transaction. It is
// not a log entry and has no transport sequence; subscribers receive upserts.
type RunUpdate struct {
	ID      string          `json:"id"`
	Type    model.EventType `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func NewRunUpdate(kind model.EventType, payload any) (RunUpdate, error) {
	if err := model.ValidatePublicEvent(kind, payload); err != nil {
		return RunUpdate{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return RunUpdate{}, err
	}
	return RunUpdate{ID: model.MustShortID("update"), Type: kind, Payload: raw}, nil
}
