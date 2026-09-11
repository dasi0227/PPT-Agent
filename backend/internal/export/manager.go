package export

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

const defaultTTL = 30 * time.Minute

type Manager struct {
	mu        sync.Mutex
	byID      map[string]*Operation
	byProject map[string]string
	requests  map[string]string
	renderer  workflow.SlideRenderer
	ttl       time.Duration
	closed    bool
}

func NewManager(renderer workflow.SlideRenderer) *Manager {
	return &Manager{byID: map[string]*Operation{}, byProject: map[string]string{}, requests: map[string]string{}, renderer: renderer, ttl: defaultTTL}
}

func (m *Manager) Existing(projectID, requestID string) (*Operation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.requests[projectID+"\x00"+requestID]
	if !ok {
		return nil, false
	}
	op, ok := m.byID[id]
	return op, ok
}

func (m *Manager) Active(projectID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.byProject[projectID]
	return ok
}

func (m *Manager) Start(id, requestID string, format Format, snapshot Snapshot) (*Operation, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrGone
	}
	if _, exists := m.byProject[snapshot.ProjectID]; exists {
		m.mu.Unlock()
		return nil, ErrAlreadyActive
	}
	ctx, cancel := context.WithCancel(context.Background())
	op := &Operation{ID: id, ProjectID: snapshot.ProjectID, ClientRequestID: requestID, Format: format,
		Status: StatusAccepted, Phase: PhaseSnapshotting, TotalPages: len(snapshot.Slides), Warnings: []string{},
		Snapshot: snapshot, CreatedAt: time.Now(), cancel: cancel, subscribers: map[int]chan Event{}, done: make(chan struct{})}
	m.byID[id] = op
	m.byProject[snapshot.ProjectID] = id
	m.requests[snapshot.ProjectID+"\x00"+requestID] = id
	m.mu.Unlock()
	op.publish("export.progress")
	go m.execute(ctx, op)
	go m.expire(id, ctx)
	return op, nil
}

func (m *Manager) Get(id string) (*Operation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op := m.byID[id]
	if op == nil {
		return nil, ErrGone
	}
	return op, nil
}

func (m *Manager) Subscribe(id string, after int64) (<-chan Event, func(), error) {
	op, err := m.Get(id)
	if err != nil {
		return nil, nil, err
	}
	op.mu.Lock()
	history := make([]Event, 0, len(op.events))
	for _, event := range op.events {
		if event.Seq > after {
			history = append(history, event)
		}
	}
	channel := make(chan Event, len(history)+16)
	for _, event := range history {
		channel <- event
	}
	if op.Status.Terminal() {
		close(channel)
		op.mu.Unlock()
		return channel, func() {}, nil
	}
	subscriberID := op.nextSubscriber
	op.nextSubscriber++
	op.subscribers[subscriberID] = channel
	op.mu.Unlock()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			op.mu.Lock()
			if current := op.subscribers[subscriberID]; current != nil {
				delete(op.subscribers, subscriberID)
				close(current)
			}
			op.mu.Unlock()
		})
	}
	return channel, stop, nil
}

func (m *Manager) Cancel(id string) error {
	op, err := m.Get(id)
	if err != nil {
		return err
	}
	op.cancel()
	op.mu.Lock()
	if !op.Status.Terminal() {
		op.Status, op.Error = StatusCanceled, &PublicError{Code: "EXPORT_CANCELED", Message: "导出已取消。"}
		op.publishLocked("export.canceled")
	}
	op.mu.Unlock()
	select {
	case <-op.done:
	case <-time.After(5 * time.Second):
	}
	m.cleanup(op, true)
	return nil
}

func (m *Manager) CancelProject(projectID string) {
	m.mu.Lock()
	id := m.byProject[projectID]
	m.mu.Unlock()
	if id != "" {
		_ = m.Cancel(id)
	}
}

func (m *Manager) BeginDelivery(id string) (*Operation, error) {
	op, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	if op.Status == StatusDelivering {
		return nil, ErrDownloadInProgress
	}
	if op.Status != StatusReady || op.Artifact == nil {
		return nil, ErrNotReady
	}
	op.Status = StatusDelivering
	op.publishLocked("export.delivery_started")
	return op, nil
}

func (m *Manager) DeliveryFailed(op *Operation) {
	op.mu.Lock()
	if op.Status == StatusDelivering {
		op.Status = StatusReady
		op.publishLocked("export.download_failed")
	}
	op.mu.Unlock()
}

func (m *Manager) Consume(op *Operation) {
	op.mu.Lock()
	if op.Status == StatusDelivering {
		op.Status = StatusConsumed
		op.publishLocked("export.consumed")
	}
	op.mu.Unlock()
	op.cancel()
	m.cleanup(op, true)
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	ids := make([]string, 0, len(m.byID))
	for id := range m.byID {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Cancel(id)
	}
}

func (m *Manager) execute(ctx context.Context, op *Operation) {
	defer close(op.done)
	op.mu.Lock()
	op.Status, op.Phase = StatusRunning, PhasePackaging
	if op.Format != FormatHTML {
		op.Phase = PhaseRendering
	}
	op.publishLocked("export.progress")
	op.mu.Unlock()
	artifact, warnings, publicErr := buildArtifact(ctx, op, m.renderer)
	if publicErr != nil {
		op.mu.Lock()
		if op.Status != StatusCanceled {
			op.Status, op.Error = StatusFailed, publicErr
			op.publishLocked("export.failed")
		}
		op.mu.Unlock()
		op.cancel()
		m.cleanup(op, false)
		return
	}
	artifact.Download = "/api/v1/exports/" + op.ID + "/download"
	op.mu.Lock()
	if ctx.Err() != nil || op.Status == StatusCanceled {
		op.mu.Unlock()
		_ = os.Remove(artifact.Path)
		return
	}
	op.Artifact = artifact
	op.Warnings = append(op.Warnings, warnings...)
	op.Status, op.Phase, op.CompletedPages = StatusReady, PhasePackaging, op.TotalPages
	op.publishLocked("export.ready")
	op.mu.Unlock()
}

// CleanupWorkRoot removes orphaned ephemeral exports left by an earlier process.
func (m *Manager) CleanupWorkRoot(workRoot string) error {
	projectsRoot := filepath.Join(workRoot, "projects")
	projects, err := os.ReadDir(projectsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, project := range projects {
		if !project.IsDir() {
			continue
		}
		if err := os.RemoveAll(filepath.Join(projectsRoot, project.Name(), ".runtime", "exports")); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) expire(id string, ctx context.Context) {
	timer := time.NewTimer(m.ttl)
	defer timer.Stop()
	select {
	case <-timer.C:
		_ = m.Cancel(id)
	case <-ctx.Done():
	}
}

func (m *Manager) cleanup(op *Operation, remove bool) {
	_ = os.RemoveAll(filepath.Dir(op.Snapshot.Root))
	m.mu.Lock()
	if remove {
		delete(m.byID, op.ID)
		delete(m.requests, op.ProjectID+"\x00"+op.ClientRequestID)
	}
	if m.byProject[op.ProjectID] == op.ID {
		delete(m.byProject, op.ProjectID)
	}
	m.mu.Unlock()
}

func (o *Operation) progress(phase Phase, slideID string, ordinal, completed int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.Status == StatusCanceled {
		return
	}
	o.Phase, o.CurrentSlideID, o.CurrentOrdinal = phase, slideID, ordinal
	if completed > o.CompletedPages {
		o.CompletedPages = completed
	}
	o.publishLocked("export.progress")
}

func (o *Operation) publish(kind string) { o.mu.Lock(); defer o.mu.Unlock(); o.publishLocked(kind) }
func (o *Operation) publishLocked(kind string) {
	o.seq++
	event := Event{Seq: o.seq, Type: kind, Payload: eventPayload(o.viewLocked())}
	o.events = append(o.events, event)
	for _, channel := range o.subscribers {
		select {
		case channel <- event:
		default:
		}
	}
	if o.Status.Terminal() {
		for id, channel := range o.subscribers {
			delete(o.subscribers, id)
			close(channel)
		}
	}
}

func publicFailure(code, message string, retryable bool, err error) *PublicError {
	details := map[string]any{}
	if err != nil && !errors.Is(err, context.Canceled) {
		details["reason"] = "导出处理失败"
	}
	return &PublicError{Code: code, Message: message, Details: details, Retryable: retryable}
}
