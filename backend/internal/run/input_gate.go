package run

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type InputClassifier func(context.Context, model.Run, string, string) (model.InputDecision, error)

// Composer steering IDs cannot contain ':'. Separate the initial-input key so
// reusing its client ID for steering cannot reuse its earlier classification.
func initialInputKey(id string) string { return "initial:" + id }

type pendingInput struct {
	done   chan struct{}
	result model.InputDecision
	err    error
}

type inputGate struct {
	mu       sync.Mutex
	pending  map[string]*pendingInput
	order    []string
	ctx      context.Context
	active   *active
	classify InputClassifier
}

func (e *Engine) WithInputClassifier(classify InputClassifier) *Engine {
	e.classifyInput = classify
	return e
}

func (e *Engine) newInputGate(ctx context.Context, a *active) *inputGate {
	if e.classifyInput == nil {
		return nil
	}
	return &inputGate{pending: map[string]*pendingInput{}, ctx: ctx, active: a, classify: e.classifyInput}
}

func (g *inputGate) start(id, text string) {
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
		p.result, p.err = g.classify(g.ctx, g.active.run, id, text)
		close(p.done)
		if p.err != nil && g.ctx.Err() == nil {
			(&inputCheckpoint{store: g.active.bus.store, runID: g.active.run.ID, active: g.active}).PersistenceFailed(g.ctx, "input_decision_write_failed")
			return
		}
		// A waiting worker has no next model boundary. Wake its existing
		// cancellation path; the scheduler publishes the actual stop reason.
		if p.err == nil && p.result.StopsRun() {
			g.active.mu.Lock()
			if g.active.phase == workflow.PhaseWaitingInput && !g.active.pauseRequested {
				result := p.result
				g.active.inputStop = &result
				g.active.cancel()
			}
			g.active.mu.Unlock()
		} else if p.err == nil && p.result.Reply() != "" && !strings.HasPrefix(id, "initial:") {
			g.active.mu.Lock()
			waiting := g.active.phase == workflow.PhaseWaitingInput && !g.active.pauseRequested
			g.active.mu.Unlock()
			if waiting {
				(&inputCheckpoint{store: g.active.bus.store, runID: g.active.run.ID, active: g.active}).respondWhileWaiting()
			}
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
		return model.InputDecision{Disposition: model.InputNormal}
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

func (e *Engine) finishInputStop(ctx context.Context, a *active, result model.InputDecision) {
	// Runtime has exited, including its in-flight cancellation/operation cleanup.
	// State and the reply claiming a stop commit in one terminal event transaction.
	a.bus.RequestCancelAuthority()
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
	payload := model.NewRunTerminalPayload(a.run.ID, time.Since(time.Unix(a.run.CreatedAt, 0)).Milliseconds(), targets, nil)
	payload.Reason = reason
	payload.Reply = &model.InputReply{InputID: strings.TrimPrefix(result.InputID, "initial:"), Text: result.Reply() + "当前任务已停止。"}
	e.finishStatus(ctx, a, model.RunCanceled, model.EventRunCanceled, payload)
}

// Startup recovery resolves checks even when the worker is paused and has no
// future model call. It never resumes authoring or replays completed tool calls.
func (e *Engine) RecoverInputChecks(ctx context.Context) error {
	if e.classifyInput == nil {
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
		bus := NewBus(current.ID, e.store)
		events, err := e.store.EventsSince(ctx, current.ID, 0)
		if err != nil {
			return err
		}
		if err := bus.Restore(events); err != nil {
			return err
		}
		if err := ensureRunStarted(ctx, bus, current); err != nil {
			return err
		}
		a := &active{run: current, bus: bus, cancel: func() {}, done: make(chan struct{})}
		a.inputGate = e.newInputGate(ctx, a)
		a.inputGate.start(initialInputKey(current.ClientRequestID), current.Command.Instruction)
		inputs, err := reader.GuardSteeringInputs(ctx, current.ID)
		if err != nil {
			return err
		}
		for _, input := range inputs {
			a.inputGate.start(input.ClientMessageID, input.Content)
		}
		if err := a.inputGate.wait(ctx); err != nil {
			if a.inputStop == nil {
				return err
			}
			e.finishInputStop(ctx, a, *a.inputStop)
			continue
		}
		initial := a.inputGate.result(initialInputKey(current.ClientRequestID))
		onlyFixed := initial.Reply() != "" && bus.planID == "" && len(bus.toolCalls) == 0
		for _, input := range inputs {
			if a.inputGate.result(input.ClientMessageID).Reply() == "" {
				onlyFixed = false
			}
		}
		if onlyFixed {
			for _, input := range inputs {
				result := a.inputGate.result(input.ClientMessageID)
				if err := bus.Emit(ctx, model.EventMessageReply, model.MessageReplyPayload{PublicEventBase: model.NewPublicEventBase(current.ID), MessageID: "input_" + input.ClientMessageID, InputID: input.ClientMessageID, Text: result.Reply()}); err != nil {
					return err
				}
				if err := e.store.MarkSteering(ctx, current.ID, []string{input.ClientMessageID}, model.SteeringInjected, time.Now().UnixNano(), ""); err != nil {
					return err
				}
			}
			if bus.finalCount == 0 {
				if err := bus.Emit(ctx, model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(current.ID), MessageID: "input_" + current.ClientRequestID, Text: initial.Reply(), AffectedTargets: []model.PublicTarget{}}); err != nil {
					return err
				}
			}
			payload := model.NewRunTerminalPayload(current.ID, 0, nil, nil)
			payload.SuppressSuggestions = true
			e.finishStatus(ctx, a, model.RunDone, model.EventRunCompleted, payload)
		}
	}
	return nil
}
