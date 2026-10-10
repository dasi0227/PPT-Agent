package run

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

const lockTimeout = 30 * time.Second

// active 是一个正在执行的 Run 的运行时句柄。
type active struct {
	checkpointCtx   context.Context
	run             model.Run
	bus             *Bus
	queue           *InputQueue
	cancel          context.CancelFunc
	done            chan struct{}
	resumed         bool
	mu              sync.Mutex
	phase           workflow.RunPhase
	cancelRequested bool
	pauseRequested  bool
}

// Engine 管理 Run 生命周期：状态机、事件扇出、HITL、取消、每 project 锁。
type Engine struct {
	store                  Store
	locks                  *LockManager
	log                    *zap.Logger
	instanceID             string
	terminalContextRefresh func(context.Context, model.Run) error
	completedRunHandler    func(context.Context, model.Run) error

	mu       sync.Mutex
	actives  map[string]*active
	stopping bool
}

func NewEngine(store Store, locks *LockManager, log *zap.Logger) *Engine {
	return &Engine{store: store, locks: locks, log: log, instanceID: uuid.NewString(), actives: map[string]*active{}}
}

// WithTerminalContextRefresh covers terminal paths that do not pass through
// Runtime, including canceling a paused Run. Configure before starting Runs.
func (e *Engine) WithTerminalContextRefresh(refresh func(context.Context, model.Run) error) *Engine {
	e.terminalContextRefresh = refresh
	return e
}

// Configure before starting Runs. The handler runs after the project lock is
// released, while the old active handle still reserves its handoff position.
func (e *Engine) WithCompletedRunHandler(handler func(context.Context, model.Run) error) *Engine {
	e.completedRunHandler = handler
	return e
}

func (e *Engine) HasActiveProject(projectID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, a := range e.actives {
		if a.run.ProjectID == projectID {
			return true
		}
	}
	return false
}

// Recovery retries only durable, unconsumed inputs. A start failure leaves the
// inbox intact and is logged without preventing settings/UI access on startup.
func (e *Engine) RecoverPendingMessages(ctx context.Context) error {
	reader, ok := e.store.(interface {
		CompletedRunsWithPendingSteering(context.Context) ([]model.Run, error)
	})
	if !ok || e.completedRunHandler == nil {
		return nil
	}
	completed, err := reader.CompletedRunsWithPendingSteering(ctx)
	if err != nil {
		return err
	}
	for _, previous := range completed {
		if err := e.completedRunHandler(ctx, previous); err != nil {
			e.log.Error("recover queued messages in new run failed", zap.String("run_id", previous.ID), zap.Error(err))
		}
	}
	return nil
}

func (e *Engine) refreshTerminalContext(ctx context.Context, current model.Run) {
	if e.terminalContextRefresh != nil {
		if err := e.terminalContextRefresh(ctx, current); err != nil {
			e.log.Warn("refresh terminal context window failed", zap.String("run_id", current.ID), zap.Error(err))
		}
	}
}

// Initialize reconciles every non-terminal Run left by a previous process
// before the HTTP server becomes reachable. Paused Runs remain durable and
// continue to block writes for their project until explicitly resumed/canceled.
func (e *Engine) Initialize(ctx context.Context) error {
	lifecycle, ok := e.store.(LifecycleStore)
	if !ok {
		return ErrLifecycleStoreUnavailable
	}
	paused, err := lifecycle.PauseNonTerminalRuns(ctx, "server_restarted", time.Now().Unix())
	if err != nil {
		return err
	}
	if len(paused) > 0 {
		e.log.Info("orphaned runs paused on startup", zap.Int("count", len(paused)), zap.String("instance_id", e.instanceID))
	}
	return nil
}

// Start 创建 Run（pending）并异步执行 canonical execution。返回创建后的 Run 元数据。
// 每 project 锁在后台 goroutine 内获取：同 project 串行、跨 project 并行（ARCH-RUN-LOCK）。

// Resume starts an eligible existing Run from its persisted runtime checkpoint.
// It does not create a new runs row; recovery metadata is carried by the
// execution and checkpoint tables.
func (e *Engine) Resume(ctx context.Context, r model.Run, execution Execution) (model.Run, error) {
	if r.Status != model.RunPaused && !r.CanContinue {
		return model.Run{}, ErrRunNotContinuable
	}
	bus := NewBus(r.ID, e.store)
	bus.owner = r.OwnerInstanceID
	bus.execution = r.ExecutionRevision
	events, err := e.store.EventsSince(ctx, r.ID, 0)
	if err != nil {
		return model.Run{}, err
	}
	if err := bus.Restore(events); err != nil {
		return model.Run{}, err
	}
	if err := ensureRunStarted(ctx, bus, r); err != nil {
		return model.Run{}, err
	}
	queue := e.durableInputQueue(r.ID)
	runCtx, cancel := context.WithCancel(context.Background())

	// Claiming the durable row and registering the in-memory execution share the
	// shutdown mutex. This closes the race where PauseAll could otherwise take
	// its snapshot between the two operations and leave a recovering orphan.
	e.mu.Lock()
	if e.stopping {
		e.mu.Unlock()
		cancel()
		return model.Run{}, ErrEngineStopping
	}
	if _, exists := e.actives[r.ID]; exists {
		e.mu.Unlock()
		cancel()
		return model.Run{}, ErrRunNotRunning
	}
	lifecycle, ok := e.store.(LifecycleStore)
	if !ok {
		e.mu.Unlock()
		cancel()
		return model.Run{}, ErrLifecycleStoreUnavailable
	}
	var claimed model.Run
	if r.Status == model.RunPaused {
		claimed, err = lifecycle.ClaimPausedRun(ctx, r.ID, e.instanceID)
	} else if continuation, ok := e.store.(interface {
		ClaimContinuableRun(context.Context, model.Run, string) (model.Run, error)
	}); ok {
		claimed, err = continuation.ClaimContinuableRun(ctx, r, e.instanceID)
	} else {
		err = ErrRunNotContinuable
	}
	if err != nil {
		e.mu.Unlock()
		cancel()
		return model.Run{}, err
	}
	bus.owner = claimed.OwnerInstanceID
	bus.execution = claimed.ExecutionRevision
	if err := bus.Emit(ctx, model.EventRunResumed, model.RunResumedPayload{
		PublicEventBase: model.NewPublicEventBase(r.ID),
	}); err != nil {
		_ = lifecycle.ReleaseRecoveringRun(ctx, r.ID, e.instanceID, "resume_event_failed", time.Now().Unix())
		e.mu.Unlock()
		cancel()
		return model.Run{}, err
	}
	runCtx = workflow.WithCheckpointLease(runCtx, claimed.OwnerInstanceID, claimed.ExecutionRevision, claimed.CheckpointRevision)
	a := &active{checkpointCtx: runCtx, run: claimed, bus: bus, queue: queue, cancel: cancel, done: make(chan struct{}), resumed: true}
	e.actives[r.ID] = a
	e.mu.Unlock()
	go e.execute(runCtx, a, execution)
	return claimed, nil
}

func ensureRunStarted(ctx context.Context, bus *Bus, run model.Run) error {
	if bus.started {
		return nil
	}
	return bus.Emit(ctx, model.EventRunStarted, model.RunStartedPayload{
		PublicEventBase: model.NewPublicEventBase(run.ID),
		Scope:           run.Command.Scope,
		Mode:            run.Command.Mode,
		UserInput:       run.Command.Instruction,
		Skills:          run.Command.PublicSkills(),
		Resources:       run.Command.PublicComponents(),
		Attachments:     run.Command.Attachments,
		DOMSelections:   model.PublicDOMSelections(run.Command.DOMSelections),
		ReferenceOrder:  run.Command.ReferenceOrder,
	})
}

// Start durably accepts the Run and its input before execution begins.
func (e *Engine) Start(ctx context.Context, r model.Run, execution Execution) (model.Run, error) {
	// Keep creation and in-memory registration atomic with respect to PauseAll.
	// This is a shutdown-only lock boundary, so the short database section is a
	// deliberate trade-off for a strict lifecycle invariant.
	e.mu.Lock()
	if e.stopping {
		e.mu.Unlock()
		return model.Run{}, ErrEngineStopping
	}
	now := time.Now().Unix()
	r.Status = model.RunPending
	r.OwnerInstanceID = e.instanceID
	r.ExecutionRevision = 1
	r.CreatedAt = now
	r.UpdatedAt = now
	if err := e.store.CreateRun(ctx, r); err != nil {
		e.mu.Unlock()
		return model.Run{}, err
	}

	if err := CommitStartBarrier(ctx); err != nil {
		_ = e.store.SetRunStatus(context.WithoutCancel(ctx), r.ID, model.RunFailed)
		e.mu.Unlock()
		return model.Run{}, err
	}
	bus := NewBus(r.ID, e.store)
	bus.owner = r.OwnerInstanceID
	bus.execution = r.ExecutionRevision
	queue := e.durableInputQueue(r.ID)
	runCtx, cancel := context.WithCancel(context.Background())

	runCtx = workflow.WithCheckpointLease(runCtx, r.OwnerInstanceID, r.ExecutionRevision, r.CheckpointRevision)
	a := &active{checkpointCtx: runCtx, run: r, bus: bus, queue: queue, cancel: cancel, done: make(chan struct{})}
	e.actives[r.ID] = a
	e.mu.Unlock()

	go e.execute(runCtx, a, execution)
	return r, nil
}

func (e *Engine) execute(ctx context.Context, a *active, execution Execution) {
	defer func() {
		if recovered := recover(); recovered != nil {
			a.mu.Lock()
			paused := a.pauseRequested
			a.mu.Unlock()
			if !paused {
				terminalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := model.NewAgentError("INTERNAL", "runtime_panic", errors.New("runtime panic"))
				e.finishStatus(terminalCtx, a, model.RunFailed, model.EventRunError, model.NewRunTerminalPayload(
					a.run.ID,
					time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds(),
					nil,
					err.Public(),
				))
				e.log.Error("runtime panic recovered", zap.String("run_id", a.run.ID), zap.Any("panic", recovered))
			}
		}
		if e.completedRunHandler != nil {
			followUpCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			current, err := e.store.GetRun(followUpCtx, a.run.ID)
			if err == nil && current.Status == model.RunDone {
				err = e.completedRunHandler(followUpCtx, current)
			}
			cancel()
			if err != nil {
				e.log.Error("start queued messages in new run failed", zap.String("run_id", a.run.ID), zap.Error(err))
			}
		}
		e.mu.Lock()
		delete(e.actives, a.run.ID)
		e.mu.Unlock()
		close(a.done)
	}()

	if !a.resumed {
		if err := a.bus.Emit(ctx, model.EventRunStarted, model.RunStartedPayload{
			PublicEventBase: model.NewPublicEventBase(a.run.ID),
			Scope:           a.run.Command.Scope, Mode: a.run.Command.Mode,
			UserInput: a.run.Command.Instruction,
			Skills:    a.run.Command.PublicSkills(), Resources: a.run.Command.PublicComponents(), Attachments: a.run.Command.Attachments,
			DOMSelections: model.PublicDOMSelections(a.run.Command.DOMSelections), ReferenceOrder: a.run.Command.ReferenceOrder,
		}); err != nil {
			e.setStatus(context.Background(), a.run.ID, model.RunFailed)
			return
		}
	}

	// 获取 project 锁（同 project 串行）。锁超时 → Run 转 failed（ARCH-RUN-LOCK-005）。
	release, err := e.locks.Acquire(ctx, a.run.ProjectID, lockTimeout)
	if err != nil {
		terminalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		e.finishStatus(terminalCtx, a, model.RunFailed, model.EventRunFailed, model.NewRunTerminalPayload(
			a.run.ID,
			time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds(),
			nil,
			model.NewAgentError("LOCK_TIMEOUT", "project_lock", err).Public(),
		))
		return
	}
	defer release()

	e.setStatus(ctx, a.run.ID, model.RunRunning)

	em := &workflowEmitter{ctx: ctx, bus: a.bus, cancel: a.cancel, active: a}
	cp := &inputCheckpoint{queue: a.queue, store: e.store, runID: a.run.ID, active: a}
	prompter := &checkpoint{engine: e, runID: a.run.ID, bus: a.bus, queue: a.queue}
	outcome := execution.Run(ctx, em, cp, prompter)

	terminalCtx, cancelTerminal := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTerminal()
	if em.Err() != nil {
		e.log.Error("workflow event delivery failed", zap.String("run_id", a.run.ID), zap.Error(em.Err()))
		if err := em.pauseAfterFailure(terminalCtx); err != nil {
			e.log.Error("pause run after event delivery failure", zap.Error(err))
		}
		return
	}
	e.finish(terminalCtx, a, outcome)
}

// finish persists the scheduler status. The workflow normally emits its own
// terminal event; the guarded fallback below covers custom test executions.
func (e *Engine) finish(ctx context.Context, a *active, outcome workflow.StructuredOutcome) {
	a.mu.Lock()
	cancelRequested := a.cancelRequested
	pauseRequested := a.pauseRequested
	a.mu.Unlock()
	if pauseRequested {
		return
	}
	if cancelRequested {
		outcome.Status = workflow.StatusCanceled
		outcome.Code = workflow.CodeCanceled
	}
	durationMS := time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds()
	if outcome.DurationMS != nil && *outcome.DurationMS >= 0 {
		durationMS = *outcome.DurationMS
	}
	if pending, err := e.store.ListPendingSteering(ctx, a.run.ID); outcome.Status != workflow.StatusCompleted && err == nil && len(pending) > 0 {
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
		if !a.bus.Terminated() {
			_ = a.bus.Emit(ctx, model.EventMessageFinal, model.MessageFinalPayload{
				PublicEventBase: model.NewPublicEventBase(a.run.ID),
				MessageID:       "msg_fallback_" + a.run.ID, Text: "已完成本次任务。",
				AffectedTargets: []model.PublicTarget{}, SuggestedNextInputs: workflow.NormalizeSuggestedNextInputs(outcome.SuggestedNextInputs),
			})
		}
		e.finishStatus(ctx, a, model.RunDone, model.EventRunCompleted, model.NewRunTerminalPayload(a.run.ID, durationMS, nil, nil))
	case workflow.StatusCanceled:
		e.finishStatus(ctx, a, model.RunCanceled, model.EventRunCanceled, model.NewRunTerminalPayload(a.run.ID, durationMS, nil, nil))
	default:
		code := outcome.Code
		if code == "" {
			code = "RUN_FAILED"
		}
		e.finishStatus(ctx, a, model.RunFailed, model.EventRunFailed, model.NewRunTerminalPayload(
			a.run.ID,
			durationMS,
			nil,
			model.NewAgentError(code, "run", errors.New(outcome.Message)).Public(),
		))
	}
}

func (e *Engine) finishStatus(ctx context.Context, a *active, status model.RunStatus, event model.EventType, payload model.RunTerminalPayload) {
	if !a.bus.Terminated() {
		e.refreshTerminalContext(ctx, a.run)
		// The store commits the terminal event and state together. Publishing
		// after setStatus would try to use ownership that was already released.
		if err := a.bus.Emit(ctx, event, payload); err != nil {
			e.log.Error("persist run terminal event failed", zap.String("run_id", a.run.ID), zap.Error(err))
			a.mu.Lock()
			a.pauseRequested = true
			a.mu.Unlock()
			if lifecycle, ok := e.store.(LifecycleStore); ok {
				if _, err := lifecycle.PauseRun(ctx, a.run.ID, a.run.OwnerInstanceID, eventFailurePauseReason(err), time.Now().Unix()); err != nil {
					e.log.Error("pause run after terminal event failure", zap.String("run_id", a.run.ID), zap.Error(err))
				}
			}
			return
		}
	}
	// Idempotent for the durable store; also supports scheduler-only stores.
	e.setStatus(ctx, a.run.ID, status)
	e.finalizeResourceApprovals(ctx, a.run)
}

func (e *Engine) finalizeResourceApprovals(ctx context.Context, current model.Run) {
	if projects, ok := e.store.(interface {
		GetProject(context.Context, string) (model.Project, error)
	}); ok {
		if project, err := projects.GetProject(ctx, current.ProjectID); err == nil {
			if err := workflow.FinalizeResourceEditApprovals(project.WorkDir, current.ID); err != nil {
				e.log.Warn("finalize resource approvals failed", zap.String("run_id", current.ID), zap.Error(err))
			}
		}
	}
}

func (e *Engine) setStatus(ctx context.Context, id string, status model.RunStatus) {
	if activeRun, ok := e.lookup(id); ok {
		activeRun.mu.Lock()
		paused := activeRun.pauseRequested
		activeRun.mu.Unlock()
		if paused && status != model.RunPaused {
			return
		}
	}
	if active, ok := e.lookup(id); ok {
		ctx = workflow.WithCheckpointLease(ctx, active.run.OwnerInstanceID, active.run.ExecutionRevision, 0)
	}
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
			return a.queue.ReplyError()
		}
		return nil
	}
	return ErrReplyMismatch
}

func (e *Engine) SubmitPlanApproval(ctx context.Context, id string, answer model.PlanApprovalAnswer) error {
	a, ok := e.lookup(id)
	if !ok {
		return ErrRunNotRunning
	}
	if !a.queue.ReplyPlanApproval(answer) {
		return a.queue.ReplyError()
	}
	return nil
}

func (e *Engine) SubmitCommandPermission(ctx context.Context, id string, answer model.CommandPermissionAnswer) error {
	a, ok := e.lookup(id)
	if !ok {
		return ErrRunNotRunning
	}
	if !a.queue.ReplyCommandPermission(answer) {
		return a.queue.ReplyError()
	}
	return nil
}

func (e *Engine) SubmitScopeExpansion(ctx context.Context, id string, answer model.ScopeExpansionAnswer) error {
	a, ok := e.lookup(id)
	if !ok {
		return ErrRunNotRunning
	}
	if !a.queue.ReplyScopeExpansion(answer) {
		return a.queue.ReplyError()
	}
	return nil
}

func (e *Engine) SubmitResourceEditApproval(ctx context.Context, id string, answer model.ResourceEditApprovalAnswer) error {
	a, ok := e.lookup(id)
	if !ok {
		return ErrRunNotRunning
	}
	if !a.queue.ReplyResourceApproval(answer) {
		return a.queue.ReplyError()
	}
	return nil
}

func (e *Engine) EmitResourceEditApprovalUpdated(ctx context.Context, id string, payload model.ResourceEditApprovalUpdatedPayload) error {
	a, ok := e.lookup(id)
	if !ok {
		return ErrRunNotRunning
	}
	return a.bus.Emit(ctx, model.EventResourceEditApprovalUpdated, payload)
}

func (e *Engine) RequestCancelWithReason(ctx context.Context, id string, reason model.RunCancelReason) (model.Run, error) {
	requestedAt := time.Now().UnixNano()
	stored, getErr := e.store.GetRun(ctx, id)
	if getErr != nil {
		return model.Run{}, ErrRunNotFound
	}
	if stored.Status == model.RunPaused {
		lifecycle, ok := e.store.(LifecycleStore)
		if !ok {
			return model.Run{}, ErrLifecycleStoreUnavailable
		}
		canceled, err := lifecycle.CancelPausedRun(ctx, id, requestedAt)
		if err != nil {
			return model.Run{}, err
		}
		e.refreshTerminalContext(ctx, canceled)
		e.finalizeResourceApprovals(ctx, canceled)
		bus := NewBus(canceled.ID, e.store)
		if events, eventErr := e.store.EventsSince(ctx, id, 0); eventErr == nil && bus.Restore(events) == nil {
			if ensureRunStarted(ctx, bus, canceled) == nil {
				payload := model.NewRunTerminalPayload(
					id, time.Since(time.Unix(canceled.CreatedAt, 0)).Milliseconds(), nil, nil,
				)
				payload.Reason = reason
				_ = bus.Emit(ctx, model.EventRunCanceled, payload)
			}
		}
		return canceled, nil
	}
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
		case "error":
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
		PublicEventBase: model.NewPublicEventBase(id), Activity: model.ActivityRunCanceling,
	})
	a.cancel()
	current.CancelRequestedAt = requestedAt
	return current, nil
}

// PauseAll is the graceful-shutdown path. Crash safety is provided separately
// by Initialize, which pauses any non-terminal rows on the next process start.
func (e *Engine) PauseAll(ctx context.Context, reason string) error {
	e.mu.Lock()
	e.stopping = true
	actives := make([]*active, 0, len(e.actives))
	for _, activeRun := range e.actives {
		actives = append(actives, activeRun)
	}
	e.mu.Unlock()

	var failures []error
	lifecycle, ok := e.store.(LifecycleStore)
	if !ok && len(actives) > 0 {
		return ErrLifecycleStoreUnavailable
	}
	for _, activeRun := range actives {
		activeRun.mu.Lock()
		activeRun.pauseRequested = true
		activeRun.mu.Unlock()
		pausedRun, err := lifecycle.PauseRun(ctx, activeRun.run.ID, e.instanceID, reason, time.Now().Unix())
		if err != nil {
			failures = append(failures, fmt.Errorf("pause %s: %w", activeRun.run.ID, err))
			activeRun.cancel()
			continue
		}
		// The workflow may have persisted a terminal state just before the
		// shutdown snapshot. Let its terminal event finish before canceling.
		if pausedRun.Status.Terminal() {
			continue
		}
		activeRun.cancel()
	}
	for _, activeRun := range actives {
		select {
		case <-activeRun.done:
		case <-ctx.Done():
			failures = append(failures, fmt.Errorf("wait for %s: %w", activeRun.run.ID, ctx.Err()))
			return errors.Join(failures...)
		}
	}
	return errors.Join(failures...)
}

func (e *Engine) SteerWithReferences(ctx context.Context, runID, expectedRunID, clientMessageID, requestHash, content string, attachments []model.AttachmentReference, domSelections []model.DOMSelection, referenceOrder []model.ReferenceOrderItem, scope model.RunScope) (model.SteeringMessage, error) {
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
		RequestHash: requestHash, Content: content, Attachments: attachments, Status: model.SteeringAccepted,
		DOMSelections: domSelections, ReferenceOrder: referenceOrder, Scope: scope,
		AcceptedAt: time.Now().UnixNano(),
	}
	if a.checkpointCtx != nil {
		ctx = workflow.ShareCheckpointLease(ctx, a.checkpointCtx)
	}
	existing, created, err := e.store.CreateSteering(ctx, message)
	if err != nil {
		if errors.Is(err, ErrRunRevisionConflict) {
			return model.SteeringMessage{}, model.NewAgentError("RUN_REVISION_CONFLICT", "steer_run", err)
		}
		if errors.Is(err, ErrRunNotRunning) {
			return model.SteeringMessage{}, model.NewAgentError("RUN_NOT_STEERABLE", "steer_run", err)
		}
		return model.SteeringMessage{}, err
	}
	if !created && existing.RequestHash != requestHash {
		return model.SteeringMessage{}, model.NewAgentError("IDEMPOTENCY_KEY_REUSED", "steer_run", nil)
	}
	if created && scope.Source.Kind != "" {
		a.run.Command.Scope = scope
	}
	return existing, nil
}

// ProjectLocks shares execution exclusion with project history switches.
func (e *Engine) ProjectLocks() *LockManager { return e.locks }
