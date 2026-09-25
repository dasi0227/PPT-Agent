package main

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	presentationexport "github.com/dasi0227/PPT-Agent/backend/internal/export"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/shortcuts"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func provideHTTPServer(cfg *config.Config, engine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    cfg.WorkAddr,
		Handler: engine,
	}
}

func provideApp(server *http.Server, engine *run.Engine, runs *service.RunService, log *zap.Logger) *App {
	return &App{server: server, engine: engine, runs: runs, log: log}
}

func engineFromRouter(r *httpapi.Router) *gin.Engine { return r.Engine() }

func provideLLMRegistry(cfg *config.Config) (*llm.Registry, error) {
	return llm.NewConfiguredRegistry(cfg.LLMPath, cfg.LLM)
}

func provideRenameProvider(registry *llm.Registry) (llm.Provider, error) {
	return llm.SideProvider{Registry: registry, Purpose: "rename"}, nil
}

func provideLockManager() *run.LockManager { return run.NewLockManager() }

func provideTranscriptStore(s *sqlitestore.Store) *contextengine.JournalTranscriptStore {
	return contextengine.NewJournalTranscriptStore(s)
}

func provideCalibrationStore() *contextengine.CalibrationStore {
	return contextengine.NewCalibrationStore()
}

func provideThreadService(s store.Store, transcripts *contextengine.JournalTranscriptStore) *service.ThreadService {
	return service.NewThreadServiceWithTranscript(s, transcripts)
}

func provideThreadEventHub(s store.Store) *service.ThreadEventHub {
	return service.NewThreadEventHub(s)
}

func provideNamingService(s store.Store, provider llm.Provider, hub *service.ThreadEventHub, log *zap.Logger) (*service.NamingService, func()) {
	svc := service.NewNamingService(s, provider, hub, log)
	return svc, svc.Close
}

func provideRunService(s store.Store, engine *run.Engine, registry *llm.Registry, workRoot service.WorkRoot, renderer *workflow.NodeSlideRenderer, transcripts *contextengine.JournalTranscriptStore, calibration *contextengine.CalibrationStore, naming *service.NamingService) *service.RunService {
	return service.NewRunService(s, engine, registry, workRoot, renderer, transcripts, calibration).WithNaming(naming)
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

// provideWorkRoot 从配置暴露全局 work_root（供 /repo 资产操作）。
func provideWorkRoot(cfg *config.Config) service.WorkRoot { return service.WorkRoot(cfg.WorkRoot) }

func provideThemeService(s store.Store, workRoot service.WorkRoot) *service.ThemeService {
	return service.NewThemeService(workRoot, s)
}

func provideComponentService(s store.Store, workRoot service.WorkRoot) *service.ComponentService {
	return service.NewComponentService(workRoot, s)
}

func provideSkillService(s store.Store, workRoot service.WorkRoot) *service.SkillService {
	return service.NewSkillService(workRoot, s)
}

func provideProjectService(s store.Store, workRoot service.WorkRoot, locks *run.LockManager, themes *service.ThemeService, exports *presentationexport.Manager) *service.ProjectService {
	return service.NewProjectServiceWithRepositories(s, workRoot, locks, themes).WithExportManager(exports)
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

func provideRouter(cfg *config.Config, log *zap.Logger, health *httpapi.HealthHandler, runH *httpapi.RunHandler, projectH *httpapi.ProjectHandler, threadH *httpapi.ThreadHandler, slideH *httpapi.SlideHandler, repositoryH *httpapi.RepositoryHandler, llmH *httpapi.LLMHandler, polishH *httpapi.PolishHandler, briefingH *httpapi.BriefingHandler, gitCommitH *httpapi.GitCommitHandler, resourceH *httpapi.ResourceHandler, contextWindowH *httpapi.ContextWindowHandler, attachmentH *httpapi.AttachmentHandler, exportH *httpapi.ExportHandler, dbStore *sqlitestore.Store) (*httpapi.Router, error) {
	return httpapi.NewRouter(cfg, log, health, runH, projectH, threadH, slideH, repositoryH, llmH, polishH, briefingH, gitCommitH, resourceH, contextWindowH, attachmentH).WithExportHandler(exportH).WithShortcutSettings(shortcuts.NewService(dbStore)).WithProjectHistory()
}

func provideSlideService(s store.Store, themes *service.ThemeService) *service.SlideService {
	return service.NewSlideServiceWithThemes(s, themes)
}

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

func provideConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if *workRootFlag != "" {
		cfg.WorkRoot, err = filepath.Abs(*workRootFlag)
		if err != nil {
			return nil, err
		}
		cfg.DBPath = filepath.Join(cfg.WorkRoot, "db", "ppt.db")
	}
	return cfg, nil
}
