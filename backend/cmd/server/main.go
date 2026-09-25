package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

// App 是装配完成的应用（由 wire 注入），持有运行所需依赖。
type App struct {
	server *http.Server
	engine *run.Engine
	runs   *service.RunService
	log    *zap.Logger
}

var workRootFlag = flag.String("work-root", "", "使用指定的新工作目录；省略时使用默认目录")

func main() {
	flag.Parse()
	app, cleanup, err := initApp()
	if err != nil {
		panic(err)
	}
	defer cleanup()
	recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), 30*time.Second)
	if err := app.runs.RecoverAllProjectMutations(recoveryCtx); err != nil {
		cancelRecovery()
		app.log.Error("recovering project mutations failed", zap.Error(err))
		return
	}
	cancelRecovery()

	go func() {
		app.log.Info("server starting", zap.String("addr", app.server.Addr))
		if err := app.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.log.Info("server shutting down")
	pauseCtx, cancelPause := context.WithTimeout(context.Background(), 5*time.Second)
	if err := app.engine.PauseAll(pauseCtx, "server_shutdown"); err != nil {
		app.log.Error("pausing active runs failed", zap.Error(err))
	}
	cancelPause()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := app.server.Shutdown(shutdownCtx); err != nil {
		app.log.Error("graceful shutdown failed", zap.Error(err))
	}
}
