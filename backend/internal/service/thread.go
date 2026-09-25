package service

import (
	"context"
	"encoding/json"

	"os"

	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"path/filepath"
)

// ThreadService manages conversation metadata and public thread journal projections.
type ThreadService struct {
	store       store.Store
	clock       func() int64
	newID       func() string
	transcripts *contextengine.JournalTranscriptStore
}

type CreateThreadParams struct {
	Title string
}

func NewThreadService(s store.Store) *ThreadService {
	return NewThreadServiceWithTranscript(s, contextengine.NewJournalTranscriptStore(s))
}

func NewThreadServiceWithTranscript(s store.Store, transcripts *contextengine.JournalTranscriptStore) *ThreadService {
	if transcripts == nil {
		transcripts = contextengine.NewJournalTranscriptStore(s)
	}
	return &ThreadService{
		store: s, clock: func() int64 { return time.Now().Unix() }, newID: uuid.NewString,
		transcripts: transcripts,
	}
}

func (svc *ThreadService) CreateThread(ctx context.Context, projectID string, p CreateThreadParams) (model.Thread, error) {
	_, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return model.Thread{}, err
	}
	id := svc.newID()
	now := svc.clock()
	title := strings.TrimSpace(p.Title)
	if title != "" {
		title, err = ValidateThreadTitle(title)
		if err != nil {
			return model.Thread{}, err
		}
	}
	th := model.Thread{
		ID:                     id,
		ProjectID:              projectID,
		Title:                  title,
		AutoRenameEnabled:      title == "",
		NamingRevision:         1,
		RenameOperationVersion: 1,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := svc.store.CreateThread(ctx, th); err != nil {
		return model.Thread{}, err
	}
	return th, nil
}

func (svc *ThreadService) RenameThread(ctx context.Context, id, title string) (model.Thread, error) {
	clean, err := ValidateThreadTitle(title)
	if err != nil {
		return model.Thread{}, err
	}
	enabled := false
	return svc.store.UpdateThreadNamingState(ctx, id, &clean, &enabled, true, svc.clock())
}

func (svc *ThreadService) ListThreads(ctx context.Context, projectID string) ([]model.Thread, error) {
	if _, err := svc.store.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	return svc.store.ListThreads(ctx, projectID)
}

func (svc *ThreadService) GetThread(ctx context.Context, id string) (model.Thread, error) {
	return svc.store.GetThread(ctx, id)
}

func (svc *ThreadService) DeleteThread(ctx context.Context, id string) error {
	th, err := svc.store.GetThread(ctx, id)
	if err != nil {
		return err
	}
	proj, err := svc.store.GetProject(ctx, th.ProjectID)
	if err != nil {
		return err
	}
	if err := svc.store.DeleteThread(ctx, id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(model.ProjectRoot(proj.WorkDir), "threads", th.ID))
}

func (svc *ThreadService) History(ctx context.Context, id string) ([]map[string]any, error) {
	events, err := svc.store.ThreadEvents(ctx, id, 0)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, event := range events {
		entry, visible, err := PublicThreadEvent(event)
		if err != nil {
			return nil, err
		}
		if visible {
			out = append(out, entry)
		}
	}
	return out, nil
}

// PublicThreadEvent is shared by history and SSE. Internal diagnostics and model
// projection operations never become frontend payloads.
func PublicThreadEvent(e threadjournal.Event) (map[string]any, bool, error) {
	entry := map[string]any{"id": e.ID, "seq": e.Seq, "ts": e.TS, "run_id": e.RunID, "command_id": e.CommandID, "attempt_id": e.AttemptID, "turn": "agent", "type": e.Type}
	var data map[string]any
	switch e.Type {
	case "run.accepted":
		var a struct {
			Command model.RunCommand `json:"command"`
		}
		if err := json.Unmarshal(e.Payload, &a); err != nil {
			return nil, false, err
		}
		entry["turn"] = "user"
		entry["type"] = "user_turn"
		data = map[string]any{"text": a.Command.Instruction, "scope": a.Command.Scope, "mode": a.Command.Mode, "skills": a.Command.PublicSkills(), "resources": a.Command.PublicComponents(), "attachments": a.Command.Attachments, "dom_selections": model.PublicDOMSelections(a.Command.DOMSelections), "reference_order": a.Command.ReferenceOrder}
	case "steering.accepted":
		var a model.SteeringMessage
		if err := json.Unmarshal(e.Payload, &a); err != nil {
			return nil, false, err
		}
		entry["turn"] = "user"
		entry["type"] = "steering"
		data = map[string]any{"client_message_id": a.ClientMessageID, "text": a.Content, "attachments": a.Attachments, "dom_selections": model.PublicDOMSelections(a.DOMSelections), "reference_order": a.ReferenceOrder, "status": a.Status}
	case "steering.injected", "steering.rejected", "command.accepted", "command.running", "command.cancel_requested", "command.completed", "command.failed", "command.canceled", "command.interrupted":
		if err := json.Unmarshal(e.Payload, &data); err != nil {
			return nil, false, err
		}
	case "briefing.result", "context.compaction_result":
		return nil, false, nil
	case "run.started":
		return nil, false, nil // Input already has its own accepted identity.
	default:
		var payload map[string]any
		if json.Unmarshal(e.Payload, &payload) != nil {
			return nil, false, threadjournal.ErrCorrupt
		}
		if e.RunID == "" || model.ValidatePublicEvent(model.EventType(e.Type), payload) != nil {
			return nil, false, nil
		}
		data = payload
	}
	if strings.HasPrefix(e.Type, "command.") && data["source"] == "automatic" {
		return nil, false, nil
	}
	entry["data"] = data
	return entry, true, nil
}

func (svc *ThreadService) JournalEvents(ctx context.Context, threadID string, after int64) ([]threadjournal.Event, error) {
	return svc.store.ThreadEvents(ctx, threadID, after)
}
func (svc *ThreadService) CommandStore() CommandStore {
	value, _ := svc.store.(CommandStore)
	return value
}
