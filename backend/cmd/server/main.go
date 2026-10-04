package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
var frontendDirFlag = flag.String("frontend-dir", "", "提供已构建的前端文件")
var desktopFlag = flag.Bool("desktop", false, "跟随桌面父进程退出，并输出就绪地址")

func main() {
	flag.Parse()
	if err := serve(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if *desktopFlag {
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin)
			stop()
		}()
	}
	cfg, err := provideConfig()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.WorkRoot, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(cfg.WorkRoot, ".server.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("工作目录正在被另一个 PPT-Agent 服务使用: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	// Reserve the address before recovery or any database changes. A development
	// server using this address must be stopped, never silently adopted or killed.
	listener, err := net.Listen("tcp", cfg.WorkAddr)
	if err != nil {
		return fmt.Errorf("无法监听 %s，请先退出占用端口的开发服务: %w", cfg.WorkAddr, err)
	}
	defer listener.Close()
	app, cleanup, err := initApp(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if *frontendDirFlag != "" {
		app.server.Handler, err = withFrontend(app.server.Handler, *frontendDirFlag)
		if err != nil {
			return err
		}
	}
	recoveryCtx, cancelRecovery := context.WithTimeout(ctx, 30*time.Second)
	if err := app.runs.RecoverAllProjectMutations(recoveryCtx); err != nil {
		cancelRecovery()
		return fmt.Errorf("recovering project mutations: %w", err)
	}
	cancelRecovery()

	serverError := make(chan error, 1)
	go func() {
		app.log.Info("server starting", zap.String("addr", app.server.Addr))
		serverError <- app.server.Serve(listener)
	}()
	if *desktopFlag && ctx.Err() == nil {
		fmt.Printf("PPT_AGENT_READY http://%s\n", listener.Addr())
	}
	select {
	case <-ctx.Done():
	case err = <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}

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
	return err
}
