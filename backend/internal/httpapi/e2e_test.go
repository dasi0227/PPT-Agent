package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// harnessRunner 用真实 harness.Loop + fake LLM 跑一次工具化 ReAct 循环（端到端 demo）。
type harnessRunner struct {
	client llm.Client
}

func (r harnessRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, _ run.Prompter) harness.Outcome {
	loop := harness.New(r.client, harness.Config{
		RunID: "e2e", Kind: model.KindCommand, Scope: model.ScopeCurrent, Mode: model.ModeNormal,
		SystemPrompt: "system", Instruction: "do it",
		Tools: []tools.Tool{tools.NewFinishTool()},
	})
	return loop.Run(ctx, em, cp)
}

func setupServer(t *testing.T, client llm.Client, runner run.Runner) (*httptest.Server, string) {
	t.Helper()
	cfg := &config.Config{DBPath: filepath.Join(t.TempDir(), "e2e.db")}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	// seed project + thread（Run 需挂在 thread 下）。
	now := time.Now().Unix()
	if err := st.CreateProject(context.Background(), model.Project{ID: "p1", Title: "t", WorkDir: "/tmp/p1", Theme: "default", Status: "draft", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := st.CreateThread(context.Background(), model.Thread{ID: "th1", ProjectID: "p1", HistoryPath: "threads/th1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("seed thread: %v", err)
	}

	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	// 用注入的 runner 替换默认 demo runner，便于端到端断言。
	runSvc := service.NewRunServiceWithFactory(st, engine, func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner {
		return runner
	})
	router := httpapi.NewRouter(cfg, zap.NewNop(), httpapi.NewHealthHandler(service.NewHealthService(st)), httpapi.NewRunHandler(runSvc), httpapi.NewSlideHandler(service.NewSlideService(st)))

	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, "th1"
}

// AC-GLOBAL-001：发起生成 Run → SSE 收到 ≥1 个 progress 和一个 done。
func TestE2ERunSSEProgressAndDone(t *testing.T) {
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		{Thought: "finish now", ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}},
	}}
	srv, threadID := setupServer(t, fake, harnessRunner{client: fake})

	runID := createRun(t, srv, threadID)
	events := readSSE(t, srv, runID, "", 0)

	if countEvent(events, "progress") < 1 {
		t.Errorf("want >=1 progress, got events: %v", eventTypes(events))
	}
	if countEvent(events, "done") != 1 {
		t.Errorf("want exactly 1 done, got events: %v", eventTypes(events))
	}
}

// AC-GLOBAL-002：running Run 注入 input → 202 Accepted。
func TestE2EInjectInputAccepted(t *testing.T) {
	gate := make(chan struct{})
	blocking := scriptRunnerE2E{fn: func(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, p run.Prompter) harness.Outcome {
		em.Emit(model.EventProgress, harness.ProgressPayload{Stage: "turn", Current: 1, Total: 3})
		close(gate)
		// 排空控制输入前先等一会（模拟进行中的调用）。
		select {
		case <-ctx.Done():
			return harness.Outcome{Status: harness.OutcomeCanceled}
		case <-time.After(500 * time.Millisecond):
		}
		_ = cp.DrainInputs()
		return harness.Outcome{Status: harness.OutcomeFinished}
	}}
	srv, threadID := setupServer(t, &llmtest.FakeClient{}, blocking)

	runID := createRun(t, srv, threadID)
	<-gate // 确保已 running

	code := postInput(t, srv, runID, `{"content":"用深色主题"}`)
	if code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", code)
	}
}

// AC-SSE-003：SSE over HTTP 带 Last-Event-ID 续传，无重复 seq≤N。
func TestE2ESSEResumeOverHTTP(t *testing.T) {
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}},
	}}
	srv, threadID := setupServer(t, fake, harnessRunner{client: fake})

	runID := createRun(t, srv, threadID)
	// 首次完整读取，拿到全部事件与最大 seq。
	all := readSSE(t, srv, runID, "", 0)
	if len(all) < 2 {
		t.Fatalf("expected multiple events, got %d", len(all))
	}
	// 带 Last-Event-ID=1 续订（run 已结束，从 store 补发历史）。
	resumed := readSSE(t, srv, runID, "1", 0)
	for _, e := range resumed {
		if e.id == "1" {
			t.Errorf("resume must not repeat seq<=1, got id=%s", e.id)
		}
	}
	if len(resumed) == 0 {
		t.Error("expected resumed events after seq 1")
	}
}

// scriptRunnerE2E 让端到端测试注入自定义 runner 逻辑。
type scriptRunnerE2E struct {
	fn func(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, p run.Prompter) harness.Outcome
}

func (r scriptRunnerE2E) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, p run.Prompter) harness.Outcome {
	return r.fn(ctx, em, cp, p)
}

// ── HTTP helpers ────────────────────────────────────────

func createRun(t *testing.T, srv *httptest.Server, threadID string) string {
	t.Helper()
	body := `{"kind":"generate","scope":"current","mode":"normal","instruction":"生成8页"}`
	resp, err := http.Post(srv.URL+"/api/v1/threads/"+threadID+"/runs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create run status %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if out.ID == "" {
		t.Fatal("empty run id")
	}
	return out.ID
}

func postInput(t *testing.T, srv *httptest.Server, runID, body string) int {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/v1/runs/"+runID+"/input", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post input: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

type sseEvent struct {
	id, event, data string
}

// readSSE 读取 SSE 流直到终态事件或超时。lastEventID 非空时带续传头。
func readSSE(t *testing.T, srv *httptest.Server, runID, lastEventID string, _ int) []sseEvent {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/runs/"+runID+"/events", nil)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse get: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
	// AC-SSE-001 / ARCH-BACKEND-005：SSE 端点 MUST NOT 被 gzip/缓冲中间件包裹，
	// 否则事件不能及时到达客户端。锁定当前中间件栈（RequestID/Recover/Log）不引入 Content-Encoding。
	if enc := resp.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("SSE response MUST NOT be encoded (e.g. gzip), got Content-Encoding=%q", enc)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("SSE response MUST set Cache-Control: no-cache, got %q", cc)
	}

	var events []sseEvent
	var cur sseEvent
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "id:"):
			cur.id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "event:"):
			cur.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			cur.data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "":
			if cur.event != "" {
				events = append(events, cur)
				if cur.event == "done" || cur.event == "error" {
					return events
				}
			}
			cur = sseEvent{}
		}
	}
	return events
}

func countEvent(events []sseEvent, typ string) int {
	n := 0
	for _, e := range events {
		if e.event == typ {
			n++
		}
	}
	return n
}

func eventTypes(events []sseEvent) string {
	var b bytes.Buffer
	for i, e := range events {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%s#%s", e.event, e.id)
	}
	return b.String()
}
