// Package httpapi 是 HTTP 层：gin handler、路由、中间件。gin.Context 仅限本层（ARCH-BACKEND-006）。
package httpapi

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Router 持有 gin 引擎与各 handler 依赖，负责路由注册。
type Router struct {
	commands      *service.CommandService
	history       *projecthistory.Manager
	source        *service.SlideSourceService
	engine        *gin.Engine
	cfg           *config.Config
	log           *zap.Logger
	health        *HealthHandler
	run           *RunHandler
	project       *ProjectHandler
	thread        *ThreadHandler
	slide         *SlideHandler
	repository    *RepositoryHandler
	llm           *LLMHandler
	polish        *PolishHandler
	briefing      *BriefingHandler
	gitCommit     *GitCommitHandler
	resources     *ResourceHandler
	contextWindow *ContextWindowHandler
	attachment    *AttachmentHandler
	export        *ExportHandler
}

func (r *Router) WithExportHandler(handler *ExportHandler) *Router {
	r.export = handler
	r.registerExport()
	return r
}

func (r *Router) registerExport() {
	if r.export == nil {
		return
	}
	v1 := r.engine.Group("/api/v1")
	v1.POST("/projects/:id/exports", r.export.Create)
	v1.GET("/exports/:id", r.export.Get)
	v1.POST("/exports/:id/heartbeat", r.export.Heartbeat)
	v1.GET("/exports/:id/events", r.export.Events)
	v1.GET("/exports/:id/download", r.export.Download)
	v1.DELETE("/exports/:id", r.export.Cancel)
}

func NewRouter(cfg *config.Config, log *zap.Logger, health *HealthHandler, runH *RunHandler, projectH *ProjectHandler, threadH *ThreadHandler, slideH *SlideHandler, repositoryH *RepositoryHandler, llmH *LLMHandler, polishH *PolishHandler, briefingH *BriefingHandler, gitCommitH *GitCommitHandler, resourceH *ResourceHandler, contextWindowH *ContextWindowHandler, attachmentH ...*AttachmentHandler) *Router {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(RequestID(), RecoverWithZap(log), LogWithZap(log))

	var attachments *AttachmentHandler
	if len(attachmentH) > 0 {
		attachments = attachmentH[0]
	}
	r := &Router{engine: engine, cfg: cfg, log: log, health: health, run: runH, project: projectH, thread: threadH, slide: slideH, repository: repositoryH, llm: llmH, polish: polishH, briefing: briefingH, gitCommit: gitCommitH, resources: resourceH, contextWindow: contextWindowH, attachment: attachments}
	if threadH != nil && threadH.svc.CommandStore() != nil {
		r.commands = service.NewCommandService(threadH.svc.CommandStore(), r.executeCommand)
	}
	engine.Use(r.projectHistoryGate())
	r.register()
	return r
}

func (r *Router) register() {
	v1 := r.engine.Group("/api/v1")
	v1.GET("/healthz", r.health.Healthz)
	if r.llm != nil {
		v1.GET("/llm/profiles", r.llm.Profiles)
		v1.GET("/settings/models", r.llm.Settings)
		v1.PUT("/settings/models", r.llm.SaveSettings)
		v1.POST("/settings/models/reload", r.llm.ReloadSettings)
	}
	v1.GET("/runtime/base.css", r.repository.RuntimeBaseCSS)
	v1.GET("/runtime/decorations.js", r.repository.RuntimeDecorationsJS)
	v1.GET("/runtime/fonts.css", r.repository.RuntimeAsset)
	v1.GET("/runtime/font-loader.js", r.repository.RuntimeAsset)
	v1.GET("/runtime/theme-bridge.js", r.repository.RuntimeAsset)
	v1.GET("/runtime/fonts/:name", r.repository.RuntimeFont)
	v1.GET("/runtime/theme-examples/:name", r.repository.RuntimeExample)
	v1.GET("/themes", r.repository.ListThemes)
	v1.GET("/themes/:id", r.repository.GetTheme)

	v1.GET("/themes/:id/css", r.repository.ThemeCSS)

	v1.GET("/components", r.repository.ListComponents)
	v1.GET("/components/:id", r.repository.GetComponent)

	v1.GET("/skills", r.repository.ListSkills)
	v1.GET("/skills/:id", r.repository.GetSkill)

	if r.resources != nil {
		v1.GET("/tags", r.resources.ListTags)
		v1.POST("/resources", r.resources.Register)
		v1.PATCH("/resources/:type/:id", r.resources.Patch)
		v1.DELETE("/resources/:type/:id", r.resources.Delete)
		v1.GET("/snippets", r.resources.ListSnippets)
		v1.GET("/snippets/:id", r.resources.GetSnippet)
		v1.POST("/snippets", r.resources.CreateSnippet)
		v1.PUT("/snippets/:id/content", r.resources.WriteSnippet)
	}

	// Project / Thread：API 契约入口，前端不需要绕过 HTTP 直接造数据。
	v1.GET("/projects", r.project.List)
	v1.POST("/projects", r.project.Create)
	v1.PATCH("/projects/:id", r.project.Patch)
	v1.GET("/projects/:id", r.project.Get)
	v1.DELETE("/projects/:id", r.project.Delete)
	v1.GET("/projects/:id/content", r.project.Content)
	v1.POST("/projects/:id/mutations", r.project.Mutate)
	v1.PATCH("/projects/:id/theme", r.project.SetTheme)
	if r.attachment != nil {
		v1.POST("/projects/:id/attachments", r.attachment.Upload)
		v1.GET("/projects/:id/attachments/:attachment_id", r.attachment.Get)
		v1.GET("/projects/:id/attachments/:attachment_id/content", r.attachment.Content)
	}
	v1.GET("/projects/:id/threads", r.thread.List)
	v1.GET("/projects/:id/thread-events", r.thread.Events)
	v1.POST("/projects/:id/threads", r.thread.Create)
	v1.PATCH("/threads/:id", r.thread.Patch)
	v1.GET("/threads/:id", r.thread.Get)
	v1.DELETE("/threads/:id", r.thread.Delete)
	v1.GET("/threads/:id/history", r.threadHistory)
	v1.GET("/threads/:id/events", r.threadJournalEvents)
	if r.commands != nil {
		v1.POST("/threads/:id/commands", r.createCommand)
		v1.GET("/commands/:id", r.getCommand)
		v1.POST("/commands/:id/cancel", r.cancelCommand)
	}
	if r.contextWindow != nil {
		v1.GET("/threads/:id/context-window", r.contextWindow.Get)
	}

	// Run：创建 / SSE 订阅 / HITL 输入 / 取消（40-api/rest-endpoints）。
	v1.POST("/threads/:id/runs", r.run.CreateRun)
	v1.GET("/threads/:id/active-run", r.run.GetActiveRunForThread)
	v1.GET("/runs/:id", r.run.GetRun)
	v1.GET("/runs/:id/screenshots/:screenshot_id", r.run.Screenshot)
	v1.POST("/runs/:id/input", r.run.Input)
	v1.POST("/runs/:id/plan-approval", r.run.PlanApproval)
	v1.POST("/runs/:id/command-permission", r.run.CommandPermission)
	v1.POST("/runs/:id/scope-expansion", r.run.ScopeExpansion)
	v1.POST("/runs/:id/steer", r.run.Steer)
	v1.POST("/runs/:id/resume", r.run.Resume)
	v1.DELETE("/runs/:id", r.run.Cancel)

	// Slide HTML preview uses stable identities.
	v1.GET("/slides/:id/render", r.slide.RenderSlide)

}

// Engine 暴露底层 gin 引擎供 server 启动使用。
func (r *Router) Engine() *gin.Engine {
	return r.engine
}
