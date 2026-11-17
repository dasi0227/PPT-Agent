package run

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type InputResolver func(context.Context, model.Run, string) (model.InputDecision, error)

// Composer steering IDs cannot contain ':'. Separate the initial-input key so
// reusing its client ID for steering cannot reuse its earlier classification.
func initialInputKey(id string) string { return "initial:" + id }

type pendingInput struct {
	done   chan struct{}
	result model.InputDecision
	err    error
}

type inputGate struct {
	mu      sync.Mutex
	pending map[string]*pendingInput
	order   []string
	ctx     context.Context
	active  *active
	resolve InputResolver
}

func (e *Engine) WithInputResolver(resolve InputResolver) *Engine {
	e.resolveInput = resolve
	return e
}

func (e *Engine) newInputGate(ctx context.Context, a *active) *inputGate {
	if e.resolveInput == nil {
		return nil
	}
	return &inputGate{pending: map[string]*pendingInput{}, ctx: ctx, active: a, resolve: e.resolveInput}
}

func (g *inputGate) start(id string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	if _, exists := g.pending[id]; exists {
		g.mu.Unlock()
		return
	}
	p := &pendingInput{done: make(chan struct{})}
	g.pending[id] = p
	g.order = append(g.order, id)
	g.mu.Unlock()
	go func() {
		p.result, p.err = g.resolve(g.ctx, g.active.run, id)
		// Record the reason before waking the worker. In-flight model calls and
		// pending approvals both observe the same cancellation.
		if p.err == nil && p.result.StopsRun() && !p.result.IsInitial() {
			g.active.mu.Lock()
			if !g.active.pauseRequested && g.active.inputStop == nil {
				result := p.result
				g.active.inputStop = &result
				g.active.cancel()
			}
			g.active.mu.Unlock()
		}
		close(p.done)
		if p.err != nil && g.ctx.Err() == nil {
			(&inputCheckpoint{store: g.active.bus.store, runID: g.active.run.ID, active: g.active}).PersistenceFailed(g.ctx, "input_decision_write_failed")
		}
	}()
}

func (g *inputGate) wait(ctx context.Context) error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	list := make([]*pendingInput, 0, len(g.order))
	for _, id := range g.order {
		list = append(list, g.pending[id])
	}
	g.mu.Unlock()
	var firstStop *model.InputDecision
	for _, p := range list {
		select {
		case <-p.done:
		case <-ctx.Done():
			return ctx.Err()
		}
		if p.err != nil {
			return p.err
		}
		if firstStop == nil && p.result.StopsRun() {
			result := p.result
			firstStop = &result
		}
	}
	if firstStop != nil {
		g.active.mu.Lock()
		g.active.inputStop = firstStop
		g.active.mu.Unlock()
		return errors.New("user input cannot be executed")
	}
	return nil
}

func (g *inputGate) result(id string) model.InputDecision {
	g.mu.Lock()
	p := g.pending[id]
	g.mu.Unlock()
	if p == nil {
		return model.InputDecision{Disposition: model.InputTask}
	}
	<-p.done
	return p.result
}

func (g *inputGate) resolvedStop() *model.InputDecision {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, id := range g.order {
		p := g.pending[id]
		select {
		case <-p.done:
			if p.err == nil && p.result.StopsRun() {
				result := p.result
				return &result
			}
		default:
		}
	}
	return nil
}

func (e *Engine) finishInputError(ctx context.Context, a *active, storageReason string) {
	a.mu.Lock()
	stopped, canceled, paused := a.inputStop, a.cancelRequested, a.pauseRequested || a.deleted
	a.mu.Unlock()
	switch {
	case paused:
		return
	case stopped != nil:
		e.finishInputStop(context.WithoutCancel(ctx), a, *stopped)
	case canceled || ctx.Err() != nil:
		e.finish(context.WithoutCancel(ctx), a, workflow.StructuredOutcome{Status: workflow.StatusCanceled})
	default:
		(&inputCheckpoint{store: e.store, runID: a.run.ID, active: a}).PersistenceFailed(ctx, storageReason)
	}
}

func (e *Engine) finishInputStop(ctx context.Context, a *active, result model.InputDecision) {
	ctx = sessionstate.WithRequestScene(ctx, a.run.SceneRevision)
	// Runtime has exited, including its in-flight cancellation/operation cleanup.
	// State and the reply claiming a stop commit in one terminal event transaction.
	reason := model.RunCancelInputRejected
	if result.Disposition == model.InputDetectionFailed {
		reason = model.RunCancelDetectionFailed
	}
	if pending, err := e.store.ListPendingSteering(ctx, a.run.ID); err == nil {
		ids := make([]string, 0, len(pending))
		for _, input := range pending {
			ids = append(ids, input.ClientMessageID)
		}
		if err := e.store.MarkSteering(ctx, a.run.ID, ids, model.SteeringRejected, time.Now().UnixNano(), string(reason)); err != nil {
			e.log.Error("persist rejected inputs failed", zap.Error(err))
		}
	}
	a.mu.Lock()
	targets := a.inputStopTargets
	a.mu.Unlock()
	payload := model.NewRunTerminalPayload(a.run.ID, time.Since(time.UnixMilli(a.run.CreatedAt)).Milliseconds(), targets, nil)
	payload.Reason = reason
	text := result.Reply()
	if result.Disposition == model.InputDetectionFailed && (a.resumed || !result.IsInitial()) {
		text += "当前任务已停止。"
	}
	payload.Reply = &model.InputReply{InputID: strings.TrimPrefix(result.InputID, "initial:"), Text: text}
	e.finishStatus(ctx, a, model.RunCanceled, model.EventRunCanceled, payload)
}

func (e *Engine) finishInitialReply(ctx context.Context, a *active, initial model.InputDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.replyOnly = true
	duration := time.Since(time.UnixMilli(a.run.CreatedAt)).Milliseconds()
	if initial.Failure != "" {
		failure := model.NewAgentError(initial.Failure, "chat_reply", nil)
		e.finishStatus(ctx, a, model.RunFailed, model.EventRunFailed, model.NewRunTerminalPayload(a.run.ID, duration, nil, failure.Public()))
		return nil
	}
	if err := a.bus.emitDirectReply(ctx, initial, model.EventMessageFinal); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	payload := model.NewRunTerminalPayload(a.run.ID, duration, nil, nil)
	payload.SuppressSuggestions = true
	e.finishStatus(ctx, a, model.RunDone, model.EventRunCompleted, payload)
	return nil
}

// Startup recovery resolves checks even when the worker is paused and has no
// future model call. It never resumes authoring or replays completed tool calls.
func (e *Engine) RecoverInputChecks(ctx context.Context) error {
	if e.resolveInput == nil {
		return nil
	}
	reader, ok := e.store.(interface {
		PausedInputRuns(context.Context) ([]model.Run, error)
		GuardSteeringInputs(context.Context, string) ([]model.SteeringMessage, error)
	})
	if !ok {
		return errors.New("input recovery store is required")
	}
	runs, err := reader.PausedInputRuns(ctx)
	if err != nil {
		return err
	}
	for _, current := range runs {
		ctx := sessionstate.WithRequestScene(ctx, current.SceneRevision)
		bus := NewBus(current, e.store)
		a := &active{run: current, bus: bus, cancel: func() {}, done: make(chan struct{})}
		a.inputGate = e.newInputGate(e.withUsageRecorder(ctx, a), a)
		a.inputGate.start(initialInputKey(current.ClientRequestID))
		inputs, err := reader.GuardSteeringInputs(ctx, current.ID)
		if err != nil {
			return err
		}
		for _, input := range inputs {
			a.inputGate.start(input.ClientMessageID)
		}
		if err := a.inputGate.wait(ctx); err != nil {
			a.mu.Lock()
			stop := a.inputStop
			a.mu.Unlock()
			if stop == nil {
				return err
			}
			e.finishInputStop(ctx, a, *stop)
			continue
		}
		initial := a.inputGate.result(initialInputKey(current.ClientRequestID))
		replyOnly := initial.RepliesDirectly() && current.CheckpointRevision == 0
		for _, input := range inputs {
			if a.inputGate.result(input.ClientMessageID).Disposition == model.InputTask {
				replyOnly = false
				break
			}
		}
		if replyOnly {
			if err := e.finishInitialReply(ctx, a, initial); err != nil {
				return err
			}
		} else if initial.RepliesDirectly() {
			if err := bus.emitDirectReply(ctx, initial, model.EventMessageReply); err != nil {
				return err
			}
		}
	}
	return nil
}
