package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
func initialize() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	root := flag.String("work-root", filepath.Join(home, ".dasi", "ppt"), "资源工作目录")
	seed := flag.String("seed-root", "../seed", "预置资源目录")
	replace := flag.Bool("replace-presets", false, "显式替换预置主题与组件（需先停止服务）")
	componentsOnly := flag.Bool("components-only", false, "仅替换组件，须配合 --replace-presets")
	flag.Parse()
	if *componentsOnly && !*replace {
		return fmt.Errorf("--components-only requires --replace-presets")
	}
	cfg := &config.Config{WorkRoot: *root, DBPath: filepath.Join(*root, "db", "ppt.db")}
	db, closeDB, err := sqlite.Open(cfg, zap.NewNop())
	if err != nil {
		return err
	}
	defer closeDB()
	st, err := persistence.NewStore(db, zap.NewNop())
	if err != nil {
		return err
	}
	svc := service.NewResourceService(service.WorkRoot(*root), st)
	if err = svc.RecoverDeletes(context.Background()); err != nil {
		return err
	}
	if *replace {
		err = svc.ReplacePresets(context.Background(), *seed, *componentsOnly)
	} else {
		err = svc.InitializeResources(context.Background(), *seed)
	}
	if err != nil {
		return err
	}
	fmt.Println("资源初始化完成：", *root)
	return nil
}
