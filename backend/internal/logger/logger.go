package logger

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// New 返回结构化 zap logger 及其 flush 清理函数。
func New(cfg *config.RuntimeConfig) (*zap.Logger, func(), error) {
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

func parseLevel(cfg *config.RuntimeConfig) (zapcore.Level, error) {
	raw := strings.TrimSpace(cfg.App.LogLevel)
	var level zapcore.Level
	if raw == "" {
		return zapcore.InfoLevel, errors.New("app.log_level is required")
	}
	if err := level.Set(raw); err != nil {
		return zapcore.InfoLevel, fmt.Errorf("invalid app.log_level %q: %w", raw, err)
	}
	return level, nil
}
