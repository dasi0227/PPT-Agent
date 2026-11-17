package workflow

import (
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// Durable observations are staged until their context/checkpoint boundary.
// Independent progress can be published while a model or approval is pending.
type publicUpdates struct {
	source  EventEmitter
	mu      sync.Mutex
	pending []sessionstate.RunUpdate
	err     error
}

func (p *publicUpdates) Emit(kind model.EventType, payload any) {
	switch kind {
	case model.EventRunProgress, model.EventContextWindowUpdated, model.EventPreviewUpdated:
		if p.source != nil {
			p.source.Emit(kind, payload)
		}
		return
	}
	p.stage(kind, payload)
}

func (p *publicUpdates) stage(kind model.EventType, payload any) {
	if filter, ok := p.source.(interface {
		PreparePublicUpdate(model.EventType, any) (any, bool)
	}); ok {
		var allowed bool
		payload, allowed = filter.PreparePublicUpdate(kind, payload)
		if !allowed {
			return
		}
	}
	update, err := sessionstate.NewRunUpdate(kind, payload)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return
	}
	if err != nil {
		p.err = err
		return
	}
	p.pending = append(p.pending, update)
}

type checkpointUpdates struct{ updates *publicUpdates }

func (p checkpointUpdates) Emit(kind model.EventType, payload any) {
	p.updates.stage(kind, payload)
}

func (p *publicUpdates) Err() error {
	p.mu.Lock()
	err := p.err
	p.mu.Unlock()
	if err != nil {
		return err
	}
	if source, ok := p.source.(interface{ Err() error }); ok {
		return source.Err()
	}
	return nil
}

func (p *publicUpdates) snapshot() []sessionstate.RunUpdate {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]sessionstate.RunUpdate(nil), p.pending...)
}

func (p *publicUpdates) committed(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending = append([]sessionstate.RunUpdate(nil), p.pending[count:]...)
}
