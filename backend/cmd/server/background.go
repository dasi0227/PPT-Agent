package main

import (
	"context"
	"time"

	"go.uber.org/zap"
)

func startSessionMaintenance(parent context.Context, prune func(context.Context) error, log *zap.Logger) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pass, stop := context.WithTimeout(ctx, 5*time.Second)
				err := prune(pass)
				stop()
				if err != nil && ctx.Err() == nil {
					log.Warn("session change maintenance failed", zap.Error(err))
				}
			}
		}
	}()
	return func() { cancel(); <-done }
}
