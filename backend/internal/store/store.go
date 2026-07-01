// Package store 定义持久化契约（interface），具体实现（GORM/FS）可替换（ARCH-BACKEND-001）。
package store

import "context"

// Store 是持久化层对外暴露的接口。M0 仅含健康检查，随里程碑推进逐步扩展领域方法。
type Store interface {
	Health(ctx context.Context) error
}
