package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/subosito/gotenv"
	"github.com/spf13/viper"
)

// Config 是经 Viper 装配后的强类型配置，集中于本包，禁止散落各处。
type Config struct {
	WorkAddr    string // WORK_ADDR
	WorkRoot    string // WORK_ROOT：全局工作根
	DBPath      string // SQLite 文件路径；固定落在 WorkRoot/db/ppt.db
	DeepSeekKey string // 仅来自环境变量，MUST NOT 落库/落日志（DEV-RULES R13）
	DeepSeekURL string // DEEPSEEK_BASE_URL；空走默认端点
	DeepSeekMdl string // DEEPSEEK_MODEL；默认 deepseek-chat
}

func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}
	v := viper.New()
	v.SetDefault("work_addr", "127.0.0.1:8787")
	v.SetDefault("work_root", defaultWorkRoot())

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv("work_addr", "WORK_ADDR")
	_ = v.BindEnv("work_root", "WORK_ROOT")
	_ = v.BindEnv("deepseek_key", "DEEPSEEK_API_KEY")
	_ = v.BindEnv("deepseek_url", "DEEPSEEK_BASE_URL")
	_ = v.BindEnv("deepseek_mdl", "DEEPSEEK_MODEL")

	cfg := &Config{
		WorkAddr:    v.GetString("work_addr"),
		WorkRoot:    v.GetString("work_root"),
		DeepSeekKey: v.GetString("deepseek_key"),
		DeepSeekURL: v.GetString("deepseek_url"),
		DeepSeekMdl: v.GetString("deepseek_mdl"),
	}
	cfg.DBPath = filepath.Join(cfg.WorkRoot, "db", "ppt.db")
	return cfg, nil
}

func defaultWorkRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".dasi", "ppt")
	}
	return filepath.Join(home, ".dasi", "ppt")
}

func loadDotEnv() error {
	if err := gotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
