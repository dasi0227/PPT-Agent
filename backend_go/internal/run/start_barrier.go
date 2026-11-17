package run

import "context"

type startBarrierKey struct{}

// WithStartBarrier commits the caller's authoring transaction after durable
// acceptance and before any background task can perform external effects.
func WithStartBarrier(ctx context.Context, commit func() error) context.Context {
	return context.WithValue(ctx, startBarrierKey{}, commit)
}
func CommitStartBarrier(ctx context.Context) error {
	if commit, ok := ctx.Value(startBarrierKey{}).(func() error); ok {
		return commit()
	}
	return nil
}
