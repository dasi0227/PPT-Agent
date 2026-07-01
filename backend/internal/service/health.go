// Package service 承载业务用例编排，依赖 store/llm 的 interface，不依赖其具体实现或 gin 类型。
package service

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// HealthService 校验后端关键依赖（当前为 store）是否就绪。
type HealthService struct {
	store store.Store
}

func NewHealthService(s store.Store) *HealthService {
	return &HealthService{store: s}
}

func (h *HealthService) Check(ctx context.Context) error {
	return h.store.Health(ctx)
}
