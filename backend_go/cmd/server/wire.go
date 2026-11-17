//go:build wireinject
// +build wireinject

package main

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/google/wire"

	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/logger"
	"github.com/dasi0227/PPT-Agent/backend/internal/persistence"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// 本文件仅在 Wire 生成代码时通过 wireinject 构建标签启用，正常构建使用 wire_gen.go。

// providerSet 列出对象创建函数，Wire 根据参数和返回类型推导创建顺序，生成依赖装配代码。
var providerSet = wire.NewSet(
	logger.New,
	sqlitestore.Open,
	persistence.NewStore,
	// 同一个 SQLite Store 分别满足业务存储接口和运行引擎的存储接口。
	wire.Bind(new(store.Store), new(*sqlitestore.Store)),
	wire.Bind(new(run.Store), new(*sqlitestore.Store)),
	provideLLMRegistry,
	provideRenameProvider,
	provideThreadEventHub,
	provideNamingService,
	provideLockManager,
	provideContextStore,
	provideWorkRoot,
	provideEngine,
	provideRenderWorker,
	provideExportManager,
	service.NewHealthService,
	provideProjectService,
	provideThreadService,
	provideAttachmentService,
	service.NewExportService,
	provideRunService,
	service.NewContextWindowService,
	service.NewPolishService,
	service.NewHandoffService,
	provideGitCommitService,
	provideSlideService,
	providePPTMutationService,
	provideThemeService,
	provideComponentService,
	provideSkillService,
	provideResourceService,
	httpapi.NewHealthHandler,
	httpapi.NewRunHandler,
	httpapi.NewAttachmentHandler,
	httpapi.NewExportHandler,
	httpapi.NewPolishHandler,
	httpapi.NewBriefingHandler,
	httpapi.NewGitCommitHandler,
	httpapi.NewLLMHandler,
	httpapi.NewProjectHandler,
	provideThreadHandler,
	httpapi.NewContextWindowHandler,
	httpapi.NewSlideHandler,
	httpapi.NewRepositoryHandler,
	httpapi.NewResourceHandler,
	provideRouter,
	engineFromRouter,
	provideHTTPServer,
	provideApp,
)

// initApp 根据配置装配应用，并返回失败或退出时释放资源的 cleanup 函数。
func initApp(cfg *config.RuntimeConfig) (*App, func(), error) {
	panic(wire.Build(providerSet))
}
