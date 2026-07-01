package config

import (
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 是经 Viper 装配后的强类型配置，集中于本包，禁止散落各处。
type Config struct {
	Env         string // development | production：控制 gin 模式与日志形态
	ListenAddr  string // PPT_LISTEN_ADDR
	WorkRoot    string // PPT_WORK_ROOT：work_dir 根
	DBPath      string // SQLite 文件路径；空则落在 WorkRoot 下
	DeepSeekKey string // 仅来自环境变量，MUST NOT 落库/落日志（DEV-RULES R13）
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetDefault("env", "development")
	v.SetDefault("listen_addr", "127.0.0.1:8787")
	v.SetDefault("work_root", "./.ppt-workspace")
	v.SetDefault("db_path", "")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv("env", "PPT_ENV")
	_ = v.BindEnv("listen_addr", "PPT_LISTEN_ADDR")
	_ = v.BindEnv("work_root", "PPT_WORK_ROOT")
	_ = v.BindEnv("db_path", "PPT_DB_PATH")
	_ = v.BindEnv("deepseek_key", "DEEPSEEK_API_KEY")

	cfg := &Config{
		Env:         v.GetString("env"),
		ListenAddr:  v.GetString("listen_addr"),
		WorkRoot:    v.GetString("work_root"),
		DBPath:      v.GetString("db_path"),
		DeepSeekKey: v.GetString("deepseek_key"),
	}
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.WorkRoot, "ppt.db")
	}
	return cfg, nil
}
