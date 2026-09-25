package run

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// Bus validates Run event order and broadcasts only after journal delivery.
// Sequence numbers belong to the thread and may have gaps within one Run.
type Bus struct {
	answeredInteractions map[string]bool
	publishedTransitions map[string]bool
	owner                string
	execution            int64
	runID                string
	threadID             string
	store                Store

	mu              sync.Mutex
	seq             int64
	subscribers     map[int]chan model.Event
	nextSubID       int
	closed          bool
	terminated      bool // 已发过终态事件，防止重复终态
	terminalStatus  string
	cancelRequested bool
	started         bool
	finalCount      int
	planID          string
	planCompleted   map[string]bool
	toolCalls       map[string]bool
	toolNames       map[string]string
	questions       map[string]bool
	scopeExpansions map[string]bool
}

func NewBus(runID string, threadID string, store Store) *Bus {
	return &Bus{
		runID: runID, threadID: threadID, store: store,
		subscribers:          map[int]chan model.Event{},
		answeredInteractions: map[string]bool{}, publishedTransitions: map[string]bool{}, planCompleted: map[string]bool{}, toolCalls: map[string]bool{},
		toolNames: map[string]string{}, questions: map[string]bool{},
		scopeExpansions: map[string]bool{},
	}
}

// Restore rebuilds the in-memory sequence guards from durable public events.
// It does not append history or notify subscribers; subsequent events continue
// from the persisted sequence instead of emitting a second run.started.
func (b *Bus) Restore(events []model.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.seq != 0 || b.started || b.terminated {
		return errors.New("bus has already started")
	}
	for _, event := range events {
		if event.RunID != b.runID || event.Seq <= b.seq {
			return errors.New("persisted run events are not ordered")
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(event.Payload), &data); err != nil {
			return err
		}
		if err := b.validateSequence(event.Type, data); err != nil {
			return err
		}
		b.seq = event.Seq
		b.recordSequence(event.Type, data)
	}
	if len(events) > 0 && !b.started {
		return errors.New("persisted run history is missing run.started")
	}
	return nil
}

// Emit 分配下一个 seq，持久化后扇出。终态事件（done/error）只允许发一次。
func (b *Bus) Emit(ctx context.Context, evt model.EventType, payload any) error {
	if err := model.ValidatePublicEvent(evt, payload); err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if data["run_id"] != b.runID {
		return errors.New("public event run_id does not match bus")
	}

	b.mu.Lock()
	if b.terminated {
		b.mu.Unlock()
		return nil
	}
	if !b.started && evt != model.EventRunStarted {
		b.mu.Unlock()
		return errors.New("run.started must be the first public event")
	}
	if b.started && evt == model.EventRunStarted {
		b.mu.Unlock()
		return errors.New("run.started may only be emitted once")
	}
	if key := answeredInteractionKey(evt, data); key != "" && b.answeredInteractions[key] {
		b.mu.Unlock()
		return nil
	}
	if key := transitionPublicationKey(evt, data); key != "" && b.publishedTransitions[key] {
		b.mu.Unlock()
		return nil
	}
	if err := b.validateSequence(evt, data); err != nil {
		b.mu.Unlock()
		return err
	}
	nextSeq := b.seq + 1
	e := model.Event{
		RunID:     b.runID,
		Seq:       nextSeq,
		Type:      evt,
		Payload:   string(raw),
		CreatedAt: time.Now().UnixMilli(),
	}
	subs := make([]chan model.Event, 0, len(b.subscribers))
	for _, ch := range b.subscribers {
		subs = append(subs, ch)
	}

	// 先持久化再扇出：保证断线重连能从 store 补齐（ARCH-RUN-005）。
	if b.execution > 0 {
		ctx = workflow.WithCheckpointLease(ctx, b.owner, b.execution, 0)
	}
	if err := b.store.AppendEvent(ctx, &e); err != nil {
		b.mu.Unlock()
		return err
	}
	b.seq = e.Seq
	b.recordSequence(evt, data)
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
	b.mu.Unlock()
	return nil
}

func (b *Bus) validateSequence(evt model.EventType, data map[string]any) error {
	switch evt {
	case model.EventPlanUpdated:
		plan, _ := data["plan"].(map[string]any)
		planID, _ := plan["plan_id"].(string)
		if b.planID != "" && planID != b.planID {
			return errors.New("plan_id cannot change")
		}

		nextCompleted := map[string]bool{}
		steps, _ := plan["steps"].([]any)
		for _, raw := range steps {
			step, _ := raw.(map[string]any)
			id, _ := step["id"].(string)
			completed := step["status"] == "completed"
			nextCompleted[id] = completed
		}
		for id := range b.planCompleted {
			if !nextCompleted[id] {
				return errors.New("completed plan steps cannot regress or disappear")
			}
		}
	case model.EventToolStarted:
		callID, _ := data["call_id"].(string)
		if _, exists := b.toolCalls[callID]; exists {
			return errors.New("tool call_id must be unique")
		}
	case model.EventToolCompleted:
		callID, _ := data["call_id"].(string)
		completed, exists := b.toolCalls[callID]
		if !exists || completed {
			return errors.New("tool.completed must match tool.started")
		}
		if b.toolNames[callID] != data["tool"] {
			return errors.New("tool.completed tool must match tool.started")
		}
	case model.EventQuestionAsked:
		for _, answered := range b.questions {
			if !answered {
				return errors.New("only one question may be pending")
			}
		}
	case model.EventQuestionAnswered:
		questionID, _ := data["question_id"].(string)
		answered, exists := b.questions[questionID]
		if !exists || answered {
			return errors.New("question.answered must match a pending question")
		}
	case model.EventScopeExpansionRequested:
		for _, answered := range b.scopeExpansions {
			if !answered {
				return errors.New("only one scope expansion may be pending")
			}
		}
	case model.EventScopeExpansionAnswered:
		interactionID, _ := data["interaction_id"].(string)
		answered, exists := b.scopeExpansions[interactionID]
		if !exists || answered {
			return errors.New("scope.expansion_answered must match a pending request")
		}
	case model.EventMessageFinal:
		if b.cancelRequested {
			return errors.New("canceled run cannot emit message.final")
		}
		if b.finalCount != 0 {
			return errors.New("message.final may only be emitted once")
		}
	case model.EventRunCompleted, model.EventRunFailed, model.EventRunCanceled:
		if b.cancelRequested && evt == model.EventRunCompleted {
			return errors.New("cancel-requested run cannot complete")
		}
		if evt == model.EventRunCompleted && b.finalCount != 1 {
			return errors.New("completed run requires exactly one prior message.final")
		}
		allowInterruptedTools := evt == model.EventRunCanceled &&
			data["reason"] == string(model.RunCancelSuperseded)
		if !allowInterruptedTools {
			for _, completed := range b.toolCalls {
				if !completed {
					return errors.New("terminal run event requires every started tool to complete")
				}
			}
		}
	case model.EventRunError:
		if b.cancelRequested {
			return errors.New("cancel-requested run cannot emit runtime error")
		}
	}
	return nil
}

func (b *Bus) recordSequence(evt model.EventType, data map[string]any) {
	if key := answeredInteractionKey(evt, data); key != "" {
		b.answeredInteractions[key] = true
	}
	if key := transitionPublicationKey(evt, data); key != "" {
		b.publishedTransitions[key] = true
	}
	switch evt {
	case model.EventRunStarted:
		b.started = true
	case model.EventRunResumed:
		// A resume event starts a new process attempt. Tool calls left open by
		// the interrupted process are abandoned. Forget their active identity
		// so a provider may safely replay the same call_id from the checkpoint.
		for callID, completed := range b.toolCalls {
			if !completed {
				delete(b.toolCalls, callID)
				delete(b.toolNames, callID)
			}
		}
	case model.EventPlanUpdated:
		plan, _ := data["plan"].(map[string]any)
		b.planID, _ = plan["plan_id"].(string)
		steps, _ := plan["steps"].([]any)
		for _, raw := range steps {
			step, _ := raw.(map[string]any)
			if step["status"] == "completed" {
				id, _ := step["id"].(string)
				b.planCompleted[id] = true
			}
		}
	case model.EventToolStarted:
		callID, _ := data["call_id"].(string)
		b.toolCalls[callID] = false
		b.toolNames[callID], _ = data["tool"].(string)
	case model.EventToolCompleted:
		callID, _ := data["call_id"].(string)
		b.toolCalls[callID] = true
	case model.EventQuestionAsked:
		questionID, _ := data["question_id"].(string)
		b.questions[questionID] = false
	case model.EventQuestionAnswered:
		questionID, _ := data["question_id"].(string)
		b.questions[questionID] = true
	case model.EventScopeExpansionRequested:
		interactionID, _ := data["interaction_id"].(string)
		b.scopeExpansions[interactionID] = false
	case model.EventScopeExpansionAnswered:
		interactionID, _ := data["interaction_id"].(string)
		b.scopeExpansions[interactionID] = true
	case model.EventMessageFinal:
		b.finalCount++
	case model.EventRunCompleted:
		b.terminated = true
		b.terminalStatus = "completed"
	case model.EventRunFailed:
		b.terminated = true
		b.terminalStatus = "failed"
	case model.EventRunError:
		b.terminated = true
		b.terminalStatus = "error"
	case model.EventRunCanceled:
		b.terminated = true
		b.terminalStatus = "canceled"
	}
}

// Subscribe 注册一个实时订阅者，返回事件 channel 与取消函数。
func (b *Bus) Subscribe() (<-chan model.Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextSubID
	b.nextSubID++
	ch := make(chan model.Event, 64)
	b.subscribers[id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if sub, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(sub)
		}
	}
}

// Close 关闭总线并释放所有订阅者。
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, ch := range b.subscribers {
		delete(b.subscribers, id)
		close(ch)
	}
}

func (b *Bus) Terminated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.terminated
}

// RequestCancelAuthority serializes cancel against the terminal event. When it
// returns false, the terminal event won the race and its status is authoritative.
func (b *Bus) RequestCancelAuthority() (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.terminated {
		return false, b.terminalStatus
	}
	b.cancelRequested = true
	return true, ""
}

func answeredInteractionKey(evt model.EventType, data map[string]any) string {
	switch evt {
	case model.EventQuestionAnswered, model.EventPlanApprovalAnswered, model.EventCommandPermissionAnswered, model.EventScopeExpansionAnswered:
		id, _ := data["interaction_id"].(string)
		if id == "" {
			id, _ = data["question_id"].(string)
		}
		if id != "" {
			return string(evt) + ":" + id
		}
	}
	return ""
}

// A committed approval can be replayed after its public transition was already
// delivered but before the checkpoint cleared its publication marker. These
// keys describe the transition itself, excluding the new event timestamp.
func transitionPublicationKey(evt model.EventType, data map[string]any) string {
	switch evt {
	case model.EventPlanUpdated:
		if id, _ := data["interaction_id"].(string); id != "" {
			return "approved-plan:" + id
		}
	case model.EventRunModeChanged:
		if data["previous_mode"] == string(model.ModePlan) && data["mode"] == string(model.ModeExecute) {
			return "plan-to-execute"
		}
	case model.EventScopeUpdated:
		if data["cause"] == "user_approved_expansion" {
			id, _ := data["interaction_id"].(string)
			if id != "" {
				return "approved-scope:" + id
			}
		}
	}
	return ""
}
