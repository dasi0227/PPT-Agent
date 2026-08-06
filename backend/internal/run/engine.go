package run

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

const lockTimeout = 30 * time.Second

// active 是一个正在执行的 Run 的运行时句柄。
type active struct {
	run             model.Run
	bus             *Bus
	queue           *InputQueue
	cancel          context.CancelFunc
	mu              sync.Mutex
	phase           workflow.RuntimePhase
	cancelRequested bool
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

// Start 创建 Run（pending）并异步执行 canonical execution。返回创建后的 Run 元数据。
// 每 project 锁在后台 goroutine 内获取：同 project 串行、跨 project 并行（ARCH-RUN-LOCK）。
func (e *Engine) Start(ctx context.Context, r model.Run, execution Execution) (model.Run, error) {
	return e.StartWithContext(ctx, r, execution, nil)
}

// Resume starts an existing non-terminal Run from a persisted runtime checkpoint.
// It does not create a new runs row; recovery metadata is carried by the
// execution and checkpoint tables.
func (e *Engine) Resume(ctx context.Context, r model.Run, execution Execution) (model.Run, error) {
	if r.Status.Terminal() {
		return model.Run{}, ErrRunNotRunning
	}
	e.mu.Lock()
	if _, exists := e.actives[r.ID]; exists {
		e.mu.Unlock()
		return model.Run{}, ErrRunNotRunning
	}
	e.mu.Unlock()
	bus := NewBus(r.ID, r.ThreadID, e.store, e.hw)
	queue := NewInputQueue()
	runCtx, cancel := context.WithCancel(context.Background())
	a := &active{run: r, bus: bus, queue: queue, cancel: cancel}
	e.mu.Lock()
	e.actives[r.ID] = a
	e.mu.Unlock()
	go e.execute(runCtx, a, execution)
	return r, nil
}

// StartWithContext atomically establishes the Run row and its auditable ContextManifest
// before any execution code runs.
func (e *Engine) StartWithContext(ctx context.Context, r model.Run, execution Execution, manifest *model.RunContext) (model.Run, error) {
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

	go e.execute(runCtx, a, execution)
	return r, nil
}

func (e *Engine) execute(ctx context.Context, a *active, execution Execution) {
	defer func() {
		a.bus.Close()
		e.mu.Lock()
		delete(e.actives, a.run.ID)
		e.mu.Unlock()
	}()

	if err := a.bus.Emit(ctx, model.EventRunStarted, model.RunStartedPayload{
		PublicEventBase: model.NewPublicEventBase(a.run.ID),
		Target:          a.run.WorkSpec.Target, Interaction: a.run.WorkSpec.Interaction,
		UserInput: a.run.WorkSpec.Instruction,
	}); err != nil {
		e.setStatus(context.Background(), a.run.ID, model.RunFailed)
		return
	}

	// 获取 project 锁（同 project 串行）。锁超时 → Run 转 failed（ARCH-RUN-LOCK-005）。
	release, err := e.locks.Acquire(ctx, a.run.ProjectID, lockTimeout)
	if err != nil {
		terminalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		e.setStatus(terminalCtx, a.run.ID, model.RunFailed)
		_ = a.bus.Emit(terminalCtx, model.EventRunFinished, model.RunFinishedPayload{
			PublicEventBase: model.NewPublicEventBase(a.run.ID),
			Status:          "failed", DurationMS: time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds(),
			Error: model.NewAgentError("LOCK_TIMEOUT", "project_lock", err).Public(),
		})
		return
	}
	defer release()

	e.setStatus(ctx, a.run.ID, model.RunRunning)

	em := &workflowEmitter{ctx: ctx, bus: a.bus}
	cp := &inputCheckpoint{queue: a.queue, store: e.store, runID: a.run.ID, active: a}
	prompter := &checkpoint{engine: e, runID: a.run.ID, bus: a.bus, queue: a.queue}
	outcome := execution.Run(ctx, em, cp, prompter)

	terminalCtx, cancelTerminal := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTerminal()
	e.finish(terminalCtx, a, outcome)
}

// finish persists the scheduler status. The workflow normally emits its own
// terminal event; the guarded fallback below covers custom test executions.
func (e *Engine) finish(ctx context.Context, a *active, outcome workflow.StructuredOutcome) {
	a.mu.Lock()
	cancelRequested := a.cancelRequested
	a.mu.Unlock()
	if cancelRequested {
		outcome.Status = workflow.StatusCanceled
		outcome.Code = workflow.CodeCanceled
	}
	if pending, err := e.store.ListPendingSteering(ctx, a.run.ID); err == nil && len(pending) > 0 {
		ids := make([]string, 0, len(pending))
		for _, message := range pending {
			ids = append(ids, message.ClientMessageID)
		}
		code := "RUN_NOT_STEERABLE"
		if outcome.Status == workflow.StatusCanceled {
			code = "RUN_CANCELING"
		}
		_ = e.store.MarkSteering(ctx, a.run.ID, ids, model.SteeringRejected, time.Now().UnixNano(), code)
	}
	switch outcome.Status {
	case workflow.StatusCompleted:
		e.setStatus(ctx, a.run.ID, model.RunDone)
		if !a.bus.Terminated() {
			_ = a.bus.Emit(ctx, model.EventMessageFinal, model.MessageFinalPayload{
				PublicEventBase: model.NewPublicEventBase(a.run.ID),
				MessageID:       "msg_fallback_" + a.run.ID, Text: "已完成本次任务。",
			})
			_ = a.bus.Emit(ctx, model.EventRunFinished, model.RunFinishedPayload{
				PublicEventBase: model.NewPublicEventBase(a.run.ID), Status: "completed",
				DurationMS: time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds(),
			})
		}
	case workflow.StatusCanceled:
		e.setStatus(ctx, a.run.ID, model.RunCanceled)
		if !a.bus.Terminated() {
			_ = a.bus.Emit(ctx, model.EventRunFinished, model.RunFinishedPayload{
				PublicEventBase: model.NewPublicEventBase(a.run.ID), Status: "canceled",
				DurationMS: time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds(),
			})
		}
	default:
		e.setStatus(ctx, a.run.ID, model.RunFailed)
		if !a.bus.Terminated() {
			code := outcome.Code
			if code == "" {
				code = "RUN_FAILED"
			}
			_ = a.bus.Emit(ctx, model.EventRunFinished, model.RunFinishedPayload{
				PublicEventBase: model.NewPublicEventBase(a.run.ID), Status: "failed",
				DurationMS: time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds(),
				Error:      model.NewAgentError(code, "run", errors.New(outcome.Message)).Public(),
			})
		}
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
// 带 replyTo 时必须匹配未应答 question 且通过结构化答案校验。
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
	return ErrReplyMismatch
}

// Cancel 取消 Run：停止后续 LLM 调用，保留已落盘产物（ARCH-RUN-004 / API-RUN-005）。
func (e *Engine) Cancel(ctx context.Context, id string) error {
	_, err := e.RequestCancel(ctx, id)
	return err
}

func (e *Engine) RequestCancel(ctx context.Context, id string) (model.Run, error) {
	requestedAt := time.Now().UnixNano()
	current, err := e.store.RequestRunCancel(ctx, id, requestedAt)
	if err != nil {
		return model.Run{}, ErrRunNotFound
	}
	if current.Status.Terminal() {
		return current, nil
	}
	if current.CancelRequestedAt != 0 && current.CancelRequestedAt != requestedAt {
		return current, nil
	}
	a, ok := e.lookup(id)
	if !ok {
		return current, nil
	}
	cancelWon, terminalStatus := a.bus.RequestCancelAuthority()
	if !cancelWon {
		switch terminalStatus {
		case "completed":
			current.Status = model.RunDone
		case "failed":
			current.Status = model.RunFailed
		case "canceled":
			current.Status = model.RunCanceled
		}
		return current, nil
	}
	a.mu.Lock()
	a.cancelRequested = true
	a.mu.Unlock()
	_ = a.bus.Emit(context.Background(), model.EventRunProgress, model.RunProgressPayload{
		PublicEventBase: model.NewPublicEventBase(id), Stage: "finalizing", Text: "正在取消",
	})
	a.cancel()
	current.CancelRequestedAt = requestedAt
	return current, nil
}

func (e *Engine) Steer(ctx context.Context, runID, expectedRunID, clientMessageID, requestHash, content string) (model.SteeringMessage, error) {
	if runID != expectedRunID {
		return model.SteeringMessage{}, model.NewAgentError("RUN_NOT_STEERABLE", "steer_run", nil)
	}
	a, ok := e.lookup(runID)
	if !ok {
		if _, err := e.store.GetRun(ctx, runID); err != nil {
			return model.SteeringMessage{}, model.NewAgentError("RUN_NOT_FOUND", "steer_run", err)
		}
		return model.SteeringMessage{}, model.NewAgentError("RUN_NOT_STEERABLE", "steer_run", nil)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelRequested {
		return model.SteeringMessage{}, model.NewAgentError("RUN_CANCELING", "steer_run", nil)
	}
	if a.queue.HasAwaiting() || a.phase == workflow.PhaseWaitingInput {
		return model.SteeringMessage{}, model.NewAgentError("RUN_WAITING_FOR_ANSWER", "steer_run", nil)
	}
	if a.phase == workflow.PhaseCompletionCheck || a.phase == workflow.PhaseCommitting || a.phase == workflow.PhaseTerminal {
		return model.SteeringMessage{}, model.NewAgentError("RUN_NOT_STEERABLE", "steer_run", nil)
	}
	message := model.SteeringMessage{
		RunID: runID, ThreadID: a.run.ThreadID, ClientMessageID: clientMessageID,
		RequestHash: requestHash, Content: content, Status: model.SteeringAccepted,
		AcceptedAt: time.Now().UnixNano(),
	}
	existing, created, err := e.store.CreateSteering(ctx, message)
	if err != nil {
		return model.SteeringMessage{}, err
	}
	if !created && existing.RequestHash != requestHash {
		return model.SteeringMessage{}, model.NewAgentError("IDEMPOTENCY_KEY_REUSED", "steer_run", nil)
	}
	if created && e.hw != nil {
		_ = a.bus.AppendSteeringHistory(ctx, message)
	}
	return existing, nil
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
