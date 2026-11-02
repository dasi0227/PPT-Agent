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
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

// App 是装配完成的应用（由 wire 注入），持有运行所需依赖。
type App struct {
	server           *http.Server
	engine           *run.Engine
	runs             *service.RunService
	log              *zap.Logger
	stopBackground   func()
	shutdownExports  func(context.Context) error
	shutdownCommands func(context.Context) error
	shutdownRenderer func(context.Context) error
}

const shutdownTimeout = 8 * time.Second

var workRootFlag = flag.String("work-root", "", "使用指定的新工作目录；省略时使用默认目录")
var frontendDirFlag = flag.String("frontend-dir", "", "提供已构建的前端文件")
var desktopFlag = flag.Bool("desktop", false, "跟随桌面父进程退出，并输出就绪地址")
var printAddressFlag = flag.String("print-address", "", "打印指定配置文件的监听地址后退出")

func main() {
	flag.Parse()
	if err := serve(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() error {
	if *printAddressFlag != "" {
		cfg, err := config.LoadFile(*printAddressFlag)
		if err != nil {
			return err
		}
		fmt.Println(cfg.WorkAddr)
		return nil
	}
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()
	if *desktopFlag {
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin)
			// Keep signal handlers installed during cleanup. Electron also sends
			// TERM; unregistering here could make that signal abort persistence.
			cancel()
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
	// HTTP streams and requests have a service lifetime. Run execution owns its
	// own context so its durable pause is saved before the worker is canceled.
	requestCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	app.server.BaseContext = func(net.Listener) context.Context { return requestCtx }
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

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	return errors.Join(err, app.shutdown(shutdownCtx, cancelRequests))
}

func (a *App) shutdown(ctx context.Context, cancelRequests context.CancelFunc) error {
	started := time.Now()
	a.log.Info("server shutting down")
	cancelRequests() // SSE handlers must return before HTTP Shutdown can finish.
	a.stopBackground()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	stop := func(name string, close func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := close(ctx); err != nil {
				mu.Lock()
				failures = append(failures, fmt.Errorf("%s: %w", name, err))
				mu.Unlock()
			}
		}()
	}
	stop("pause runs", func(ctx context.Context) error { return a.engine.PauseAll(ctx, "server_shutdown") })
	stop("stop exports", a.shutdownExports)
	stop("stop commands", a.shutdownCommands)
	stop("stop HTTP", func(ctx context.Context) error {
		if err := a.server.Shutdown(ctx); err != nil {
			_ = a.server.Close()
			return err
		}
		return nil
	})
	wg.Wait()
	// Runs and exports no longer need the browser. Its close uses the remaining
	// budget instead of starting another independent shutdown timeout.
	if err := a.shutdownRenderer(ctx); err != nil {
		failures = append(failures, fmt.Errorf("stop renderer: %w", err))
	}
	err := errors.Join(failures...)
	if err != nil {
		a.log.Error("server shutdown incomplete", zap.Duration("elapsed", time.Since(started)), zap.Error(err))
	} else {
		a.log.Info("server stopped", zap.Duration("elapsed", time.Since(started)))
	}
	return err
}
