package run

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// memStore 是 run.Store 的内存实现，供 engine 测试用（不打真实 SQLite）。
type memStore struct {
	mu     sync.Mutex
	runs   map[string]model.Run
	events map[string][]model.Event
}

func newMemStore() *memStore {
	return &memStore{runs: map[string]model.Run{}, events: map[string][]model.Event{}}
}

func (s *memStore) CreateRun(_ context.Context, r model.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[r.ID] = r
	return nil
}

func (s *memStore) GetRun(_ context.Context, id string) (model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return model.Run{}, ErrRunNotFound
	}
	return r, nil
}

func (s *memStore) SetRunStatus(_ context.Context, id string, status model.RunStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return ErrRunNotFound
	}
	r.Status = status
	s.runs[id] = r
	return nil
}

func (s *memStore) AppendEvent(_ context.Context, e model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[e.RunID] = append(s.events[e.RunID], e)
	return nil
}

func (s *memStore) EventsSince(_ context.Context, runID string, afterSeq int64) ([]model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Event
	for _, e := range s.events[runID] {
		if e.Seq > afterSeq {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *memStore) status(id string) model.RunStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[id].Status
}

// scriptRunner 是可控 runner：按脚本发事件，可选阻塞等待 ctx/信号。
type scriptRunner struct {
	fn func(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, p Prompter) harness.Outcome
}

func (r scriptRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, p Prompter) harness.Outcome {
	return r.fn(ctx, em, cp, p)
}

func newEngine() (*Engine, *memStore) {
	st := newMemStore()
	return NewEngine(st, NewLockManager(), nil, zap.NewNop()), st
}

// drainEvents 收集 SSE 事件直到 channel 关闭。
func drainEvents(ch <-chan model.Event) []model.Event {
	var out []model.Event
	for e := range ch {
		out = append(out, e)
	}
	return out
}

// AC-GLOBAL-001 / AC-SSE-001：正常执行 → seq 连续递增，恰好一个 done。
func TestSSESequenceAndSingleDone(t *testing.T) {
	e, st := newEngine()
	runner := scriptRunner{fn: func(ctx context.Context, em harness.Emitter, _ harness.Checkpointer, _ Prompter) harness.Outcome {
		em.Emit(model.EventProgress, harness.ProgressPayload{Stage: "turn", Current: 1, Total: 3})
		em.Emit(model.EventThought, harness.ThoughtPayload{Text: "thinking"})
		return harness.Outcome{Status: harness.OutcomeFinished, Summary: "ok"}
	}}

	r, err := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// 等待执行完成。
	waitStatus(t, st, r.ID, model.RunDone)

	ch, stop, err := e.Subscribe(context.Background(), r.ID, 0)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer stop()
	events := drainEvents(ch)

	assertSeqContiguous(t, events)
	assertExactlyOneDone(t, events)
	// 至少含一个 progress（AC-GLOBAL-001）。
	if countType(events, model.EventProgress) < 1 {
		t.Error("expected at least one progress event")
	}
}

// AC-V2-CTR-003：plan/plan.update 走 run_events 持久化并参与 Last-Event-ID 续传，无重复。
func TestPlanEventsPersistAndResume(t *testing.T) {
	e, st := newEngine()
	runner := scriptRunner{fn: func(_ context.Context, em harness.Emitter, _ harness.Checkpointer, _ Prompter) harness.Outcome {
		em.Emit(model.EventPlan, harness.PlanPayload{ID: "plan_r1", Title: "构建 2 页", Steps: []harness.PlanStepPayload{
			{ID: "p0", Title: "第 1 页", Status: "pending"},
			{ID: "p1", Title: "第 2 页", Status: "pending"},
		}})
		em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: "plan_r1", StepID: "p0", Status: "in_progress"})
		em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: "plan_r1", StepID: "p0", Status: "completed"})
		return harness.Outcome{Status: harness.OutcomeFinished}
	}}
	r, _ := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)
	waitStatus(t, st, r.ID, model.RunDone)

	// 全量订阅：恰好一个 plan，两个 plan.update，且非终态不改变"恰好一个 done"。
	ch, stop, err := e.Subscribe(context.Background(), r.ID, 0)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	all := drainEvents(ch)
	stop()
	if countType(all, model.EventPlan) != 1 {
		t.Errorf("want exactly 1 plan, got %d", countType(all, model.EventPlan))
	}
	if countType(all, model.EventPlanUpdate) != 2 {
		t.Errorf("want 2 plan.update, got %d", countType(all, model.EventPlanUpdate))
	}
	assertSeqContiguous(t, all)
	assertExactlyOneDone(t, all)

	// plan 的 seq，用于验证续传跳过。
	var planSeq int64
	for _, ev := range all {
		if ev.Type == model.EventPlan {
			planSeq = ev.Seq
		}
	}

	// 从 plan 之后续订：不再重复收到 plan，只收到后续 plan.update/done。
	ch2, stop2, err := e.Subscribe(context.Background(), r.ID, planSeq)
	if err != nil {
		t.Fatalf("resume subscribe: %v", err)
	}
	defer stop2()
	resumed := drainEvents(ch2)
	for _, ev := range resumed {
		if ev.Seq <= planSeq {
			t.Errorf("resume must skip seq<=%d, got %d", planSeq, ev.Seq)
		}
		if ev.Type == model.EventPlan {
			t.Error("resume must not re-deliver plan (no duplicate)")
		}
	}
	if countType(resumed, model.EventPlanUpdate) != 2 {
		t.Errorf("resume should still carry 2 plan.update, got %d", countType(resumed, model.EventPlanUpdate))
	}
}

// AC-SSE-003：带 Last-Event-ID 续传，收到 seq>N 的事件，无重复。
func TestSSEResume(t *testing.T) {
	e, st := newEngine()
	runner := scriptRunner{fn: func(_ context.Context, em harness.Emitter, _ harness.Checkpointer, _ Prompter) harness.Outcome {
		em.Emit(model.EventProgress, harness.ProgressPayload{Current: 1})
		em.Emit(model.EventProgress, harness.ProgressPayload{Current: 2})
		em.Emit(model.EventInfo, harness.InfoPayload{Text: "x"})
		return harness.Outcome{Status: harness.OutcomeFinished}
	}}
	r, _ := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)
	waitStatus(t, st, r.ID, model.RunDone)

	// 从 seq=2 之后续订。
	ch, stop, err := e.Subscribe(context.Background(), r.ID, 2)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer stop()
	events := drainEvents(ch)

	for _, ev := range events {
		if ev.Seq <= 2 {
			t.Errorf("resume must skip seq<=2, got %d", ev.Seq)
		}
	}
	if len(events) == 0 {
		t.Error("expected events after seq 2")
	}
}

// AC-RUN-API-001：done 的 Run 注入 input → ErrRunNotRunning（409）。
func TestInjectInputOnDoneRun(t *testing.T) {
	e, st := newEngine()
	runner := scriptRunner{fn: func(_ context.Context, _ harness.Emitter, _ harness.Checkpointer, _ Prompter) harness.Outcome {
		return harness.Outcome{Status: harness.OutcomeFinished}
	}}
	r, _ := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)
	waitStatus(t, st, r.ID, model.RunDone)

	err := e.InjectInput(context.Background(), r.ID, "hi", "")
	if err != ErrRunNotRunning {
		t.Fatalf("want ErrRunNotRunning, got %v", err)
	}
}

// AC-RUN-API-004：waiting 状态收到匹配 reply_to → 转回 running。
func TestNeedsInputReplyResumesRunning(t *testing.T) {
	e, st := newEngine()
	replied := make(chan string, 1)
	runner := scriptRunner{fn: func(ctx context.Context, _ harness.Emitter, _ harness.Checkpointer, p Prompter) harness.Outcome {
		ans, err := p.NeedsInput(ctx, "evt_x", "需要更多信息？", []string{"a", "b"})
		if err != nil {
			return harness.Outcome{Status: harness.OutcomeCanceled}
		}
		replied <- ans
		return harness.Outcome{Status: harness.OutcomeFinished}
	}}
	r, _ := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)

	// 等待进入 waiting。
	waitStatus(t, st, r.ID, model.RunWaiting)

	// 应答 → 应转回 running 并最终 done。
	if err := e.InjectInput(context.Background(), r.ID, "答案", "evt_x"); err != nil {
		t.Fatalf("reply: %v", err)
	}
	select {
	case ans := <-replied:
		if ans != "答案" {
			t.Errorf("bad reply content: %q", ans)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not resume after reply")
	}
	waitStatus(t, st, r.ID, model.RunDone)
}

// API-RUN-003：reply_to 未匹配任何未应答 needs_input → ErrReplyMismatch。
func TestReplyMismatch(t *testing.T) {
	e, st := newEngine()
	proceed := make(chan struct{})
	runner := scriptRunner{fn: func(ctx context.Context, _ harness.Emitter, _ harness.Checkpointer, _ Prompter) harness.Outcome {
		<-proceed
		return harness.Outcome{Status: harness.OutcomeFinished}
	}}
	r, _ := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)
	waitStatus(t, st, r.ID, model.RunRunning)

	err := e.InjectInput(context.Background(), r.ID, "x", "nonexistent")
	if err != ErrReplyMismatch {
		close(proceed)
		t.Fatalf("want ErrReplyMismatch, got %v", err)
	}
	close(proceed)
	waitStatus(t, st, r.ID, model.RunDone)
}

// AC-RUN-API-005：取消 running Run → 转 canceled（已落盘产物保留：事件仍在 store）。
func TestCancelKeepsArtifacts(t *testing.T) {
	e, st := newEngine()
	started := make(chan struct{})
	runner := scriptRunner{fn: func(ctx context.Context, em harness.Emitter, _ harness.Checkpointer, _ Prompter) harness.Outcome {
		em.Emit(model.EventArtifact, harness.ArtifactPayload{ArtifactType: "file", Ref: "slides/000/index.html"})
		close(started)
		<-ctx.Done() // 阻塞直到取消
		return harness.Outcome{Status: harness.OutcomeCanceled}
	}}
	r, _ := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)
	<-started

	if err := e.Cancel(context.Background(), r.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	waitStatus(t, st, r.ID, model.RunCanceled)

	// 已落盘 artifact 事件保留。
	evs, _ := st.EventsSince(context.Background(), r.ID, 0)
	if countType(evs, model.EventArtifact) != 1 {
		t.Error("artifact event must be retained after cancel")
	}
}

// AC-RUN-006：talk 模式（此处以「runner 不发 artifact」验证外壳不强加 artifact）。
func TestTalkNoArtifact(t *testing.T) {
	e, st := newEngine()
	runner := scriptRunner{fn: func(_ context.Context, em harness.Emitter, _ harness.Checkpointer, _ Prompter) harness.Outcome {
		em.Emit(model.EventInfo, harness.InfoPayload{Text: "分析：建议用深色"})
		return harness.Outcome{Status: harness.OutcomeFinished}
	}}
	r, _ := e.Start(context.Background(), model.Run{ID: "r1", ProjectID: "p1"}, runner)
	waitStatus(t, st, r.ID, model.RunDone)

	evs, _ := st.EventsSince(context.Background(), r.ID, 0)
	if countType(evs, model.EventArtifact) != 0 {
		t.Error("talk mode must not emit artifact")
	}
	if countType(evs, model.EventInfo) < 1 {
		t.Error("expected info event")
	}
}

// ── helpers ─────────────────────────────────────────────

func waitStatus(t *testing.T, st *memStore, id string, want model.RunStatus) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		if st.status(id) == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("run %s did not reach %s (now %s)", id, want, st.status(id))
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func assertSeqContiguous(t *testing.T, events []model.Event) {
	t.Helper()
	for i, e := range events {
		want := int64(i + 1)
		if e.Seq != want {
			t.Errorf("seq not contiguous at %d: want %d got %d", i, want, e.Seq)
		}
	}
}

func assertExactlyOneDone(t *testing.T, events []model.Event) {
	t.Helper()
	terminals := 0
	for _, e := range events {
		if e.Type.Terminal() {
			terminals++
		}
	}
	if terminals != 1 {
		t.Errorf("want exactly one terminal event, got %d", terminals)
	}
	if len(events) > 0 && !events[len(events)-1].Type.Terminal() {
		t.Error("terminal event must be last")
	}
}

func countType(events []model.Event, typ model.EventType) int {
	n := 0
	for _, e := range events {
		if e.Type == typ {
			n++
		}
	}
	return n
}
