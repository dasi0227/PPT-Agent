package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	presentationexport "github.com/dasi0227/PPT-Agent/backend/internal/export"
	"github.com/dasi0227/PPT-Agent/backend/internal/fileopen"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/shortcuts"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// 本文件集中提供供 Wire 调用的对象创建函数；业务实现位于 internal 下的各个包。

// provideHTTPServer 将路由引擎接到 HTTP 服务上；实际监听由 main.go 中的 serve 启动。
func provideHTTPServer(cfg *config.RuntimeConfig, engine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    cfg.WorkAddr,
		Handler: engine,
	}
}

// provideApp 汇总主流程需要的对象，以及各模块的维护和关闭函数。
func provideApp(server *http.Server, engine *run.Engine, runs *service.RunService, log *zap.Logger, renderer *workflow.NodeSlideRenderer, exports *presentationexport.Manager, naming *service.NamingService, router *httpapi.Router, dbStore *sqlitestore.Store) *App {
	return &App{server: server, engine: engine, runs: runs, log: log,
		pruneSessionChanges: dbStore.PruneSessionChanges,
		stopBackground:      naming.Close, shutdownExports: exports.Shutdown, shutdownCommands: router.Shutdown, shutdownRenderer: renderer.Shutdown}
}

func engineFromRouter(r *httpapi.Router) *gin.Engine { return r.Engine() }

// provideLLMRegistry 根据模型配置创建注册表，供运行、命名等服务选择模型。
func provideLLMRegistry(cfg *config.RuntimeConfig) (*llm.Registry, error) {
	return llm.NewConfiguredRegistry(cfg.ConfigPath, cfg.LLM)
}

func provideRenameProvider(registry *llm.Registry) (llm.Provider, error) {
	return llm.SideProvider{Registry: registry, Purpose: "rename"}, nil
}

func provideLockManager() *run.LockManager { return run.NewLockManager() }

func provideContextStore(s *sqlitestore.Store) *contextengine.ContextStore {
	return contextengine.NewContextStore(s)
}

func provideThreadService(s store.Store) *service.ThreadService {
	return service.NewThreadService(s)
}

func provideThreadEventHub(s store.Store) *service.ThreadEventHub {
	return service.NewThreadEventHub(s)
}

func provideNamingService(s store.Store, provider llm.Provider, registry *llm.Registry, hub *service.ThreadEventHub, log *zap.Logger) (*service.NamingService, func()) {
	svc := service.NewNamingService(s, provider, registry, hub, log).WithDecisions(registry)
	return svc, svc.Close
}

// provideRunService 将运行引擎、模型、上下文、渲染器和命令执行器接入 Agent 运行服务。
func provideRunService(s store.Store, engine *run.Engine, registry *llm.Registry, workRoot service.WorkRoot, renderer *workflow.NodeSlideRenderer, transcripts *contextengine.ContextStore, naming *service.NamingService, cfg *config.RuntimeConfig) *service.RunService {
	commands := commandexec.NewRunner(cfg.Sandbox, cfg.ConfigPath, cfg.WorkRoot)
	return service.NewRunService(s, engine, registry, workRoot, renderer, transcripts).WithNaming(naming).WithCommandRunner(commands)
}

func provideThreadHandler(threads *service.ThreadService, naming *service.NamingService) *httpapi.ThreadHandler {
	return httpapi.NewThreadHandler(threads, naming)
}

func provideGitCommitService(s store.Store, registry *llm.Registry, locks *run.LockManager) (*service.GitCommitService, error) {
	svc := service.NewGitCommitService(s, registry, locks)
	if err := svc.Initialize(context.Background()); err != nil {
		return nil, err
	}
	return svc, nil
}

// provideWorkRoot 向资源服务提供固定的全局工作目录。
func provideWorkRoot(cfg *config.RuntimeConfig) service.WorkRoot {
	return service.WorkRoot(cfg.WorkRoot)
}

func provideThemeService(s store.Store, workRoot service.WorkRoot) *service.ThemeService {
	return service.NewThemeService(workRoot, s)
}

func provideComponentService(s store.Store, workRoot service.WorkRoot) *service.ComponentService {
	return service.NewComponentService(workRoot, s)
}

func provideSkillService(s store.Store, workRoot service.WorkRoot) *service.SkillService {
	return service.NewSkillService(workRoot, s)
}

func provideProjectService(s store.Store, workRoot service.WorkRoot, locks *run.LockManager, themes *service.ThemeService) *service.ProjectService {
	return service.NewProjectServiceWithRepositories(s, workRoot, locks, themes)
}

func provideAttachmentService(s store.Store, locks *run.LockManager) *service.AttachmentService {
	return service.NewAttachmentServiceWithLocks(s, locks)
}
func providePPTMutationService(s store.Store, locks *run.LockManager) *service.PPTMutationService {
	return service.NewPPTMutationServiceWithLocks(s, locks)
}

func provideExportManager(renderer *workflow.NodeSlideRenderer, workRoot service.WorkRoot, log *zap.Logger) (*presentationexport.Manager, func(), error) {
	manager := presentationexport.NewManager(renderer).WithLogger(log)
	if err := manager.CleanupWorkRoot(string(workRoot)); err != nil {
		return nil, nil, err
	}
	return manager, manager.Close, nil
}

// provideRouter 汇总各业务处理器，并接入推荐、导出、快捷键、文件打开设置和项目历史功能。
func provideRouter(cfg *config.RuntimeConfig, log *zap.Logger, health *httpapi.HealthHandler, runH *httpapi.RunHandler, projectH *httpapi.ProjectHandler, threadH *httpapi.ThreadHandler, slideH *httpapi.SlideHandler, repositoryH *httpapi.RepositoryHandler, llmH *httpapi.LLMHandler, polishH *httpapi.PolishHandler, briefingH *httpapi.BriefingHandler, gitCommitH *httpapi.GitCommitHandler, resourceH *httpapi.ResourceHandler, contextWindowH *httpapi.ContextWindowHandler, attachmentH *httpapi.AttachmentHandler, exportH *httpapi.ExportHandler, dbStore *sqlitestore.Store, registry *llm.Registry, hub *service.ThreadEventHub) (*httpapi.Router, error) {
	return httpapi.NewRouter(cfg, log, health, runH, projectH, threadH, slideH, repositoryH, llmH, polishH, briefingH, gitCommitH, resourceH, contextWindowH, attachmentH).WithSuggestions(service.NewSuggestService(dbStore, registry, hub)).WithExportHandler(exportH).WithShortcutSettings(shortcuts.NewService(dbStore)).WithFileSettings(fileopen.NewService(dbStore, cfg.WorkRoot)).WithProjectHistory()
}

func provideSlideService(s store.Store, themes *service.ThemeService, renderer *workflow.NodeSlideRenderer) *service.SlideService {
	return service.NewSlideService(s, themes, renderer)
}

// provideResourceService 先恢复中断的资源删除，再开放资源管理；导入预置资源由 init-resources 单独执行。
func provideResourceService(s store.Store, workRoot service.WorkRoot) (*service.ResourceService, error) {
	svc := service.NewResourceService(workRoot, s)
	if err := svc.RecoverDeletes(context.Background()); err != nil {
		return nil, err
	}
	return svc, nil
}

func provideEngine(rs run.Store, locks *run.LockManager, log *zap.Logger) (*run.Engine, error) {
	engine := run.NewEngine(rs, locks, log)
	if err := engine.Initialize(context.Background()); err != nil {
		return nil, err
	}
	return engine, nil
}

// provideRenderWorker 检查渲染 Worker 是否可用，并返回退出或装配失败时使用的清理函数。
func provideRenderWorker() (*workflow.NodeSlideRenderer, func(), error) {
	renderer := workflow.NewNodeSlideRenderer(workflow.NodeRendererConfig{})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := renderer.Health(ctx); err != nil {
		_ = renderer.Close()
		return nil, nil, err
	}
	return renderer, func() { _ = renderer.Close() }, nil
}

// provideConfig 加载配置，并生成固定工作目录、数据库路径及监听地址。
func provideConfig() (*config.RuntimeConfig, error) {
	return config.LoadConfig()
}
