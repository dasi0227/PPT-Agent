// Package sessionstate defines durable session state and its public synchronization
// protocol. Changes are notifications of committed state, never a recovery log.
package sessionstate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var (
	ErrSceneChanged        = errors.New("SCENE_CHANGED")
	ErrCursorReset         = errors.New("CURSOR_RESET_REQUIRED")
	ErrVersionConflict     = errors.New("VERSION_CONFLICT")
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrProjectRestoring    = errors.New("PROJECT_RESTORING")
	ErrContentExpired      = errors.New("CONTENT_REF_EXPIRED")
	ErrInvalidState        = errors.New("INVALID_SESSION_STATE")
)

// Cursor positions are meaningful only within their project scene. Decimal
// strings preserve int64 precision in JavaScript and in persisted client state.
type Cursor struct {
	ProjectID     string `json:"project_id"`
	ThreadID      string `json:"thread_id"`
	SceneRevision int64  `json:"scene_revision,string"`
	Seq           int64  `json:"seq,string"`
}

func (c Cursor) Encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeCursor(value string) (Cursor, error) {
	var c Cursor
	if len(value) == 0 || len(value) > 2048 {
		return c, ErrCursorReset
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(raw, &c) != nil || c.ProjectID == "" || c.ThreadID == "" || c.SceneRevision < 1 || c.Seq < 0 {
		return Cursor{}, ErrCursorReset
	}
	return c, nil
}

type Fence struct {
	ProjectID         string
	ThreadID          string
	SceneRevision     int64
	RunID             string
	OwnerInstanceID   string
	ExecutionRevision int64
}

type CommitIdentity struct {
	Key         string
	RequestHash string
}

type Receipt struct {
	Related            []RelatedCommit `json:"related,omitempty"`
	Result             json.RawMessage `json:"result,omitempty"`
	CommitID           string          `json:"commit_id"`
	Cursor             string          `json:"cursor"`
	SceneRevision      int64           `json:"scene_revision,string"`
	Seq                int64           `json:"seq,string"`
	ContextRevision    int64           `json:"context_revision,string"`
	CheckpointRevision int64           `json:"checkpoint_revision,string"`
}

// RelatedCommit records another thread advanced by the same database commit.
// Its cursor is an acknowledgement only, never a client synchronization shortcut.
type RelatedCommit struct {
	ThreadID      string `json:"thread_id"`
	CommitID      string `json:"commit_id"`
	Cursor        string `json:"cursor"`
	SceneRevision int64  `json:"scene_revision,string"`
	Seq           int64  `json:"seq,string"`
}

// CommandMutation returns an action receipt separately from the current entity.
// The receipt cursor acknowledges the action; clients still consume every batch.
type CommandMutation struct {
	Command model.CommandExecution `json:"command"`
	Receipt Receipt                `json:"receipt"`
}

// Message bodies are immutable. Context updates carry only new bodies and the
// complete ordered ID collection, so saving never reads historical versions.
type Message struct {
	ID               string
	RunID            string
	InputID          string
	RoundID          string
	Kind             string
	Native           llm.Message
	Public           json.RawMessage
	SourceMessageIDs []string
}

type ContextSnapshot struct {
	ThreadID      string
	SceneRevision int64
	Revision      int64
	Messages      []Message
}

type ContextUpdate struct {
	ExpectedRevision int64
	MessageIDs       []string
	NewMessages      []Message
	Reason           string
}

type ThreadView struct {
	ContextRevision    int64             `json:"context_revision,string"`
	ID                 string            `json:"id"`
	ProjectID          string            `json:"project_id"`
	Title              string            `json:"title"`
	CreatedAt          int64             `json:"created_at"`
	UpdatedAt          int64             `json:"updated_at"`
	AutoSuggestEnabled bool              `json:"auto_suggest_enabled"`
	AutoRenameEnabled  bool              `json:"auto_rename_enabled"`
	NamingRevision     int64             `json:"naming_revision,string"`
	EntityRevision     int64             `json:"entity_revision,string"`
	UpdatedSeq         int64             `json:"updated_seq,string"`
	Plan               *model.PublicPlan `json:"plan"`
	PlanRevision       int64             `json:"plan_revision,string"`
	PlanRunID          *string           `json:"plan_run_id"`
	ActiveRunID        *string           `json:"active_run_id"`
	Suggestions        json.RawMessage   `json:"suggestions"`
}

type RunView struct {
	ID                       string                       `json:"id"`
	ThreadID                 string                       `json:"thread_id"`
	ProjectID                string                       `json:"project_id"`
	SceneRevision            int64                        `json:"scene_revision,string"`
	EntityRevision           int64                        `json:"entity_revision,string"`
	UpdatedSeq               int64                        `json:"updated_seq,string"`
	Status                   string                       `json:"status"`
	Scope                    model.RunScope               `json:"scope"`
	Mode                     model.RunMode                `json:"mode"`
	Model                    *string                      `json:"model"`
	Skills                   []model.PublicSkill          `json:"skills"`
	Components               []model.PublicLoadedResource `json:"components"`
	DroppedMentionedSlideIDs []string                     `json:"dropped_mentioned_slide_ids"`
	CanContinue              bool                         `json:"can_continue"`
	PauseReason              string                       `json:"pause_reason,omitempty"`
	PausedAt                 int64                        `json:"paused_at,omitempty"`
	CancelRequestedAt        int64                        `json:"cancel_requested_at,omitempty"`
	ActiveDurationMS         int64                        `json:"active_duration_ms"`
	ActiveSince              *int64                       `json:"active_since"`
	Progress                 json.RawMessage              `json:"progress"`
	Usage                    json.RawMessage              `json:"usage"`
	Terminal                 json.RawMessage              `json:"terminal"`
	TerminalRef              string                       `json:"terminal_ref,omitempty"`
	ContextWindow            json.RawMessage              `json:"context_window"`
	CreatedAt                int64                        `json:"created_at"`
	UpdatedAt                int64                        `json:"updated_at"`
}

type TimelineGroup struct {
	ItemsMode       string          `json:"items_mode,omitempty"`
	HasMoreItems    bool            `json:"has_more_items"`
	OlderItemCursor *string         `json:"older_item_cursor"`
	ID              string          `json:"id"`
	ThreadID        string          `json:"thread_id"`
	OrderSeq        int64           `json:"order_seq,string"`
	Kind            string          `json:"kind"`
	RunID           string          `json:"run_id,omitempty"`
	CommandID       string          `json:"command_id,omitempty"`
	InputID         string          `json:"input_id,omitempty"`
	Status          string          `json:"status"`
	Summary         json.RawMessage `json:"summary"`
	ItemCount       int             `json:"item_count"`
	EntityRevision  int64           `json:"entity_revision,string"`
	UpdatedSeq      int64           `json:"updated_seq,string"`
}

type TimelineItem struct {
	CreatedSeq     int64           `json:"created_seq,string"`
	ID             string          `json:"id"`
	ThreadID       string          `json:"thread_id"`
	GroupID        string          `json:"group_id"`
	Position       int64           `json:"position,string"`
	Kind           string          `json:"kind"`
	EntityRevision int64           `json:"entity_revision,string"`
	UpdatedSeq     int64           `json:"updated_seq,string"`
	CreatedAt      int64           `json:"created_at"`
	MessageID      string          `json:"message_id,omitempty"`
	ToolCallID     string          `json:"tool_call_id,omitempty"`
	InteractionID  string          `json:"interaction_id,omitempty"`
	AttemptID      string          `json:"attempt_id,omitempty"`
	Summary        json.RawMessage `json:"summary"`
	BodyRef        string          `json:"body_ref,omitempty"`
	DetailRef      string          `json:"detail_ref,omitempty"`
}

type Change struct {
	Type           string          `json:"type"`
	Entity         json.RawMessage `json:"entity,omitempty"`
	ID             string          `json:"id,omitempty"`
	EntityRevision int64           `json:"entity_revision,omitempty,string"`
}

type Batch struct {
	ProjectID     string   `json:"project_id"`
	ThreadID      string   `json:"thread_id"`
	SceneRevision int64    `json:"scene_revision,string"`
	Seq           int64    `json:"seq,string"`
	CommitID      string   `json:"commit_id"`
	Changes       []Change `json:"changes"`
}

func (b Batch) Cursor() Cursor {
	return Cursor{ProjectID: b.ProjectID, ThreadID: b.ThreadID, SceneRevision: b.SceneRevision, Seq: b.Seq}
}

func Upsert(kind string, entity any) (Change, error) {
	switch kind {
	case "thread", "run", "command", "interaction", "timeline.group", "timeline.item":
	default:
		return Change{}, fmt.Errorf("%w: unknown entity %q", ErrInvalidState, kind)
	}
	raw, err := json.Marshal(entity)
	return Change{Type: kind + ".upsert", Entity: raw}, err
}

type ChangePage struct {
	Batches []Batch `json:"batches"`
	Cursor  string  `json:"cursor"`
	HasMore bool    `json:"has_more"`
}

type ContentRef struct {
	ThreadID      string `json:"thread_id"`
	ItemID        string `json:"item_id"`
	SceneRevision int64  `json:"scene_revision,string"`
	ItemRevision  int64  `json:"item_revision,string"`
	SourceType    string `json:"source_type"`
	SourceID      string `json:"source_id"`
}

func (r ContentRef) Encode() string {
	raw, _ := json.Marshal(r)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeContentRef(value string) (ContentRef, error) {
	var r ContentRef
	if len(value) == 0 || len(value) > 4096 {
		return r, ErrContentExpired
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(raw, &r) != nil || r.ThreadID == "" || r.ItemID == "" || r.SourceID == "" || r.SceneRevision < 1 || r.ItemRevision < 1 || (r.SourceType != "message" && r.SourceType != "detail") {
		return ContentRef{}, ErrContentExpired
	}
	return r, nil
}
