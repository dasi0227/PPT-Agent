package main

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// startSessionMaintenance 每分钟清理一次超出保留范围的会话增量记录，返回停止并等待任务退出的函数。
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
				// 每轮最多执行 5 秒，避免维护任务长时间占用资源。
				pass, stop := context.WithTimeout(ctx, 5*time.Second)
				err := prune(pass)
				stop()
				if err != nil && ctx.Err() == nil {
					log.Warn("session change maintenance failed", zap.Error(err))
				}
			}
		}
	}()
	// 仅发出取消信号还不够：必须等正在执行的清理结束，主流程才能安全关闭数据库。
	return func() { cancel(); <-done }
}
