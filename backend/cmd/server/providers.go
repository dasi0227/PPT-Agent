package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func provideHTTPServer(cfg *config.Config, engine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    cfg.WorkAddr,
		Handler: engine,
	}
}

func provideApp(server *http.Server, log *zap.Logger, _ seedDone) *App {
	return &App{server: server, log: log}
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

// provideLLMClient 从配置装配 DeepSeek 客户端（Key 仅来自 env，不落日志 ARCH-LLM-003）。
func provideLLMClient(cfg *config.Config) llm.Client {
	return llm.NewDeepSeek(llm.DeepSeekConfig{
		APIKey:  cfg.DeepSeekKey,
		BaseURL: cfg.DeepSeekURL,
		Model:   cfg.DeepSeekMdl,
	})
}

func provideLockManager() *run.LockManager { return run.NewLockManager() }

// provideWorkRoot 从配置暴露全局 work_root（供 /repo 资产操作）。
func provideWorkRoot(cfg *config.Config) service.WorkRoot { return service.WorkRoot(cfg.WorkRoot) }

func provideAssetService(s store.Store, workRoot service.WorkRoot) *service.AssetService {
	return service.NewAssetService(s, string(workRoot))
}

func provideEngine(rs run.Store, locks *run.LockManager, log *zap.Logger) *run.Engine {
	return run.NewEngine(rs, locks, nil, log)
}
