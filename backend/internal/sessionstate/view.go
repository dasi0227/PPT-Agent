package sessionstate

import "encoding/json"

type View struct {
	ProjectID           string            `json:"project_id"`
	ThreadID            string            `json:"thread_id"`
	SceneRevision       int64             `json:"scene_revision,string"`
	Seq                 int64             `json:"seq,string"`
	Cursor              string            `json:"cursor"`
	Thread              ThreadView        `json:"thread"`
	ActiveRun           *RunView          `json:"active_run"`
	ActiveCommands      []json.RawMessage `json:"active_commands"`
	PendingInteractions []json.RawMessage `json:"pending_interactions"`
	Timeline            TimelinePage      `json:"timeline"`
	ContextWindow       json.RawMessage   `json:"context_window"`
}
