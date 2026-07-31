package run

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const lockTimeout = 30 * time.Second

// active 是一个正在执行的 Run 的运行时句柄。
type active struct {
	run    model.Run
	bus    *Bus
	queue  *InputQueue
	cancel context.CancelFunc
}

// Engine 管理 Run 生命周期：状态机、事件扇出、HITL、取消、每 project 锁。
type Engine struct {
	store Store
	locks *LockManager
	hw    HistoryWriter
	log   *zap.Logger

	mu      sync.Mutex
	actives map[string]*active
}

func NewEngine(store Store, locks *LockManager, hw HistoryWriter, log *zap.Logger) *Engine {
	return &Engine{store: store, locks: locks, hw: hw, log: log, actives: map[string]*active{}}
}

// Start 创建 Run（pending）并异步执行 runner。返回创建后的 Run 元数据。
// 每 project 锁在后台 goroutine 内获取：同 project 串行、跨 project 并行（ARCH-RUN-LOCK）。
func (e *Engine) Start(ctx context.Context, r model.Run, runner Runner) (model.Run, error) {
	return e.StartWithContext(ctx, r, runner, nil)
}

// StartWithContext atomically establishes the Run row and its auditable ContextManifest
// before any runner code executes.
func (e *Engine) StartWithContext(ctx context.Context, r model.Run, runner Runner, manifest *model.RunContext) (model.Run, error) {
	now := time.Now().Unix()
	r.Status = model.RunPending
	r.CreatedAt = now
	r.UpdatedAt = now
	if err := e.store.CreateRun(ctx, r); err != nil {
		return model.Run{}, err
	}
	if manifest != nil {
		manifest.RunID = r.ID
		if manifest.CreatedAt == 0 {
			manifest.CreatedAt = now
		}
		contextStore, ok := e.store.(interface {
			SaveRunContext(context.Context, model.RunContext) error
		})
		if !ok {
			_ = e.store.SetRunStatus(ctx, r.ID, model.RunFailed)
			return model.Run{}, ErrContextStoreUnavailable
		}
		if err := contextStore.SaveRunContext(ctx, *manifest); err != nil {
			_ = e.store.SetRunStatus(ctx, r.ID, model.RunFailed)
			return model.Run{}, err
		}
	}

	bus := NewBus(r.ID, r.ThreadID, e.store, e.hw)
	queue := NewInputQueue()
	runCtx, cancel := context.WithCancel(context.Background())

	a := &active{run: r, bus: bus, queue: queue, cancel: cancel}
	e.mu.Lock()
	e.actives[r.ID] = a
	e.mu.Unlock()

	go e.execute(runCtx, a, runner)
	return r, nil
}

func (e *Engine) execute(ctx context.Context, a *active, runner Runner) {
	defer func() {
		a.bus.Close()
		e.mu.Lock()
		delete(e.actives, a.run.ID)
		e.mu.Unlock()
	}()

	// 获取 project 锁（同 project 串行）。锁超时 → Run 转 failed（ARCH-RUN-LOCK-005）。
	release, err := e.locks.Acquire(ctx, a.run.ProjectID, lockTimeout)
	if err != nil {
		e.setStatus(ctx, a.run.ID, model.RunFailed)
		_ = a.bus.Emit(ctx, model.EventError, harness.ErrorPayload{Code: "LOCK_TIMEOUT", Message: err.Error()})
		return
	}
	defer release()

	e.setStatus(ctx, a.run.ID, model.RunRunning)

	em := &harnessEmitter{ctx: ctx, bus: a.bus}
	cp := &harnessCheckpoint{queue: a.queue}
	prompter := &checkpoint{engine: e, runID: a.run.ID, bus: a.bus, queue: a.queue}
	outcome := runner.Run(ctx, em, cp, prompter)

	e.finish(ctx, a, outcome)
}

// finish 依据 harness outcome 发唯一终态事件并落状态（API-SSE-002）。
func (e *Engine) finish(ctx context.Context, a *active, outcome harness.Outcome) {
	switch outcome.Status {
	case harness.OutcomeFinished:
		e.setStatus(ctx, a.run.ID, model.RunDone)
		// 结构化交付优先（V2-M5 Stage 5）；缺省回退 {summary}（兼容 edit/outline/command）。
		result := outcome.Result
		if result == nil {
			result = map[string]any{"summary": outcome.Summary}
		}
		_ = a.bus.Emit(ctx, model.EventDone, harness.DonePayload{Result: result})
	case harness.OutcomeCanceled:
		e.setStatus(ctx, a.run.ID, model.RunCanceled)
		_ = a.bus.Emit(ctx, model.EventError, harness.ErrorPayload{Code: harness.CodeCanceled, Message: "run canceled"})
	default:
		e.setStatus(ctx, a.run.ID, model.RunFailed)
		_ = a.bus.Emit(ctx, model.EventError, harness.ErrorPayload{Code: outcome.Code, Message: outcome.Message})
	}
}

func (e *Engine) setStatus(ctx context.Context, id string, status model.RunStatus) {
	if err := e.store.SetRunStatus(ctx, id, status); err != nil {
		e.log.Warn("set run status failed", zap.String("run_id", id), zap.Error(err))
	}
}

func (e *Engine) lookup(id string) (*active, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	a, ok := e.actives[id]
	return a, ok
}

// InjectInput 注入控制输入（HITL）。仅 running/waiting 接受，否则 ErrRunNotRunning（API-RUN-001）。
// 带 replyTo 时必须匹配未应答 needs_input，否则 ErrReplyMismatch（API-RUN-003）。
func (e *Engine) InjectInput(ctx context.Context, id, content, replyTo string) error {
	a, ok := e.lookup(id)
	if !ok {
		// 已结束的 Run 不在 actives 中：查库确认终态 → 409。
		r, err := e.store.GetRun(ctx, id)
		if err != nil {
			return ErrRunNotFound
		}
		if r.Status.Terminal() {
			return ErrRunNotRunning
		}
		return ErrRunNotRunning
	}
	if replyTo != "" {
		if !a.queue.Reply(replyTo, content) {
			return ErrReplyMismatch
		}
		return nil
	}
	a.queue.Enqueue(content)
	return nil
}

// Cancel 取消 Run：停止后续 LLM 调用，保留已落盘产物（ARCH-RUN-004 / API-RUN-005）。
func (e *Engine) Cancel(ctx context.Context, id string) error {
	a, ok := e.lookup(id)
	if !ok {
		if _, err := e.store.GetRun(ctx, id); err != nil {
			return ErrRunNotFound
		}
		return nil // 已终止，幂等
	}
	a.cancel()
	return nil
}

// Subscribe 订阅 Run 事件流，支持 Last-Event-ID 续传（afterSeq）。
// 先补发 store 中 seq>afterSeq 的历史事件，再接入实时流，无重复无丢失（ARCH-RUN-005 / API-SSE-003）。
func (e *Engine) Subscribe(ctx context.Context, id string, afterSeq int64) (<-chan model.Event, func(), error) {
	if _, err := e.store.GetRun(ctx, id); err != nil {
		return nil, nil, ErrRunNotFound
	}

	a, live := e.lookup(id)

	out := make(chan model.Event, 128)
	var (
		liveCh   <-chan model.Event
		liveStop func()
	)
	if live {
		liveCh, liveStop = a.bus.Subscribe()
	}

	stop := func() {
		if liveStop != nil {
			liveStop()
		}
	}

	go func() {
		defer close(out)

		// 1) 补发历史（续传）。
		history, err := e.store.EventsSince(ctx, id, afterSeq)
		if err == nil {
			for _, ev := range history {
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			}
			if len(history) > 0 {
				afterSeq = history[len(history)-1].Seq
			}
		}

		// 2) 非活跃（已结束）：历史即全部，直接结束。
		if !live {
			return
		}

		// 3) 实时流；跳过与历史重叠的 seq，防重复。
		for {
			select {
			case ev, ok := <-liveCh:
				if !ok {
					return
				}
				if ev.Seq <= afterSeq {
					continue
				}
				afterSeq = ev.Seq
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, stop, nil
}
