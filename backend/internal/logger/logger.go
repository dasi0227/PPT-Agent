package logger

import (
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// New 返回结构化 zap logger 及其 flush 清理函数。
func New(cfg *config.Config) (*zap.Logger, func(), error) {
	var (
		l   *zap.Logger
		err error
	)
	if cfg.Env == "production" {
		l, err = zap.NewProduction()
	} else {
		l, err = zap.NewDevelopment()
	}
	if err != nil {
		return nil, nil, err
	}
	return l, func() { _ = l.Sync() }, nil
}
