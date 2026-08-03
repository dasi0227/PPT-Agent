package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

const defaultDeepSeekTimeoutSeconds = 180

// Config 是经 Viper 装配后的强类型配置，集中于本包，禁止散落各处。
type Config struct {
	WorkAddr         string        // WORK_ADDR
	WorkRoot         string        // WORK_ROOT：全局工作根
	DBPath           string        // SQLite 文件路径；固定落在 WorkRoot/db/ppt.db
	DeepSeekKey      string        // 仅来自环境变量，MUST NOT 落库/落日志（DEV-RULES R13）
	DeepSeekURL      string        // DEEPSEEK_BASE_URL；空走默认端点
	DeepSeekMdl      string        // DEEPSEEK_MODEL；默认 deepseek-chat
	DeepSeekTimeout  time.Duration // DEEPSEEK_TIMEOUT_SECONDS：http.Client.Timeout（整体超时），默认 180s
	LLMVision        bool          // LLM_VISION：当前 OpenAI-compatible provider 是否支持图片输入
	LLMMaxImageBytes int           // LLM_MAX_IMAGE_BYTES
}

func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}
	v := viper.New()
	v.SetDefault("work_addr", "127.0.0.1:8787")
	v.SetDefault("work_root", defaultWorkRoot())
	v.SetDefault("deepseek_timeout", defaultDeepSeekTimeoutSeconds)
	v.SetDefault("llm_max_image_bytes", 4*1024*1024)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv("work_addr", "WORK_ADDR")
	_ = v.BindEnv("work_root", "WORK_ROOT")
	_ = v.BindEnv("deepseek_key", "DEEPSEEK_API_KEY")
	_ = v.BindEnv("deepseek_url", "DEEPSEEK_BASE_URL")
	_ = v.BindEnv("deepseek_mdl", "DEEPSEEK_MODEL")
	_ = v.BindEnv("deepseek_timeout", "DEEPSEEK_TIMEOUT_SECONDS")
	_ = v.BindEnv("llm_vision", "LLM_VISION")
	_ = v.BindEnv("llm_max_image_bytes", "LLM_MAX_IMAGE_BYTES")

	// 单位=秒的整数；<=0 视为无效并回落到默认，避免整体超时被误设为 0 导致立即失败。
	timeoutSec := v.GetInt("deepseek_timeout")
	if timeoutSec <= 0 {
		timeoutSec = defaultDeepSeekTimeoutSeconds
	}

	cfg := &Config{
		WorkAddr:         v.GetString("work_addr"),
		WorkRoot:         v.GetString("work_root"),
		DeepSeekKey:      v.GetString("deepseek_key"),
		DeepSeekURL:      v.GetString("deepseek_url"),
		DeepSeekMdl:      v.GetString("deepseek_mdl"),
		DeepSeekTimeout:  time.Duration(timeoutSec) * time.Second,
		LLMVision:        v.GetBool("llm_vision"),
		LLMMaxImageBytes: v.GetInt("llm_max_image_bytes"),
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
