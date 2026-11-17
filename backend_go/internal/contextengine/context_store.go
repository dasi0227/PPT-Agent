package contextengine

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// ContextBackend reads authoritative current snapshots, never historical edits.
type ContextBackend interface {
	LoadContext(context.Context, string) (sessionstate.ContextSnapshot, error)
	LoadContextVersion(context.Context, string, int64) (sessionstate.ContextSnapshot, error)
}

type ContextStore struct{ backend ContextBackend }

func NewContextStore(backend ContextBackend) *ContextStore { return &ContextStore{backend: backend} }

func (s *ContextStore) Open(ctx context.Context, threadID string, revision *int64) (*ContextSession, error) {
	var snapshot sessionstate.ContextSnapshot
	var err error
	if revision == nil {
		snapshot, err = s.backend.LoadContext(ctx, threadID)
	} else {
		snapshot, err = s.backend.LoadContextVersion(ctx, threadID, *revision)
	}
	if err != nil {
		return nil, err
	}
	return NewContextSession(snapshot)
}
