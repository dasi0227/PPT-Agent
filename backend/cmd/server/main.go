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
	server              *http.Server
	engine              *run.Engine
	runs                *service.RunService
	log                 *zap.Logger
	stopBackground      func()
	shutdownExports     func(context.Context) error
	shutdownCommands    func(context.Context) error
	shutdownRenderer    func(context.Context) error
	pruneSessionChanges func(context.Context) error
}

const shutdownTimeout = 8 * time.Second

var frontendDirFlag = flag.String("frontend-dir", "", "提供已构建的前端文件")
var desktopFlag = flag.Bool("desktop", false, "跟随桌面父进程退出，并输出就绪地址")
var printAddressFlag = flag.String("print-address", "", "打印指定配置文件的监听地址后退出")

// main 是后端服务的程序入口，解析命令行参数，并统一处理启动和运行错误。
func main() {
	flag.Parse()
	if err := serve(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// serve 管理服务的完整生命周期：加载配置、独占工作目录、组装依赖、恢复状态、监听请求和关闭。
func serve() error {
	// 开发脚本和前端代理只需读取监听地址时，直接退出，不打开数据库或启动服务。
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
		// 桌面父进程退出并关闭标准输入后，触发与退出信号相同的收尾流程。
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin)
			// 收尾期间仍保留信号处理；Electron 也会发送 TERM，不能让它中断状态保存。
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
	// 同一工作目录只允许一个服务或资源初始化进程使用，避免并发修改数据。
	lock, err := os.OpenFile(filepath.Join(cfg.WorkRoot, ".server.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("工作目录正在被另一个 PPT-Agent 服务使用: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	// 先占用端口，再恢复状态或修改数据库；端口冲突时要求先退出已有服务。
	listener, err := net.Listen("tcp", cfg.WorkAddr)
	if err != nil {
		return fmt.Errorf("无法监听 %s，请先退出占用端口的开发服务: %w", cfg.WorkAddr, err)
	}
	defer listener.Close()
	// initApp 的实际实现位于 wire_gen.go，按依赖关系创建数据库、服务、路由等对象。
	app, cleanup, err := initApp(cfg)
	if err != nil {
		return err
	}
	// 最终关闭数据库等底层资源；正常退出时先由 app.shutdown 保存运行状态。
	defer cleanup()
	// HTTP 请求与 Agent 执行使用独立的取消上下文，避免断开请求时提前中断运行状态保存。
	requestCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	app.server.BaseContext = func(net.Listener) context.Context { return requestCtx }
	if *frontendDirFlag != "" {
		app.server.Handler, err = withFrontend(app.server.Handler, *frontendDirFlag)
		if err != nil {
			return err
		}
	}
	// 接收请求前，先恢复上次中断的项目文件变更，避免用户读到未完成的修改。
	recoveryCtx, cancelRecovery := context.WithTimeout(ctx, 30*time.Second)
	if err := app.runs.RecoverAllProjectMutations(recoveryCtx); err != nil {
		cancelRecovery()
		return fmt.Errorf("recovering project mutations: %w", err)
	}
	cancelRecovery()
	stopSessionMaintenance := startSessionMaintenance(ctx, app.pruneSessionChanges, app.log)
	defer stopSessionMaintenance()

	// 后台处理 HTTP 请求，主流程等待退出信号或监听错误。
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

	// 退出信号已取消 ctx，因此另建上下文，为保存状态和释放资源提供统一的收尾时间。
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	stopSessionMaintenance()
	return errors.Join(err, app.shutdown(shutdownCtx, cancelRequests))
}

// shutdown 在关闭底层资源前保存运行的暂停状态，并停止后台任务、导出、命令及 HTTP 服务。
func (a *App) shutdown(ctx context.Context, cancelRequests context.CancelFunc) error {
	started := time.Now()
	a.log.Info("server shutting down")
	cancelRequests() // 先结束 SSE 等长连接，否则 HTTP Shutdown 会一直等待请求返回。
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
	// 各模块并行停止，共用上面的收尾期限；PauseAll 负责持久化暂停状态。
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
	// 等运行和导出停止后再关闭渲染器，仍使用同一个收尾期限。
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
