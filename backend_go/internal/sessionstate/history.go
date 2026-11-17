package sessionstate

import (
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ProjectCheckpoint struct {
	RunID    string                `json:"run_id"`
	ThreadID string                `json:"thread_id"`
	Time     int64                 `json:"time"`
	Snapshot string                `json:"snapshot"`
	Input    model.CreateRunParams `json:"input"`
}

type HistoryState struct {
	SceneRevision int64               `json:"scene_revision,string"`
	Revision      int64               `json:"revision,string"`
	Checkpoints   []ProjectCheckpoint `json:"checkpoints"`
	Latest        string              `json:"latest,omitempty"`
	LatestTime    int64               `json:"latest_time,omitempty"`
	Scene         json.RawMessage     `json:"scene,omitempty"`
}

// The preimage is durable before active files change. This record stays outside
// business snapshots, as do scene versions and completed switch receipts.
type HistoryOperation struct {
	ID              string       `json:"id"`
	Kind            string       `json:"kind"`
	RequestHash     string       `json:"request_hash"`
	SceneRevision   int64        `json:"scene_revision,string"`
	HistoryRevision int64        `json:"history_revision,string"`
	Before          string       `json:"before"`
	Target          string       `json:"target,omitempty"`
	Next            HistoryState `json:"next"`
}

type HistoryControl struct {
	State     HistoryState
	Restoring bool
	Pending   *HistoryOperation
}
