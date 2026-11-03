package sessionstate

import (
	"encoding/base64"
	"encoding/json"
)

type PageCursor struct {
	ProjectID     string `json:"project_id"`
	ThreadID      string `json:"thread_id"`
	SceneRevision int64  `json:"scene_revision,string"`
	GroupID       string `json:"group_id,omitempty"`
	Position      int64  `json:"position,string"`
	ID            string `json:"id"`
}

func (c PageCursor) Encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodePageCursor(value string) (PageCursor, error) {
	var c PageCursor
	if len(value) == 0 || len(value) > 2048 {
		return c, ErrCursorReset
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(raw, &c) != nil || c.ProjectID == "" || c.ThreadID == "" || c.SceneRevision < 1 || c.Position < 0 || c.ID == "" {
		return PageCursor{}, ErrCursorReset
	}
	return c, nil
}

type TimelinePage struct {
	SceneRevision int64           `json:"scene_revision,string"`
	ReadSeq       int64           `json:"read_seq,string"`
	Groups        []TimelineGroup `json:"groups"`
	Items         []TimelineItem  `json:"items"`
	OlderCursor   *string         `json:"older_cursor"`
	HasMore       bool            `json:"has_more"`
	IsEmpty       bool            `json:"is_empty"`
}

type ItemPage struct {
	SceneRevision int64          `json:"scene_revision,string"`
	ReadSeq       int64          `json:"read_seq,string"`
	GroupID       string         `json:"group_id"`
	Items         []TimelineItem `json:"items"`
	OlderCursor   *string        `json:"older_cursor"`
	HasMore       bool           `json:"has_more"`
}
