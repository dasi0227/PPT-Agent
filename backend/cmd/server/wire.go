//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/logger"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// providerSet 声明全部 provider；wire 在编译期据此生成装配代码。
var providerSet = wire.NewSet(
	config.Load,
	logger.New,
	sqlitestore.Open,
	sqlitestore.NewStore,
	wire.Bind(new(store.Store), new(*sqlitestore.Store)),
	wire.Bind(new(run.Store), new(*sqlitestore.Store)),
	provideLLMClient,
	provideLockManager,
	provideWorkRoot,
	provideEngine,
	service.NewHealthService,
	service.NewRunService,
	service.NewSlideService,
	httpapi.NewHealthHandler,
	httpapi.NewRunHandler,
	httpapi.NewSlideHandler,
	httpapi.NewRouter,
	engineFromRouter,
	provideHTTPServer,
	provideSeed,
	provideApp,
)

func initApp() (*App, func(), error) {
	panic(wire.Build(providerSet))
}
