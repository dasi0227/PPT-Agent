package run

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type steeringHandoffKey struct{}

// SteeringHandoff is an internal start contract. The first message becomes the
// new Run's input; the remaining messages become its ordered steering inbox.
type SteeringHandoff struct {
	SourceRunID string
	Messages    []model.SteeringMessage
}

func WithSteeringHandoff(ctx context.Context, sourceRunID string, messages []model.SteeringMessage) context.Context {
	return context.WithValue(ctx, steeringHandoffKey{}, SteeringHandoff{SourceRunID: sourceRunID, Messages: messages})
}

func SteeringHandoffFromContext(ctx context.Context) (SteeringHandoff, bool) {
	handoff, ok := ctx.Value(steeringHandoffKey{}).(SteeringHandoff)
	return handoff, ok && handoff.SourceRunID != "" && len(handoff.Messages) > 0
}
