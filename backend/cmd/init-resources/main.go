package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/persistence"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
)

func main() {
	if err := initialize(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initialize 将预置资源写入工作目录并登记到数据库
func initialize() error {
	// seed-root 指定预置资源来源，写入目标固定为用户主目录下的 .dasi/ppt。
	seed := flag.String("seed-root", "seed", "预置资源目录")
	flag.Parse()
	work, err := config.UserWorkRoot()
	if err != nil {
		return err
	}
	// 初始化工具只需要固定工作目录和数据库路径，无需加载模型配置。
	cfg := &config.RuntimeConfig{WorkRoot: work, DBPath: filepath.Join(work, "db", "ppt.db")}
	if err := os.MkdirAll(cfg.WorkRoot, 0700); err != nil {
		return err
	}
	// 与 server 共用独占锁，防止初始化资源时服务仍在修改同一工作目录。
	lock, err := os.OpenFile(filepath.Join(cfg.WorkRoot, ".server.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	// syscall.Flock(文件描述符, 锁操作) 用于对文件进行锁操作：LOCH_SH 共享锁、LOCH_EX 排他锁、LOCH_NB 不等待上锁、LOCH_UN 释放锁
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("工作目录正在被使用，请先退出 PPT-Agent 服务: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	db, closeDB, err := sqlite.Open(cfg, zap.NewNop())
	if err != nil {
		return err
	}
	// defer 按注册的相反顺序执行：先关闭数据库，再解锁，最后关闭锁文件。
	defer closeDB()
	// 创建存储层时初始化或校验表结构，并恢复中断的项目历史操作；表结构来自 internal/store/sqlite/schema.sql。
	st, err := persistence.NewStore(db)
	if err != nil {
		return err
	}
	svc := service.NewResourceService(service.WorkRoot(cfg.WorkRoot), st)
	// 先完成上次中断的资源删除，再导入预置资源，避免文件与数据库状态冲突。
	if err = svc.RecoverDeletes(context.Background()); err != nil {
		return err
	}
	// 重复执行会恢复预置内容和元数据，同时保留启停选择及用户自行创建的资源。
	if err = svc.InitializeResources(context.Background(), *seed); err != nil {
		return err
	}
	fmt.Println("资源初始化完成：", cfg.WorkRoot)
	return nil
}
