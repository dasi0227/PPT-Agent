package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
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

func provideApp(server *http.Server, engine *run.Engine, log *zap.Logger, _ seedDone) *App {
	return &App{server: server, engine: engine, log: log}
}

// seedDone 是冷启动 seeding 完成的哨兵：provideApp 依赖它，保证服务启动前 seed 就绪（DS-SEED-001）。
type seedDone struct{}

// provideSeed 首启把 seed 资产载入 work_root 的 _assets/ 并 upsert SQLite（幂等）。
func provideSeed(cfg *config.Config, s store.Store, log *zap.Logger) (seedDone, error) {
	seeder := asset.NewSeeder(s, cfg.WorkRoot, nil, nil)
	if err := seeder.Seed(context.Background()); err != nil {
		return seedDone{}, err
	}
	log.Info("seed assets loaded", zap.String("work_root", cfg.WorkRoot))
	return seedDone{}, nil
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

// provideWorkRoot 从配置暴露全局 work_root（供 /repo 资产操作）。
func provideWorkRoot(cfg *config.Config) service.WorkRoot { return service.WorkRoot(cfg.WorkRoot) }

func provideAssetService(s store.Store, workRoot service.WorkRoot) *service.AssetService {
	return service.NewAssetService(s, string(workRoot))
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
