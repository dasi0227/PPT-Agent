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
	provideLLMRegistry,
	provideLockManager,
	provideWorkRoot,
	provideEngine,
	provideHistoryWriter,
	provideRenderWorker,
	service.NewHealthService,
	provideProjectService,
	service.NewThreadService,
	service.NewRunService,
	service.NewPolishService,
	provideGitCommitService,
	provideSlideService,
	service.NewPPTMutationService,
	service.NewThemeService,
	service.NewComponentService,
	service.NewSkillService,
	httpapi.NewHealthHandler,
	httpapi.NewRunHandler,
	httpapi.NewPolishHandler,
	httpapi.NewGitCommitHandler,
	httpapi.NewLLMHandler,
	httpapi.NewProjectHandler,
	httpapi.NewThreadHandler,
	httpapi.NewSlideHandler,
	httpapi.NewRepositoryHandler,
	httpapi.NewRouter,
	engineFromRouter,
	provideHTTPServer,
	provideSeed,
	provideApp,
)

func initApp() (*App, func(), error) {
	panic(wire.Build(providerSet))
}
