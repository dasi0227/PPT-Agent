// Package httpapi 是 HTTP 层：gin handler、路由、中间件。gin.Context 仅限本层（ARCH-BACKEND-006）。
package httpapi

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// Router 持有 gin 引擎与各 handler 依赖，负责路由注册。
type Router struct {
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
	prompt        *PromptHandler
	contextWindow *ContextWindowHandler
	attachment    *AttachmentHandler
}

func NewRouter(cfg *config.Config, log *zap.Logger, health *HealthHandler, runH *RunHandler, projectH *ProjectHandler, threadH *ThreadHandler, slideH *SlideHandler, repositoryH *RepositoryHandler, llmH *LLMHandler, polishH *PolishHandler, briefingH *BriefingHandler, gitCommitH *GitCommitHandler, promptH *PromptHandler, contextWindowH *ContextWindowHandler, attachmentH ...*AttachmentHandler) *Router {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(RequestID(), RecoverWithZap(log), LogWithZap(log))

	var attachments *AttachmentHandler
	if len(attachmentH) > 0 {
		attachments = attachmentH[0]
	}
	r := &Router{engine: engine, cfg: cfg, log: log, health: health, run: runH, project: projectH, thread: threadH, slide: slideH, repository: repositoryH, llm: llmH, polish: polishH, briefing: briefingH, gitCommit: gitCommitH, prompt: promptH, contextWindow: contextWindowH, attachment: attachments}
	r.register()
	return r
}

func (r *Router) register() {
	v1 := r.engine.Group("/api/v1")
	v1.GET("/healthz", r.health.Healthz)
	if r.llm != nil {
		v1.GET("/llm/profiles", r.llm.Profiles)
	}
	v1.GET("/runtime/base.css", r.repository.RuntimeBaseCSS)
	v1.GET("/themes", r.repository.ListThemes)
	v1.GET("/themes/:id", r.repository.GetTheme)
	v1.PATCH("/themes/:id", r.repository.PatchTheme)
	v1.GET("/themes/:id/css", r.repository.ThemeCSS)
	v1.DELETE("/themes/:id", r.repository.DeleteTheme)
	v1.GET("/components", r.repository.ListComponents)
	v1.GET("/components/:id", r.repository.GetComponent)
	v1.PATCH("/components/:id", r.repository.PatchComponent)
	v1.DELETE("/components/:id", r.repository.DeleteComponent)
	v1.GET("/skills", r.repository.ListSkills)
	v1.GET("/skills/:id", r.repository.GetSkill)
	v1.PATCH("/skills/:id", r.repository.PatchSkill)
	v1.DELETE("/skills/:id", r.repository.DeleteSkill)
	if r.prompt != nil {
		v1.GET("/prompts", r.prompt.List)
		v1.GET("/prompts/:id", r.prompt.Get)
		v1.POST("/prompts", r.prompt.Create)
		v1.PUT("/prompts/:id", r.prompt.Update)
		v1.PATCH("/prompts/:id", r.prompt.Patch)
		v1.DELETE("/prompts/:id", r.prompt.Delete)
	}

	// Project / Thread：API 契约入口，前端不需要绕过 HTTP 直接造数据。
	v1.GET("/projects", r.project.List)
	v1.POST("/projects", r.project.Create)
	v1.PATCH("/projects/:id", r.project.Patch)
	v1.GET("/projects/:id", r.project.Get)
	v1.DELETE("/projects/:id", r.project.Delete)
	if r.polish != nil {
		v1.POST("/projects/:id/polish", r.polish.Polish)
	}
	if r.briefing != nil {
		v1.POST("/projects/:id/kickoff", r.briefing.Kickoff)
		v1.POST("/projects/:id/handoff", r.briefing.Handoff)
	}
	v1.GET("/projects/:id/content", r.project.Content)
	v1.POST("/projects/:id/mutations", r.project.Mutate)
	v1.PATCH("/projects/:id/theme", r.project.SetTheme)
	if r.attachment != nil {
		v1.POST("/projects/:id/attachments", r.attachment.Upload)
		v1.GET("/projects/:id/attachments/:attachment_id", r.attachment.Get)
		v1.GET("/projects/:id/attachments/:attachment_id/content", r.attachment.Content)
	}
	if r.gitCommit != nil {
		v1.POST("/projects/:id/git-commits", r.gitCommit.Create)
		v1.GET("/git-commits/:id", r.gitCommit.Get)
		v1.GET("/git-commits/:id/events", r.gitCommit.Events)
	}
	v1.GET("/projects/:id/threads", r.thread.List)
	v1.POST("/projects/:id/threads", r.thread.Create)
	v1.PATCH("/threads/:id", r.thread.Patch)
	v1.GET("/threads/:id", r.thread.Get)
	v1.DELETE("/threads/:id", r.thread.Delete)
	v1.GET("/threads/:id/history", r.thread.History)
	if r.contextWindow != nil {
		v1.GET("/threads/:id/context-window", r.contextWindow.Get)
		v1.POST("/threads/:id/compact", r.contextWindow.Compact)
	}

	// Run：创建 / SSE 订阅 / HITL 输入 / 取消（40-api/rest-endpoints）。
	v1.POST("/threads/:id/runs", r.run.CreateRun)
	v1.GET("/threads/:id/active-run", r.run.GetActiveRunForThread)
	v1.GET("/runs/:id", r.run.GetRun)
	v1.GET("/runs/:id/events", r.run.Events)
	v1.GET("/runs/:id/screenshots/:screenshot_id", r.run.Screenshot)
	v1.POST("/runs/:id/input", r.run.Input)
	v1.POST("/runs/:id/plan-approval", r.run.PlanApproval)
	v1.POST("/runs/:id/command-permission", r.run.CommandPermission)
	v1.POST("/runs/:id/scope-expansion", r.run.ScopeExpansion)
	v1.POST("/runs/:id/steer", r.run.Steer)
	v1.POST("/runs/:id/resume", r.run.Resume)
	v1.DELETE("/runs/:id", r.run.Cancel)

	// Slide HTML preview and version operations use stable identities.
	v1.GET("/slides/:id/render", r.slide.RenderSlide)
	v1.GET("/slides/:id/versions", r.slide.ListVersions)
	v1.POST("/slides/:id/rollback", r.slide.Rollback)

}

// Engine 暴露底层 gin 引擎供 server 启动使用。
func (r *Router) Engine() *gin.Engine {
	return r.engine
}
