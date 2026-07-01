package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
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
