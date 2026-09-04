package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func provideHTTPServer(cfg *config.Config, engine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    cfg.WorkAddr,
		Handler: engine,
	}
}

func provideApp(server *http.Server, engine *run.Engine, log *zap.Logger) *App {
	return &App{server: server, engine: engine, log: log}
}

func engineFromRouter(r *httpapi.Router) *gin.Engine { return r.Engine() }

func provideLLMRegistry(cfg *config.Config) (*llm.Registry, error) {
	profiles := make([]llm.ProfileConfig, 0, len(cfg.LLM.Profiles))
	for _, profile := range cfg.LLM.Profiles {
		profiles = append(profiles, llm.ProfileConfig{
			Name: profile.Name, Provider: profile.Provider,
			URL: profile.URL, Model: profile.Model, Key: profile.Key,
		})
	}
	return llm.NewRegistry(cfg.LLM.Default, profiles)
}

func provideLockManager() *run.LockManager { return run.NewLockManager() }

func provideTranscriptStore() *contextengine.FSTranscriptStore {
	return contextengine.NewFSTranscriptStore()
}

func provideCalibrationStore() *contextengine.CalibrationStore {
	return contextengine.NewCalibrationStore()
}

func provideThreadService(s store.Store, transcripts *contextengine.FSTranscriptStore) *service.ThreadService {
	return service.NewThreadServiceWithTranscript(s, transcripts)
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

func provideProjectService(s store.Store, workRoot service.WorkRoot, locks *run.LockManager, themes *service.ThemeService) *service.ProjectService {
	return service.NewProjectServiceWithRepositories(s, workRoot, locks, themes)
}

func provideSlideService(s store.Store, themes *service.ThemeService) *service.SlideService {
	return service.NewSlideServiceWithThemes(s, themes)
}

func providePromptService(s store.Store) (*service.PromptService, error) {
	svc := service.NewPromptService(s)
	if os.Getenv("DASI_SEED_DEFAULT_PROMPTS") == "1" {
		if _, err := svc.SeedDefaults(context.Background()); err != nil {
			return nil, err
		}
	}
	return svc, nil
}

func provideEngine(rs run.Store, locks *run.LockManager, hw run.HistoryWriter, log *zap.Logger) (*run.Engine, error) {
	engine := run.NewEngine(rs, locks, hw, log)
	if err := engine.Initialize(context.Background()); err != nil {
		return nil, err
	}
	return engine, nil
}

// provideHistoryWriter 用底层 store 作为 ThreadLocator：Store 已实现 GetThread/GetProject（隐式接口）。
func provideHistoryWriter(s store.Store) run.HistoryWriter {
	return run.NewFSHistoryWriter(s)
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
