// Package httpapi 是 HTTP 层：gin handler、路由、中间件。gin.Context 仅限本层（ARCH-BACKEND-006）。
package httpapi

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// Router 持有 gin 引擎与各 handler 依赖，负责路由注册。
type Router struct {
	engine  *gin.Engine
	cfg     *config.Config
	log     *zap.Logger
	health  *HealthHandler
	run     *RunHandler
	project *ProjectHandler
	thread  *ThreadHandler
	slide   *SlideHandler
	asset   *AssetHandler
}

func NewRouter(cfg *config.Config, log *zap.Logger, health *HealthHandler, runH *RunHandler, projectH *ProjectHandler, threadH *ThreadHandler, slideH *SlideHandler, assetH *AssetHandler) *Router {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(RequestID(), RecoverWithZap(log), LogWithZap(log))

	r := &Router{engine: engine, cfg: cfg, log: log, health: health, run: runH, project: projectH, thread: threadH, slide: slideH, asset: assetH}
	r.register()
	return r
}

func (r *Router) register() {
	v1 := r.engine.Group("/api/v1")
	v1.GET("/healthz", r.health.Healthz)

	// Project / Thread：API 契约入口，前端不需要绕过 HTTP 直接造数据。
	v1.GET("/projects", r.project.List)
	v1.POST("/projects", r.project.Create)
	v1.GET("/projects/:id", r.project.Get)
	v1.DELETE("/projects/:id", r.project.Delete)
	v1.GET("/projects/:id/slides", r.project.ListSlides)
	v1.GET("/projects/:id/threads", r.thread.List)
	v1.POST("/projects/:id/threads", r.thread.Create)
	v1.GET("/threads/:id", r.thread.Get)
	v1.DELETE("/threads/:id", r.thread.Delete)
	v1.GET("/threads/:id/history", r.thread.History)

	// Run：创建 / SSE 订阅 / HITL 输入 / 取消（40-api/rest-endpoints）。
	v1.POST("/threads/:id/runs", r.run.CreateRun)
	v1.GET("/runs/:id/events", r.run.Events)
	v1.POST("/runs/:id/input", r.run.Input)
	v1.DELETE("/runs/:id", r.run.Cancel)

	// Slide：读取 / 版本列表 / 回滚（40-api openapi /slides/{id}...）。
	v1.GET("/slides/:id", r.slide.GetSlide)
	v1.GET("/slides/:id/versions", r.slide.ListVersions)
	v1.POST("/slides/:id/rollback", r.slide.Rollback)

	v1.GET("/assets", r.asset.List)
	v1.POST("/assets", r.asset.Create)
	v1.GET("/assets/:id", r.asset.Get)
	v1.PATCH("/assets/:id", r.asset.Patch)
	v1.DELETE("/assets/:id", r.asset.Delete)
	v1.POST("/assets/:id/rollback", r.asset.Rollback)
}

// Engine 暴露底层 gin 引擎供 server 启动使用。
func (r *Router) Engine() *gin.Engine {
	return r.engine
}
