package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/google/uuid"
)

type ThreadEvent struct {
	ID       string
	Event    string
	Payload  map[string]any
	Epoch    string
	Sequence int64
}

type projectThreadStream struct {
	epoch       string
	sequence    int64
	nextSubID   int
	subscribers map[int]chan ThreadEvent
}

type ThreadEventHub struct {
	store     store.Store
	mu        sync.Mutex
	byProject map[string]*projectThreadStream
}

func NewThreadEventHub(s store.Store) *ThreadEventHub {
	return &ThreadEventHub{store: s, byProject: map[string]*projectThreadStream{}}
}

func (h *ThreadEventHub) streamLocked(projectID string) *projectThreadStream {
	stream := h.byProject[projectID]
	if stream == nil {
		stream = &projectThreadStream{epoch: uuid.NewString(), subscribers: map[int]chan ThreadEvent{}}
		h.byProject[projectID] = stream
	}
	return stream
}

func (h *ThreadEventHub) Epoch(projectID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.streamLocked(projectID).epoch
}

func threadEventValue(thread model.Thread) map[string]any {
	return map[string]any{
		"id": thread.ID, "project_id": thread.ProjectID, "title": thread.Title,
		"history_path": thread.HistoryPath, "status": thread.Status,
		"auto_rename_enabled": thread.AutoRenameEnabled, "naming_revision": thread.NamingRevision,
		"created_at": thread.CreatedAt, "updated_at": thread.UpdatedAt,
	}
}

func (h *ThreadEventHub) eventLocked(projectID, name string, data map[string]any) ThreadEvent {
	stream := h.streamLocked(projectID)
	stream.sequence++
	data["schema_version"] = 1
	data["project_id"] = projectID
	data["stream_epoch"] = stream.epoch
	data["sequence"] = stream.sequence
	return ThreadEvent{
		ID: fmt.Sprintf("%s:%d", stream.epoch, stream.sequence), Event: name,
		Payload: data, Epoch: stream.epoch, Sequence: stream.sequence,
	}
}

func (h *ThreadEventHub) publish(projectID, name string, data map[string]any) {
	h.mu.Lock()
	event := h.eventLocked(projectID, name, data)
	stream := h.streamLocked(projectID)
	for id, subscriber := range stream.subscribers {
		select {
		case subscriber <- event:
		default:
			close(subscriber)
			delete(stream.subscribers, id)
		}
	}
	h.mu.Unlock()
}

func (h *ThreadEventHub) PublishUpdated(thread model.Thread) {
	h.publish(thread.ProjectID, "thread.naming.updated", map[string]any{"thread": threadEventValue(thread)})
}

func (h *ThreadEventHub) PublishResult(threadID, projectID, operationID, requestID, outcome, safeError string) {
	data := map[string]any{"thread_id": threadID, "request_id": requestID, "outcome": outcome}
	if operationID != "" {
		data["operation_id"] = operationID
	}
	if safeError != "" {
		data["error"] = safeError
	}
	h.publish(projectID, "thread.naming.result", data)
}

func (h *ThreadEventHub) Subscribe(ctx context.Context, projectID string) (<-chan ThreadEvent, func(), error) {
	h.mu.Lock()
	stream := h.streamLocked(projectID)
	stream.nextSubID++
	id := stream.nextSubID
	channel := make(chan ThreadEvent, 64)
	stream.subscribers[id] = channel
	threads, err := h.store.ListThreads(ctx, projectID)
	if err != nil {
		delete(stream.subscribers, id)
		close(channel)
		h.mu.Unlock()
		return nil, nil, err
	}
	values := make([]map[string]any, len(threads))
	for index, thread := range threads {
		values[index] = threadEventValue(thread)
	}
	snapshot := map[string]any{
		"schema_version": 1, "project_id": projectID, "stream_epoch": stream.epoch,
		"sequence": stream.sequence, "threads": values,
	}
	channel <- ThreadEvent{
		ID: fmt.Sprintf("%s:%d", stream.epoch, stream.sequence), Event: "threads.snapshot",
		Payload: snapshot, Epoch: stream.epoch, Sequence: stream.sequence,
	}
	h.mu.Unlock()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			h.mu.Lock()
			current := h.byProject[projectID]
			if current != nil {
				if subscriber, exists := current.subscribers[id]; exists {
					delete(current.subscribers, id)
					close(subscriber)
				}
			}
			h.mu.Unlock()
		})
	}
	return channel, stop, nil
}

func (h *ThreadEventHub) ResetProject(ctx context.Context, projectID string) {
	h.mu.Lock()
	stream := h.streamLocked(projectID)
	stream.epoch = uuid.NewString()
	stream.sequence = 0
	threads, err := h.store.ListThreads(ctx, projectID)
	values := []map[string]any{}
	if err == nil {
		values = make([]map[string]any, len(threads))
		for index, thread := range threads {
			values[index] = threadEventValue(thread)
		}
	}
	event := ThreadEvent{
		ID: fmt.Sprintf("%s:0", stream.epoch), Event: "threads.snapshot", Epoch: stream.epoch,
		Payload: map[string]any{"schema_version": 1, "project_id": projectID, "stream_epoch": stream.epoch, "sequence": int64(0), "threads": values},
	}
	for id, subscriber := range stream.subscribers {
		select {
		case subscriber <- event:
		default:
			close(subscriber)
			delete(stream.subscribers, id)
		}
	}
	h.mu.Unlock()
}
