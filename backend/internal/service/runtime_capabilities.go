package service

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func optionalContextIndexStore(s store.Store) workflow.ContextIndexStore {
	if value, ok := s.(interface {
		SaveContextIndex(context.Context, workflow.ContextIndex) (string, error)
		GetContextIndex(context.Context, string) (workflow.ContextIndex, error)
		LatestContextIndex(context.Context, string) (workflow.ContextIndex, error)
	}); ok {
		return value
	}
	return nil
}

func optionalSemanticReviewStore(s store.Store) workflow.SemanticReviewStore {
	if value, ok := s.(interface {
		SaveSemanticReview(context.Context, workflow.StoredSemanticReview) error
	}); ok {
		return value
	}
	return nil
}
