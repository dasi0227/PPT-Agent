// Package testsupport supplies deterministic file-backed journal fixtures. It
// deliberately has no database or workflow dependency and is not used at runtime.
package testsupport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

type Journal struct {
	root string
	mu   sync.Mutex
}

func NewJournal(workDir string) *Journal { return &Journal{root: model.ProjectRoot(workDir)} }
func (j *Journal) read(threadID string) ([]threadjournal.Event, error) {
	events, err := threadjournal.Read(j.root, filepath.Base(j.root), threadID)
	if errors.Is(err, os.ErrNotExist) {
		return []threadjournal.Event{}, nil
	}
	return events, err
}
func (j *Journal) AppendThreadEvent(_ context.Context, threadID string, event threadjournal.Event) (threadjournal.Event, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	events, err := j.read(threadID)
	if err != nil {
		return event, err
	}
	event.ID = model.MustShortID("evt")
	event.Seq = int64(len(events) + 1)
	event.TS = time.Now().UnixMilli()
	event, err = threadjournal.Prepare(j.root, threadID, event)
	if err != nil {
		return event, err
	}
	return event, threadjournal.Deliver(j.root, filepath.Base(j.root), threadID, event)
}
func (j *Journal) ThreadEvents(_ context.Context, threadID string, after int64) ([]threadjournal.Event, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	events, err := j.read(threadID)
	if err != nil {
		return nil, err
	}
	out := []threadjournal.Event{}
	for _, e := range events {
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out, nil
}
func (j *Journal) FlushThreadEvents(context.Context, string) error { return nil }
