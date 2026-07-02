package service

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/demo"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// RunnerFactory 按 Run 元数据与入参构造一次执行的 runner。
// 抽出工厂便于测试注入替身，也为 M2+ 按 kind/scope 选择真实 agent 预留扩展点。
type RunnerFactory func(r model.Run, p model.CreateRunParams) run.Runner

// RunService 编排一次 Agent 执行：解析 thread→project 归属、构造 runner、委派 engine。
// M1 使用 demo runner；真实业务 agent 在 M2+ 接入。
type RunService struct {
	store   store.Store
	engine  *run.Engine
	factory RunnerFactory
}

// NewRunService 用默认 demo runner 工厂装配（生产路径）。
func NewRunService(s store.Store, engine *run.Engine, client llm.Client, log *zap.Logger) *RunService {
	factory := func(r model.Run, p model.CreateRunParams) run.Runner {
		return demo.New(client, r.ID, p.Scope, p.Mode, p.Instruction)
	}
	return &RunService{store: s, engine: engine, factory: factory}
}

// NewRunServiceWithFactory 允许注入自定义 runner 工厂（测试用）。
func NewRunServiceWithFactory(s store.Store, engine *run.Engine, factory RunnerFactory) *RunService {
	return &RunService{store: s, engine: engine, factory: factory}
}

// CreateRun 发起一次 Run：查 thread 得 project 归属（加锁单元），构造 runner 交给 engine。
func (svc *RunService) CreateRun(ctx context.Context, threadID string, p model.CreateRunParams) (model.Run, error) {
	th, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.Run{}, err
	}

	r := model.Run{
		ID:        uuid.NewString(),
		ThreadID:  th.ID,
		ProjectID: th.ProjectID,
		Kind:      p.Kind,
		Scope:     p.Scope,
		PageIndex: p.PageIndex,
		Mode:      p.Mode,
		Command:   p.Command,
	}

	runner := svc.factory(r, p)
	return svc.engine.Start(ctx, r, runner)
}

// InjectInput 转发到 engine（HITL 控制输入）。
func (svc *RunService) InjectInput(ctx context.Context, runID, content, replyTo string) error {
	return svc.engine.InjectInput(ctx, runID, content, replyTo)
}

// Cancel 取消 Run。
func (svc *RunService) Cancel(ctx context.Context, runID string) error {
	return svc.engine.Cancel(ctx, runID)
}

// Subscribe 订阅 SSE 事件流（支持 Last-Event-ID）。
func (svc *RunService) Subscribe(ctx context.Context, runID string, afterSeq int64) (<-chan model.Event, func(), error) {
	return svc.engine.Subscribe(ctx, runID, afterSeq)
}
