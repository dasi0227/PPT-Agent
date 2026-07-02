package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

func provideHTTPServer(cfg *config.Config, engine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: engine,
	}
}

func provideApp(server *http.Server, log *zap.Logger) *App {
	return &App{server: server, log: log}
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

func provideEngine(rs run.Store, locks *run.LockManager, log *zap.Logger) *run.Engine {
	return run.NewEngine(rs, locks, log)
}
