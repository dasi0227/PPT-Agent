package service

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var commandActivityID = regexp.MustCompile(`^[A-Za-z0-9:_-]{1,128}$`)

func (svc *ThreadService) BeginCommandActivity(ctx context.Context, id, threadID, kind, method string, request json.RawMessage) (model.CommandActivity, error) {
	if id == "" {
		id = "command:" + model.MustShortID("cmd")
	}
	if !commandActivityID.MatchString(id) {
		return model.CommandActivity{}, model.NewAgentError("BAD_REQUEST", "command", nil)
	}
	thread, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.CommandActivity{}, err
	}
	now := time.Now().UnixMilli()
	return svc.store.BeginCommandActivity(ctx, model.CommandActivity{
		ID: id, AttemptID: model.MustShortID("attempt"), ThreadID: threadID, ProjectID: thread.ProjectID,
		Kind: kind, Method: method, Status: "loading", Phase: -1, PreviousTitle: thread.Title,
		Request: request, Result: json.RawMessage("null"), CreatedAt: now, UpdatedAt: now,
	})
}

func (svc *ThreadService) SaveCommandActivity(ctx context.Context, activity model.CommandActivity) error {
	activity.UpdatedAt = time.Now().UnixMilli()
	return svc.store.SaveCommandActivity(ctx, activity)
}

// Automatic compaction is owned by a run. Its progress and terminal outcome
// already live in run_events; project them without starting another command.
func interruptedCompactionActivities(events []model.Event, thread model.Thread) []model.CommandActivity {
	activities := map[string]model.CommandActivity{}
	order := []string{}
	for _, event := range events {
		var payload struct {
			Compaction *struct {
				ID    string `json:"id"`
				Phase int    `json:"phase"`
			} `json:"compaction"`
		}
		if event.Type == model.EventContextWindowUpdated || event.Type == model.EventContextCompacted {
			if json.Unmarshal([]byte(event.Payload), &payload) != nil || payload.Compaction == nil {
				continue
			}
			id := payload.Compaction.ID
			if event.Type == model.EventContextCompacted {
				delete(activities, id)
				continue
			}
			activity, exists := activities[id]
			if !exists {
				order = append(order, id)
				activity = model.CommandActivity{
					ID: "context-compaction:" + id, AttemptID: event.RunID,
					ThreadID: thread.ID, ProjectID: thread.ProjectID, Kind: "compact", Method: "auto",
					Status: "loading", Request: json.RawMessage("{}"), Result: json.RawMessage("null"), CreatedAt: event.CreatedAt * 1000,
				}
			}
			activity.Phase = payload.Compaction.Phase
			activity.UpdatedAt = event.CreatedAt * 1000
			activities[id] = activity
		}
		switch event.Type {
		case model.EventRunCompleted, model.EventRunFailed, model.EventRunError, model.EventRunCanceled, model.EventRunResumed:
			for id, activity := range activities {
				if activity.AttemptID != event.RunID || activity.Status != "loading" {
					continue
				}
				activity.Status = "failed"
				if event.Type == model.EventRunCanceled || event.Type == model.EventRunResumed {
					activity.Status = "canceled"
				}
				activity.UpdatedAt = event.CreatedAt * 1000
				activities[id] = activity
			}
		}
	}
	out := make([]model.CommandActivity, 0, len(activities))
	for _, id := range order {
		if activity, exists := activities[id]; exists {
			out = append(out, activity)
		}
	}
	return out
}
