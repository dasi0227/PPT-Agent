// Package httpapi 是 HTTP 层：gin handler、路由、中间件。gin.Context 仅限本层（ARCH-BACKEND-006）。
package httpapi

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// Router 持有 gin 引擎与各 handler 依赖，负责路由注册。
type Router struct {
	engine *gin.Engine
	cfg    *config.Config
	log    *zap.Logger
	health *HealthHandler
}

func NewRouter(cfg *config.Config, log *zap.Logger, health *HealthHandler) *Router {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(RequestID(), RecoverWithZap(log), LogWithZap(log))

	r := &Router{engine: engine, cfg: cfg, log: log, health: health}
	r.register()
	return r
}

func (r *Router) register() {
	v1 := r.engine.Group("/api/v1")
	v1.GET("/healthz", r.health.Healthz)
}

// Engine 暴露底层 gin 引擎供 server 启动使用。
func (r *Router) Engine() *gin.Engine {
	return r.engine
}
