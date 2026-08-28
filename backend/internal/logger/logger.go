package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// New 返回结构化 zap logger 及其 flush 清理函数。
func New(cfg *config.Config) (*zap.Logger, func(), error) {
	level, err := parseLevel(cfg)
	if err != nil {
		return nil, nil, err
	}
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.Sampling = nil
	zapCfg.DisableStacktrace = false
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	l, err := zapCfg.Build(zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, nil, err
	}
	restoreGlobals := zap.ReplaceGlobals(l)
	return l, func() {
		restoreGlobals()
		_ = l.Sync()
	}, nil
}

func parseLevel(cfg *config.Config) (zapcore.Level, error) {
	raw := "debug"
	if cfg != nil && strings.TrimSpace(cfg.LogLevel) != "" {
		raw = strings.TrimSpace(cfg.LogLevel)
	}
	var level zapcore.Level
	if err := level.Set(raw); err != nil {
		return zapcore.InfoLevel, fmt.Errorf("invalid LOG_LEVEL %q: %w", raw, err)
	}
	return level, nil
}
