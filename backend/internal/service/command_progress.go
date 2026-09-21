package service

import "context"

type commandProgressKey struct{}

// WithCommandProgress reports actual service boundaries, never elapsed-time estimates.
func WithCommandProgress(ctx context.Context, report func(int) error) context.Context {
	return context.WithValue(ctx, commandProgressKey{}, report)
}

func commandPhase(ctx context.Context, phase int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if report, ok := ctx.Value(commandProgressKey{}).(func(int) error); ok {
		return report(phase)
	}
	return nil
}
