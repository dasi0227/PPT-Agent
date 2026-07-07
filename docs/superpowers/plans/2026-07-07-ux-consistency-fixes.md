# UX 一致性修复 · 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 一次性修复用户反馈的 7 个 UX/一致性问题（输入回显、会话历史持久化、自动刷新、命名规范、全局大纲/HTML 切换、网格空态、Markdown 渲染）。

**Architecture:** 后端在 `run.Bus` 层集中把白名单 SSE 事件 append 到 `history.jsonl`；前端 `runStore` 用生命周期钩子在 `createRun` 头部插入 `user_turn`、在 `done|error` 时自动 `loadProjectSlides`、通过 `hydrateTimeline` 支持刷新回放；`deckStore` 引入全局 `globalView`；Timeline 用 `MarkdownMessage` 统一文本渲染。

**Tech Stack:** Go 1.22 / Gin / SQLite / GORM · React 18 / Zustand / Vite / Vitest · SSE (EventSource) · react-markdown + remark-gfm

## Global Constraints

- 后端提交前缀：`feat(ux-consistency):`（M6 提交约定沿用）
- 后端储存根：`~/.dasi/ppt`；`history.jsonl` 位于 `<workdir>/threads/<thread_id>.jsonl`
- TDD 严格：写失败测试 → 确认失败 → 最小实现 → 确认通过 → 提交
- 后端全量测试命令：`go test ./...`（cwd = `backend/`）
- 前端测试命令：`npm test -- --run`（cwd = `frontend/`），单文件 `npm test -- --run <path>`
- history append 失败绝不阻塞 SSE 主流程（log + drop）
- SSE 白名单：`run.started / token / info / needs_input / done / error`；其余不落盘
- 前端命名："未命名 N"（正则 `^未命名 (\d+)$`），删除不回收序号
- 全局视图默认值：`globalView='html'`
- Markdown 渲染范围：user_turn / markdown / info / thought / final_result(string) / needs_input.prompt；tool_call / artifact / error 保留结构化

---

## 阶段 1 · 后端 · history 持久化（问题 #2）

### Task 1.1: 扩展 RunStartedPayload 携带 user_input

**Files:**
- Modify: `backend/internal/harness/events.go`
- Modify: `backend/internal/harness/loop.go:47-53`
- Modify: `backend/internal/agent/assist/prompt_runner.go:30-34`（或等价位置）
- Modify: `backend/internal/agent/assist/recap_runner.go:27-31`（或等价位置）

**Interfaces:**
- Produces: `RunStartedPayload.UserInput string json:"user_input,omitempty"`（供 bus 转 history_writer）

- [ ] **Step 1: 写失败测试**

新增 `backend/internal/harness/events_test.go`（若不存在则创建）：

```go
package harness

import (
    "encoding/json"
    "testing"
)

func TestRunStartedPayloadCarriesUserInput(t *testing.T) {
    p := RunStartedPayload{RunID: "r1", Kind: "outline", Scope: "current", Mode: "normal", UserInput: "hello"}
    raw, err := json.Marshal(p)
    if err != nil {
        t.Fatal(err)
    }
    var got map[string]any
    _ = json.Unmarshal(raw, &got)
    if got["user_input"] != "hello" {
        t.Fatalf("expected user_input=hello, got %v", got["user_input"])
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/harness/ -run TestRunStartedPayloadCarriesUserInput -v`
Expected: FAIL（字段不存在 / 结构体没有该字段）。

- [ ] **Step 3: 加字段**

修改 [events.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/harness/events.go) 的 `RunStartedPayload`：

```go
type RunStartedPayload struct {
    RunID     string `json:"run_id"`
    Kind      string `json:"kind"`
    Scope     string `json:"scope"`
    Mode      string `json:"mode"`
    UserInput string `json:"user_input,omitempty"`
}
```

- [ ] **Step 4: 三处 emit 补 UserInput**

`backend/internal/harness/loop.go` 第 48-53 行：

```go
em.Emit(model.EventRunStarted, RunStartedPayload{
    RunID:     l.cfg.RunID,
    Kind:      string(l.cfg.Kind),
    Scope:     string(l.cfg.Scope),
    Mode:      string(l.cfg.Mode),
    UserInput: l.cfg.Instruction,
})
```

`backend/internal/agent/assist/prompt_runner.go`、`recap_runner.go` 中 `harness.RunStartedPayload{ ... }` 字面量同样补 `UserInput: <instruction>`（这两个 Runner 的 instruction 变量名可能不同，扫描函数签名找到对应字段即可）。

- [ ] **Step 5: 运行全量测试**

Run: `cd backend && go test ./...`
Expected: PASS（新测试通过；既有测试不受影响，因 `UserInput` 是新增字段且 `omitempty`）。

- [ ] **Step 6: 提交**

```bash
cd backend
git add internal/harness/events.go internal/harness/events_test.go internal/harness/loop.go internal/agent/assist/prompt_runner.go internal/agent/assist/recap_runner.go
git commit -m "feat(ux-consistency): carry user_input in run.started payload"
```

---

### Task 1.2: 新增 HistoryWriter 接口 + FS 实现

**Files:**
- Create: `backend/internal/run/history_writer.go`
- Create: `backend/internal/run/history_writer_test.go`

**Interfaces:**
- Produces:
  ```go
  type HistoryEntry struct {
      Seq   int64          `json:"seq"`
      TS    int64          `json:"ts"`
      RunID string         `json:"run_id"`
      Turn  string         `json:"turn"`   // "user" | "agent"
      Type  string         `json:"type"`   // "user_turn" | "markdown" | "info" | "needs_input" | "final_result" | "error"
      Data  map[string]any `json:"data"`
  }
  type HistoryWriter interface {
      Append(ctx context.Context, threadID string, entry HistoryEntry) error
  }
  type ThreadLocator interface {
      GetThread(ctx context.Context, id string) (model.Thread, error)
      GetProject(ctx context.Context, id string) (model.Project, error)
  }
  func NewFSHistoryWriter(loc ThreadLocator) *FSHistoryWriter
  ```

- [ ] **Step 1: 写失败测试**

新建 `backend/internal/run/history_writer_test.go`：

```go
package run

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "testing"

    "github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type fakeLoc struct {
    proj model.Project
    thr  model.Thread
}

func (f *fakeLoc) GetThread(_ context.Context, id string) (model.Thread, error) {
    if f.thr.ID != id {
        return model.Thread{}, os.ErrNotExist
    }
    return f.thr, nil
}
func (f *fakeLoc) GetProject(_ context.Context, id string) (model.Project, error) {
    if f.proj.ID != id {
        return model.Project{}, os.ErrNotExist
    }
    return f.proj, nil
}

func TestFSHistoryWriter_AppendCreatesFile(t *testing.T) {
    dir := t.TempDir()
    if err := os.MkdirAll(filepath.Join(dir, "threads"), 0o755); err != nil {
        t.Fatal(err)
    }
    loc := &fakeLoc{
        proj: model.Project{ID: "p1", WorkDir: dir},
        thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
    }
    w := NewFSHistoryWriter(loc)
    err := w.Append(context.Background(), "t1", HistoryEntry{Seq: 1, TS: 100, RunID: "r1", Turn: "user", Type: "user_turn", Data: map[string]any{"text": "hi"}})
    if err != nil {
        t.Fatal(err)
    }
    raw, err := os.ReadFile(filepath.Join(dir, "threads/t1.jsonl"))
    if err != nil {
        t.Fatal(err)
    }
    line := strings.TrimSpace(string(raw))
    var got HistoryEntry
    if err := json.Unmarshal([]byte(line), &got); err != nil {
        t.Fatal(err)
    }
    if got.Seq != 1 || got.Turn != "user" || got.Data["text"] != "hi" {
        t.Fatalf("unexpected entry: %+v", got)
    }
}

func TestFSHistoryWriter_ConcurrentSameThread(t *testing.T) {
    dir := t.TempDir()
    _ = os.MkdirAll(filepath.Join(dir, "threads"), 0o755)
    loc := &fakeLoc{
        proj: model.Project{ID: "p1", WorkDir: dir},
        thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
    }
    w := NewFSHistoryWriter(loc)
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(seq int) {
            defer wg.Done()
            _ = w.Append(context.Background(), "t1", HistoryEntry{Seq: int64(seq), Turn: "agent", Type: "info", Data: map[string]any{"text": "x"}})
        }(i)
    }
    wg.Wait()
    raw, _ := os.ReadFile(filepath.Join(dir, "threads/t1.jsonl"))
    lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
    if len(lines) != 50 {
        t.Fatalf("expected 50 lines, got %d", len(lines))
    }
    for _, l := range lines {
        var e HistoryEntry
        if err := json.Unmarshal([]byte(l), &e); err != nil {
            t.Fatalf("invalid json line: %s", l)
        }
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/run/ -run FSHistoryWriter -v`
Expected: FAIL（`NewFSHistoryWriter` / `HistoryEntry` / `HistoryWriter` 未定义）。

- [ ] **Step 3: 写实现**

新建 [backend/internal/run/history_writer.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/run/history_writer.go)：

```go
package run

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "sync"

    "github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type HistoryEntry struct {
    Seq   int64          `json:"seq"`
    TS    int64          `json:"ts"`
    RunID string         `json:"run_id"`
    Turn  string         `json:"turn"`
    Type  string         `json:"type"`
    Data  map[string]any `json:"data"`
}

type HistoryWriter interface {
    Append(ctx context.Context, threadID string, entry HistoryEntry) error
}

type ThreadLocator interface {
    GetThread(ctx context.Context, id string) (model.Thread, error)
    GetProject(ctx context.Context, id string) (model.Project, error)
}

// FSHistoryWriter 按 thread 独立 mutex 串行化 append 到 <workdir>/<history_path>。
type FSHistoryWriter struct {
    loc ThreadLocator

    mu    sync.Mutex
    locks map[string]*sync.Mutex
}

func NewFSHistoryWriter(loc ThreadLocator) *FSHistoryWriter {
    return &FSHistoryWriter{loc: loc, locks: map[string]*sync.Mutex{}}
}

func (w *FSHistoryWriter) mutexFor(threadID string) *sync.Mutex {
    w.mu.Lock()
    defer w.mu.Unlock()
    m, ok := w.locks[threadID]
    if !ok {
        m = &sync.Mutex{}
        w.locks[threadID] = m
    }
    return m
}

func (w *FSHistoryWriter) Append(ctx context.Context, threadID string, entry HistoryEntry) error {
    th, err := w.loc.GetThread(ctx, threadID)
    if err != nil {
        return err
    }
    proj, err := w.loc.GetProject(ctx, th.ProjectID)
    if err != nil {
        return err
    }
    full := filepath.Join(proj.WorkDir, th.HistoryPath)
    if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
        return err
    }
    raw, err := json.Marshal(entry)
    if err != nil {
        return err
    }
    raw = append(raw, '\n')

    m := w.mutexFor(threadID)
    m.Lock()
    defer m.Unlock()

    f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = f.Write(raw)
    return err
}
```

- [ ] **Step 4: 运行测试**

Run: `cd backend && go test ./internal/run/ -run FSHistoryWriter -v`
Expected: PASS（两个测试通过）。

- [ ] **Step 5: 提交**

```bash
cd backend
git add internal/run/history_writer.go internal/run/history_writer_test.go
git commit -m "feat(ux-consistency): FSHistoryWriter with per-thread mutex"
```

---

### Task 1.3: Bus 注入 HistoryWriter + Publish 白名单副作用

**Files:**
- Modify: `backend/internal/run/bus.go`
- Modify: `backend/internal/run/engine.go:49`（`NewBus` 调用点）
- Create test additions in: `backend/internal/run/bus_test.go`（若不存在则新建）

**Interfaces:**
- Consumes: `HistoryWriter`（Task 1.2）、`RunStartedPayload.UserInput`（Task 1.1）
- Produces: `NewBus(runID string, threadID string, store Store, hw HistoryWriter) *Bus`（构造函数签名变更）；`func isWhitelistedForHistory(evt model.EventType) bool`

- [ ] **Step 1: 写失败测试**

新建 [backend/internal/run/bus_test.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/run/bus_test.go)：

```go
package run

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "testing"

    "github.com/dasi0227/PPT-Agent/backend/internal/harness"
    "github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type memStore2 struct {
    mu sync.Mutex
    ev []model.Event
}

func (s *memStore2) AppendEvent(_ context.Context, e model.Event) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.ev = append(s.ev, e)
    return nil
}
func (s *memStore2) EventsSince(_ context.Context, runID string, seq int64) ([]model.Event, error) {
    return nil, nil
}
func (s *memStore2) CreateRun(_ context.Context, _ model.Run) error         { return nil }
func (s *memStore2) SetRunStatus(_ context.Context, _ string, _ model.RunStatus) error { return nil }
func (s *memStore2) GetRun(_ context.Context, _ string) (model.Run, error) {
    return model.Run{}, nil
}

func TestBusAppendsWhitelistedEventsToHistory(t *testing.T) {
    dir := t.TempDir()
    _ = os.MkdirAll(filepath.Join(dir, "threads"), 0o755)
    loc := &fakeLoc{
        proj: model.Project{ID: "p1", WorkDir: dir},
        thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
    }
    hw := NewFSHistoryWriter(loc)
    b := NewBus("r1", "t1", &memStore2{}, hw)

    _ = b.Emit(context.Background(), model.EventRunStarted, harness.RunStartedPayload{RunID: "r1", Kind: "outline", Scope: "current", Mode: "normal", UserInput: "hi"})
    _ = b.Emit(context.Background(), model.EventThought, harness.ThoughtPayload{Text: "thinking"}) // 不落
    _ = b.Emit(context.Background(), model.EventInfo, harness.InfoPayload{Text: "info line"})
    _ = b.Emit(context.Background(), model.EventDone, harness.DonePayload{Result: map[string]any{"ok": true}})

    raw, err := os.ReadFile(filepath.Join(dir, "threads/t1.jsonl"))
    if err != nil {
        t.Fatal(err)
    }
    lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
    if len(lines) != 3 { // run.started, info, done
        t.Fatalf("expected 3 lines, got %d: %q", len(lines), string(raw))
    }
    var e0 HistoryEntry
    if err := json.Unmarshal([]byte(lines[0]), &e0); err != nil {
        t.Fatal(err)
    }
    if e0.Turn != "user" || e0.Type != "user_turn" || e0.Data["text"] != "hi" {
        t.Fatalf("unexpected first entry: %+v", e0)
    }
    var e2 HistoryEntry
    _ = json.Unmarshal([]byte(lines[2]), &e2)
    if e2.Type != "final_result" || e2.Turn != "agent" {
        t.Fatalf("unexpected done entry: %+v", e2)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/run/ -run TestBusAppendsWhitelisted -v`
Expected: FAIL（`NewBus` 只接受 3 参 / 未调用 hw）。

- [ ] **Step 3: 修改 Bus 签名 + 加白名单 + emit 后 append**

修改 [bus.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/run/bus.go)：

```go
type Bus struct {
    runID    string
    threadID string
    store    Store
    hw       HistoryWriter

    mu          sync.Mutex
    seq         int64
    subscribers map[int]chan model.Event
    nextSubID   int
    closed      bool
    terminated  bool
}

func NewBus(runID string, threadID string, store Store, hw HistoryWriter) *Bus {
    return &Bus{runID: runID, threadID: threadID, store: store, hw: hw, subscribers: map[int]chan model.Event{}}
}

func isWhitelistedForHistory(evt model.EventType) bool {
    switch evt {
    case model.EventRunStarted, model.EventToken, model.EventInfo, model.EventNeedsInput, model.EventDone, model.EventError:
        return true
    }
    return false
}

// buildHistoryEntry 把 SSE 事件映射成 history schema。
// - run.started → turn=user, type=user_turn, data={text: user_input}
// - token/info  → turn=agent, type=markdown, data={text: <raw text>}
// - needs_input → turn=agent, type=needs_input, data=<raw>
// - done        → turn=agent, type=final_result, data=<raw>
// - error       → turn=agent, type=error, data=<raw>
func buildHistoryEntry(e model.Event) (HistoryEntry, bool) {
    var data map[string]any
    _ = json.Unmarshal([]byte(e.Payload), &data)
    entry := HistoryEntry{Seq: e.Seq, TS: e.CreatedAt, RunID: e.RunID, Data: data}
    switch e.Type {
    case model.EventRunStarted:
        entry.Turn = "user"
        entry.Type = "user_turn"
        entry.Data = map[string]any{"text": data["user_input"], "mode": data["mode"], "scope": data["scope"]}
        if entry.Data["text"] == nil || entry.Data["text"] == "" {
            return HistoryEntry{}, false // 无 user_input 不落盘
        }
    case model.EventToken, model.EventInfo:
        entry.Turn = "agent"
        entry.Type = "markdown"
    case model.EventNeedsInput:
        entry.Turn = "agent"
        entry.Type = "needs_input"
    case model.EventDone:
        entry.Turn = "agent"
        entry.Type = "final_result"
    case model.EventError:
        entry.Turn = "agent"
        entry.Type = "error"
    default:
        return HistoryEntry{}, false
    }
    return entry, true
}
```

在 `Emit` 里 `AppendEvent` 成功后追加：

```go
if err := b.store.AppendEvent(ctx, e); err != nil {
    return err
}
if b.hw != nil && isWhitelistedForHistory(evt) {
    if entry, ok := buildHistoryEntry(e); ok {
        if err := b.hw.Append(ctx, b.threadID, entry); err != nil {
            // 不阻塞主流程；这里为简化不引入 logger，交由调用方观测
        }
    }
}
```

- [ ] **Step 4: 修改 Engine 调用**

修改 `backend/internal/run/engine.go`：`Engine` 加字段 `hw HistoryWriter`；`NewEngine` 增加参数：

```go
type Engine struct {
    store Store
    locks *LockManager
    hw    HistoryWriter
    log   *zap.Logger
    // ...
}

func NewEngine(store Store, locks *LockManager, hw HistoryWriter, log *zap.Logger) *Engine {
    return &Engine{store: store, locks: locks, hw: hw, log: log, actives: map[string]*active{}}
}
```

`Engine.Start` 里 `bus := NewBus(r.ID, e.store)` 改为 `bus := NewBus(r.ID, r.ThreadID, e.store, e.hw)`。

- [ ] **Step 5: 修所有 NewEngine 调用点**

Run: `cd backend && grep -rn "NewEngine(" --include=*.go`
把每处调用（含测试）改为多传一个 `nil` 或 `NewFSHistoryWriter(...)` 参数。测试里传 `nil` 即可，因为 `Bus.Emit` 已判空。

- [ ] **Step 6: 运行测试**

Run: `cd backend && go test ./...`
Expected: PASS（新 Bus 测试通过，既有测试因传 nil hw 也不变）。

- [ ] **Step 7: 提交**

```bash
cd backend
git add internal/run/bus.go internal/run/bus_test.go internal/run/engine.go $(git ls-files -m)
git commit -m "feat(ux-consistency): bus appends whitelisted events to history"
```

---

### Task 1.4: ThreadService.History 支持损坏行跳过 + seq 排序

**Files:**
- Modify: `backend/internal/service/thread.go:86-126`
- Create: `backend/internal/service/thread_history_test.go`

**Interfaces:**
- Consumes: 现有 `History(ctx, id) ([]map[string]any, error)`
- Produces: 相同签名，语义变更为跳过损坏行

- [ ] **Step 1: 写失败测试**

新建 [backend/internal/service/thread_history_test.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/service/thread_history_test.go)：

```go
package service_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"

    "go.uber.org/zap"

    "github.com/dasi0227/PPT-Agent/backend/internal/config"
    "github.com/dasi0227/PPT-Agent/backend/internal/model"
    "github.com/dasi0227/PPT-Agent/backend/internal/service"
    sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestHistorySkipsCorruptLinesAndSortsBySeq(t *testing.T) {
    ctx := context.Background()
    work := t.TempDir()
    workDir := filepath.Join(work, "p1")
    _ = os.MkdirAll(filepath.Join(workDir, "threads"), 0o755)

    lines := []string{
        `{"seq":2,"ts":200,"run_id":"r1","turn":"agent","type":"markdown","data":{"text":"b"}}`,
        `{corrupt`,
        `{"seq":1,"ts":100,"run_id":"r1","turn":"user","type":"user_turn","data":{"text":"a"}}`,
    }
    _ = os.WriteFile(filepath.Join(workDir, "threads/t1.jsonl"), []byte(joinLines(lines)+"\n"), 0o644)

    cfg := &config.Config{DBPath: filepath.Join(work, "t.db"), WorkRoot: work}
    db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    t.Cleanup(cleanup)
    st, err := sqlitestore.NewStore(db, zap.NewNop())
    if err != nil {
        t.Fatal(err)
    }
    now := time.Now().Unix()
    if err := st.CreateProject(ctx, model.Project{ID: "p1", WorkDir: workDir, Title: "t", Theme: "d", Status: "ready", CreatedAt: now, UpdatedAt: now}); err != nil {
        t.Fatal(err)
    }
    if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
        t.Fatal(err)
    }

    svc := service.NewThreadService(st)
    out, err := svc.History(ctx, "t1")
    if err != nil {
        t.Fatalf("history err: %v", err)
    }
    if len(out) != 2 {
        t.Fatalf("expected 2 valid entries, got %d", len(out))
    }
    if getFloat(out[0], "seq") != 1 || getFloat(out[1], "seq") != 2 {
        t.Fatalf("expected sorted by seq, got %+v", out)
    }
}

func joinLines(lines []string) string {
    out := ""
    for i, l := range lines {
        if i > 0 {
            out += "\n"
        }
        out += l
    }
    return out
}

func getFloat(m map[string]any, k string) float64 {
    if v, ok := m[k].(float64); ok {
        return v
    }
    return 0
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/service/ -run TestHistorySkipsCorrupt -v`
Expected: FAIL（当前 History 遇到坏行整体报错）。

- [ ] **Step 3: 修改 History 实现**

改 `backend/internal/service/thread.go` 的 `History`：

```go
func (svc *ThreadService) History(ctx context.Context, id string) ([]map[string]any, error) {
    th, err := svc.store.GetThread(ctx, id)
    if err != nil {
        return nil, err
    }
    proj, err := svc.store.GetProject(ctx, th.ProjectID)
    if err != nil {
        return nil, err
    }
    sb, err := tools.NewSandbox(proj.WorkDir)
    if err != nil {
        return nil, err
    }
    raw, err := sb.Read(th.HistoryPath)
    if os.IsNotExist(err) {
        return []map[string]any{}, nil
    }
    if err != nil {
        return nil, err
    }
    var out []map[string]any
    scanner := bufio.NewScanner(strings.NewReader(string(raw)))
    scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        var msg map[string]any
        if err := json.Unmarshal([]byte(line), &msg); err != nil {
            // 跳过损坏行（不中断读取）
            continue
        }
        out = append(out, msg)
    }
    if err := scanner.Err(); err != nil {
        return nil, err
    }
    // 按 seq 升序排序
    sort.SliceStable(out, func(i, j int) bool {
        return getFloat(out[i], "seq") < getFloat(out[j], "seq")
    })
    if out == nil {
        out = []map[string]any{}
    }
    return out, nil
}

func getFloat(m map[string]any, k string) float64 {
    if v, ok := m[k].(float64); ok {
        return v
    }
    return 0
}
```

在 import 里加 `"sort"`。

- [ ] **Step 4: 运行测试**

Run: `cd backend && go test ./internal/service/ -run TestHistorySkipsCorrupt -v`
Expected: PASS。

再跑全量：`cd backend && go test ./...`。

- [ ] **Step 5: 提交**

```bash
cd backend
git add internal/service/thread.go internal/service/thread_history_test.go
git commit -m "feat(ux-consistency): thread history skips corrupt lines and sorts by seq"
```

---

### Task 1.5: bootstrap 注入 FSHistoryWriter

**Files:**
- Modify: `backend/cmd/server/main.go`（或等价 bootstrap；如实际位置不同，用 `grep -rn "NewEngine(" backend/cmd` 定位）

**Interfaces:**
- Consumes: `NewFSHistoryWriter`（Task 1.2）、`NewEngine(..., hw, log)`（Task 1.3）

- [ ] **Step 1: 定位 bootstrap 位置**

Run: `cd backend && grep -rn "NewEngine(" cmd/ 2>/dev/null | head`

假设输出指向 `backend/cmd/server/main.go` 附近。

- [ ] **Step 2: 修改 bootstrap 注入**

在构造 `store`（`sqlite.Open(...)`）之后、构造 `Engine` 之前，实例化 writer 并传入：

```go
hw := run.NewFSHistoryWriter(&threadLocator{store: store})
engine := run.NewEngine(store, locks, hw, logger)
```

`threadLocator` 是一个薄适配器（若 store 本身实现了 `GetThread + GetProject`，可以直接把 store 传进去）：

```go
type threadLocator struct {
    store store.Store
}
func (l *threadLocator) GetThread(ctx context.Context, id string) (model.Thread, error) {
    return l.store.GetThread(ctx, id)
}
func (l *threadLocator) GetProject(ctx context.Context, id string) (model.Project, error) {
    return l.store.GetProject(ctx, id)
}
```

如果 `store.Store` 接口本身满足 `run.ThreadLocator`，直接 `hw := run.NewFSHistoryWriter(store)`。

- [ ] **Step 3: 构建 + 冒烟**

Run: `cd backend && go build ./...`
Expected: 无编译错误。

Run: `./restart.sh` (根目录) 让后端启动，`curl http://localhost:8787/api/v1/projects` 至少返回 200。

- [ ] **Step 4: 提交**

```bash
cd backend
git add cmd/
git commit -m "feat(ux-consistency): wire FSHistoryWriter into engine"
```

---

### Task 1.6: 端到端验证 history 落盘

**Files:**
- Create: `backend/internal/httpapi/thread_history_e2e_test.go`

**Interfaces:**
- Consumes: 前面所有阶段 1 的组件

- [ ] **Step 1: 写 e2e 测试**

参考现有 `backend/internal/httpapi/edit_e2e_test.go` 的服务器搭建模式，新建 [thread_history_e2e_test.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/httpapi/thread_history_e2e_test.go)：

```go
package httpapi_test

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/dasi0227/PPT-Agent/backend/internal/model"
    // ...其他 helpers 复用同包既有的 setup 函数
)

func TestThreadHistoryE2E_UserTurnAndFinalResultLanded(t *testing.T) {
    // 使用与 edit_e2e_test.go 相同的 setup：起 gin 服务，注入 FSHistoryWriter
    srv, workdir, projectID, threadID := setupServerWithProject(t)
    defer srv.Close()

    payload := map[string]any{"kind": "command", "instruction": "/recap", "mode": "normal"}
    raw, _ := json.Marshal(payload)
    resp, err := http.Post(srv.URL+"/api/v1/threads/"+threadID+"/runs", "application/json", strings.NewReader(string(raw)))
    if err != nil || resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        t.Fatalf("create run: %v %s", err, string(b))
    }

    // 等 SSE 结束（用现有 helper 或轮询 GET /runs/:id 直到 done）
    time.Sleep(500 * time.Millisecond)

    // 读 history.jsonl
    path := filepath.Join(workdir, "threads", threadID+".jsonl")
    b, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read history: %v", err)
    }
    if len(strings.TrimSpace(string(b))) == 0 {
        t.Fatal("history.jsonl is empty")
    }
    // 至少有 user_turn 一行
    if !strings.Contains(string(b), `"type":"user_turn"`) {
        t.Fatalf("expected user_turn in history, got: %s", string(b))
    }

    // 再走 API 端读
    r2, _ := http.Get(srv.URL + "/api/v1/threads/" + threadID + "/history")
    var out []map[string]any
    _ = json.NewDecoder(r2.Body).Decode(&out)
    if len(out) < 1 {
        t.Fatalf("expected history entries via API, got %d", len(out))
    }
    _ = context.Background()
    _ = model.Thread{} // avoid unused import
}
```

> 若 `setupServerWithProject` 不存在，从 [edit_e2e_test.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/httpapi/edit_e2e_test.go) 复制 setup 逻辑，或先抽出一个共享 helper。

- [ ] **Step 2: 运行**

Run: `cd backend && go test ./internal/httpapi/ -run TestThreadHistoryE2E -v`
Expected: PASS。

- [ ] **Step 3: 提交**

```bash
cd backend
git add internal/httpapi/thread_history_e2e_test.go
git commit -m "feat(ux-consistency): e2e verifies history append via bus"
```

---

## 阶段 2 · 前端 store · 三合一（问题 #1 / #3 / #4 / #5）

### Task 2.1: runStore 头插 user_turn + done reload + hydrateTimeline

**Files:**
- Modify: `frontend/src/stores/runStore.ts`
- Modify: `frontend/src/features/agent/eventReducer.ts`（先加类型，避免 runStore 引用不到）
- Modify: `frontend/src/stores/runStore.multithread.test.ts`（若有影响则更新）
- Create test additions: `frontend/src/stores/runStore.test.ts`（若不存在则新建）

**Interfaces:**
- Produces:
  ```ts
  export interface UserTurnItem { id: string; type: 'user_turn'; text: string; timestamp: number }
  interface RunStoreV2 {
      hydrateTimeline: (threadId: string, items: TimelineItem[]) => void;
      // ... 既有 API 不变
  }
  ```

- [ ] **Step 1: 类型扩展**

修改 [eventReducer.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/eventReducer.ts)：

```ts
export type TimelineItemType = 'markdown' | 'thought' | 'tool_call' | 'artifact' | 'final_result' | 'needs_input' | 'error' | 'user_turn';

export interface UserTurnItem extends BaseTimelineItem {
  type: 'user_turn';
  text: string;
}

export type TimelineItem =
  | MarkdownMessageItem
  | ThoughtItem
  | ToolCallItem
  | ArtifactItem
  | FinalResultItem
  | NeedsInputItem
  | ErrorItem
  | UserTurnItem;
```

- [ ] **Step 2: 写失败测试**

新建 [frontend/src/stores/runStore.test.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/stores/runStore.test.ts)：

```ts
import { beforeEach, describe, expect, test, vi } from 'vitest';
import { useRunStore } from './runStore';
import { useProjectStore } from './projectStore';
import type { TimelineItem } from '../features/agent/eventReducer';

describe('runStore user_turn + reload + hydrate', () => {
  beforeEach(() => {
    useRunStore.setState({ sessions: {} });
    useProjectStore.setState({ projects: [], activeProjectId: 'p1', slidesByProjectId: {}, loadingProjects: false });
  });

  test('createRun inserts user_turn synchronously before network', async () => {
    // 通过 mock fetch 让 create 挂起，验证在 await 之前 user_turn 已进入 timeline
    let resolveCreate: (v: any) => void = () => {};
    const spy = vi.spyOn(global, 'fetch').mockImplementation(
      () => new Promise((r) => { resolveCreate = r; }),
    );
    const p = useRunStore.getState().createRun('t1', { kind: 'edit', instruction: 'hello' });
    // 同步断言：timeline 已有 user_turn
    const items = useRunStore.getState().sessions['t1']?.timelineItems ?? [];
    expect(items.length).toBe(1);
    expect(items[0].type).toBe('user_turn');
    expect((items[0] as any).text).toBe('hello');
    resolveCreate(new Response(JSON.stringify({ id: 'r1' }), { status: 200 }));
    await p.catch(() => {});
    spy.mockRestore();
  });

  test('hydrateTimeline is idempotent when timelineItems non-empty', () => {
    const seed: TimelineItem = { id: 'existing', type: 'markdown', text: 'seed', timestamp: 1 };
    useRunStore.setState({ sessions: { t1: { activeRunId: null, status: 'idle', mode: 'normal', scope: 'current', timelineItems: [seed], pendingInput: null, progress: null, eventSourceClose: null, plan: null } } });
    useRunStore.getState().hydrateTimeline('t1', [{ id: 'other', type: 'markdown', text: 'other', timestamp: 2 }]);
    const items = useRunStore.getState().sessions['t1'].timelineItems;
    expect(items).toHaveLength(1);
    expect(items[0].id).toBe('existing');
  });

  test('hydrateTimeline writes when empty', () => {
    useRunStore.setState({ sessions: {} });
    useRunStore.getState().hydrateTimeline('t2', [{ id: 'h1', type: 'markdown', text: 'restored', timestamp: 5 }]);
    expect(useRunStore.getState().sessions['t2'].timelineItems).toHaveLength(1);
  });
});
```

- [ ] **Step 3: 运行确认失败**

Run: `cd frontend && npm test -- --run src/stores/runStore.test.ts`
Expected: FAIL（`hydrateTimeline` 不存在 / user_turn 未插入）。

- [ ] **Step 4: 修改 runStore**

修改 [runStore.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/stores/runStore.ts)：

在 `RunStoreV2` interface 加：
```ts
hydrateTimeline: (threadId: string, items: TimelineItem[]) => void;
```

在 store 实现里，`createRun` 头部先插 user_turn（**不要清空 timelineItems**）：

```ts
createRun: async (threadId, payload) => {
  try {
    get().sessions[threadId]?.eventSourceClose?.();

    // 立即回显：user_turn 头插（不等后端 200）
    const userItem: TimelineItem = {
      id: `user_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      type: 'user_turn',
      text: payload.instruction,
      timestamp: Date.now(),
    };
    updateSession(threadId, (prev) => ({
      timelineItems: [...prev.timelineItems, userItem],
      status: 'running',
      pendingInput: null,
      mode: payload.mode ?? 'normal',
      scope: payload.scope ?? 'current',
      progress: null,
      plan: null,
      eventSourceClose: null,
      activeRunId: null,
    }));

    const run = await runsApi.create(threadId, payload);
    patchSession(threadId, { activeRunId: run.id });
    get().subscribeRun(threadId, run.id);
  } catch (err) {
    patchSession(threadId, { status: 'error' });
    console.error(err);
  }
},
```

在 `subscribeRun` 的 done/error 分支追加 reload：

```ts
if (event.event === 'done' || event.event === 'error') {
  get().sessions[threadId]?.eventSourceClose?.();
  patchSession(threadId, { eventSourceClose: null });
  const pid = useProjectStore.getState().activeProjectId;
  if (pid) void useProjectStore.getState().loadProjectSlides(pid);
}
```

新增 action：

```ts
hydrateTimeline: (threadId, items) => {
  const existing = get().sessions[threadId]?.timelineItems ?? [];
  if (existing.length > 0) return;
  patchSession(threadId, { timelineItems: items });
},
```

导入 `useProjectStore`（顶部加 `import { useProjectStore } from './projectStore';`），注意避免循环——`projectStore` 已经 import 了 `runStore`，双向 import 在 ESM 下是合法的但要确保运行时按需 `getState()`。

- [ ] **Step 5: 运行测试**

Run: `cd frontend && npm test -- --run src/stores/runStore.test.ts`
Expected: PASS。

Run: `cd frontend && npm test -- --run`（全量）确保其他测试未回归。若 `runStore.multithread.test.ts` 断言 timelineItems 长度受影响，同步更新期望值。

- [ ] **Step 6: 提交**

```bash
cd frontend
git add src/stores/runStore.ts src/stores/runStore.test.ts src/features/agent/eventReducer.ts
git commit -m "feat(ux-consistency): runStore user_turn head insert + done reload + hydrate"
```

---

### Task 2.2: threadStore nextUntitledName + 替换命名

**Files:**
- Modify: `frontend/src/stores/threadStore.ts`
- Modify: `frontend/src/features/agent/ThreadTabs.tsx:42-44`
- Modify: `frontend/src/stores/threadStore.test.ts`

**Interfaces:**
- Produces: `nextUntitledName: (projectId: string) => string`

- [ ] **Step 1: 写失败测试**

在 [threadStore.test.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/stores/threadStore.test.ts) 追加：

```ts
describe('nextUntitledName', () => {
  beforeEach(() => {
    useThreadStore.setState({ threadsByProjectId: {}, draftThreadsByProjectId: {}, openThreadIdsByProjectId: {}, activeThreadIdByProjectId: {} });
  });

  test('empty project returns 未命名 1', () => {
    expect(useThreadStore.getState().nextUntitledName('p1')).toBe('未命名 1');
  });

  test('monotonic across existing untitled entries', () => {
    useThreadStore.setState({
      threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', title: '未命名 1', created_at: 0, updated_at: 0 }, { id: 't2', project_id: 'p1', title: '未命名 3', created_at: 0, updated_at: 0 }] },
    });
    expect(useThreadStore.getState().nextUntitledName('p1')).toBe('未命名 4');
  });

  test('ignores non-matching titles', () => {
    useThreadStore.setState({
      threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', title: '主线程', created_at: 0, updated_at: 0 }] },
    });
    expect(useThreadStore.getState().nextUntitledName('p1')).toBe('未命名 1');
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- --run src/stores/threadStore.test.ts`
Expected: FAIL（`nextUntitledName` 不存在）。

- [ ] **Step 3: 实现**

修改 [threadStore.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/stores/threadStore.ts)：

在 interface `ThreadState` 加 `nextUntitledName: (projectId: string) => string;`

在 store 实现里加：

```ts
nextUntitledName: (projectId) => {
  const all = [
    ...(get().threadsByProjectId[projectId] || []),
    ...(get().draftThreadsByProjectId[projectId] || []),
  ];
  const re = /^未命名 (\d+)$/;
  const maxN = all.reduce((max, t) => {
    const m = (t.title || '').match(re);
    return m ? Math.max(max, parseInt(m[1], 10)) : max;
  }, 0);
  return `未命名 ${maxN + 1}`;
},
```

替换 `ensureActiveThread` 里 `createDraftThread(projectId, '主线程')` 为 `createDraftThread(projectId, get().nextUntitledName(projectId))`。

- [ ] **Step 4: 替换 ThreadTabs.handleNew**

修改 [ThreadTabs.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/ThreadTabs.tsx) 第 42-44 行：

```tsx
const handleNew = () => {
  createDraftThread(activeProjectId, useThreadStore.getState().nextUntitledName(activeProjectId));
};
```

（需要在 useThreadStore 解构里加 `nextUntitledName` 到 store 引用；或直接如上 getState 拿）

- [ ] **Step 5: 跑测试**

Run: `cd frontend && npm test -- --run`
Expected: PASS（所有 threadStore 相关测试通过，含既有 `ensureActiveThread` 行为——若既有测试断言了 title === '主线程'，需要同步改成 '未命名 1'）。

- [ ] **Step 6: 提交**

```bash
cd frontend
git add src/stores/threadStore.ts src/stores/threadStore.test.ts src/features/agent/ThreadTabs.tsx
git commit -m "feat(ux-consistency): rename default thread to 未命名 N"
```

---

### Task 2.3: deckStore globalView + effectiveView 重写

**Files:**
- Modify: `frontend/src/stores/deckStore.ts`
- Modify: `frontend/src/stores/deckStore.test.ts`

**Interfaces:**
- Produces:
  ```ts
  globalView: 'outline' | 'html';   // 默认 'html'
  setGlobalView: (view: 'outline' | 'html') => void;
  effectiveView: (slideId: string, hasHtml: boolean) => 'outline' | 'html';   // 语义变更
  ```

- [ ] **Step 1: 写新测试并更新既有测试**

替换 [deckStore.test.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/stores/deckStore.test.ts) 内容：

```ts
import { beforeEach, describe, expect, test } from 'vitest';
import { useDeckStore } from './deckStore';

describe('deckStore globalView', () => {
  beforeEach(() => {
    useDeckStore.setState({ globalView: 'html', viewByPage: {} });
  });

  test('default is html, fallback to outline when hasHtml=false', () => {
    const s = useDeckStore.getState();
    expect(s.effectiveView('s1', true)).toBe('html');
    expect(s.effectiveView('s2', false)).toBe('outline');
  });

  test('globalView=outline overrides hasHtml', () => {
    useDeckStore.getState().setGlobalView('outline');
    const s = useDeckStore.getState();
    expect(s.effectiveView('s1', true)).toBe('outline');
    expect(s.effectiveView('s2', false)).toBe('outline');
  });

  test('setGlobalView(html) restores default', () => {
    useDeckStore.getState().setGlobalView('outline');
    useDeckStore.getState().setGlobalView('html');
    expect(useDeckStore.getState().globalView).toBe('html');
    expect(useDeckStore.getState().effectiveView('s1', true)).toBe('html');
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- --run src/stores/deckStore.test.ts`
Expected: FAIL（`globalView` / `setGlobalView` 不存在，effectiveView 逻辑还是旧的）。

- [ ] **Step 3: 修改 deckStore**

修改 [deckStore.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/stores/deckStore.ts)：

```ts
export type PageView = 'outline' | 'html';

interface DeckState {
  currentPage: number;
  overviewOpen: boolean;
  previewMode: 'main' | 'overview' | 'single';
  iframeReady: boolean;
  loadCount: number;
  viewByPage: Record<string, PageView>;
  globalView: PageView;

  setCurrentPage: (index: number) => void;
  goNext: () => void;
  goPrev: () => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setIframeReady: (ready: boolean) => void;
  reloadSlide: (index: number) => void;
  setPageView: (slideId: string, view: PageView) => void;   // 保留，noop 或写入 viewByPage 但不影响决策
  setGlobalView: (view: PageView) => void;
  effectiveView: (slideId: string, hasHtml: boolean) => PageView;
}

export const useDeckStore = create<DeckState>((set, get) => ({
  currentPage: 0,
  overviewOpen: false,
  previewMode: 'main',
  iframeReady: false,
  loadCount: 0,
  viewByPage: {},
  globalView: 'html',

  setCurrentPage: (index) => set({ currentPage: index }),
  goNext: () => set((state) => ({ currentPage: state.currentPage + 1 })),
  goPrev: () => set((state) => ({ currentPage: Math.max(0, state.currentPage - 1) })),
  enterOverview: () => set({ overviewOpen: true, previewMode: 'overview' }),
  exitOverview: () => set({ overviewOpen: false, previewMode: 'main' }),
  setIframeReady: (ready) => set({ iframeReady: ready }),
  reloadSlide: () => set((state) => ({ loadCount: state.loadCount + 1 })),

  setPageView: (slideId, view) => set((state) => ({ viewByPage: { ...state.viewByPage, [slideId]: view } })),   // 保留但不影响 effectiveView
  setGlobalView: (view) => set({ globalView: view }),

  effectiveView: (_slideId, hasHtml) => {
    if (get().globalView === 'outline') return 'outline';
    return hasHtml ? 'html' : 'outline';
  },
}));
```

- [ ] **Step 4: 跑测试**

Run: `cd frontend && npm test -- --run src/stores/deckStore.test.ts`
Expected: PASS。

`cd frontend && npm test -- --run` 全量。若 `PreviewWorkspace.test.tsx` 中有对 `viewByPage` 决策的断言，将在阶段 3 一并修正。

- [ ] **Step 5: 提交**

```bash
cd frontend
git add src/stores/deckStore.ts src/stores/deckStore.test.ts
git commit -m "feat(ux-consistency): deckStore globalView drives outline/html switch"
```

---

## 阶段 3 · 前端 · 渲染与 replay（问题 #1 / #2 前端 / #6 / #7）

### Task 3.1: Timeline 加 user_turn case + eventReducer 测试

**Files:**
- Modify: `frontend/src/features/agent/Timeline.tsx`
- Modify: `frontend/src/features/agent/eventReducer.test.ts`（若不存在则新建）

**Interfaces:**
- Consumes: `UserTurnItem`（Task 2.1 已定义）

- [ ] **Step 1: 写测试**

在 `frontend/src/features/agent/eventReducer.test.ts` 追加/新建：

```ts
import { describe, expect, test } from 'vitest';
import type { UserTurnItem } from './eventReducer';

describe('UserTurnItem type shape', () => {
  test('carries id/type/text/timestamp', () => {
    const item: UserTurnItem = { id: 'u1', type: 'user_turn', text: 'hello', timestamp: 1 };
    expect(item.type).toBe('user_turn');
    expect(item.text).toBe('hello');
  });
});
```

- [ ] **Step 2: 加 Timeline case**

修改 [Timeline.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/Timeline.tsx) 的 switch，在开头加：

```tsx
case 'user_turn':
  return (
    <div key={item.id} className="flex justify-end">
      <div className="max-w-[85%] rounded-lg bg-mode-normal/10 border border-mode-normal/20 px-3 py-2">
        <MarkdownMessage content={item.text} />
      </div>
    </div>
  );
```

- [ ] **Step 3: 运行测试**

Run: `cd frontend && npm test -- --run`
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
cd frontend
git add src/features/agent/Timeline.tsx src/features/agent/eventReducer.test.ts
git commit -m "feat(ux-consistency): render user_turn as right-aligned bubble"
```

---

### Task 3.2: ThoughtCard / FinalResultCard / NeedsInputCard → MarkdownMessage

**Files:**
- Modify: `frontend/src/features/agent/ThoughtCard.tsx`
- Modify: `frontend/src/features/agent/FinalResultCard.tsx`
- Modify: `frontend/src/features/agent/NeedsInputCard.tsx`
- Modify: `frontend/src/features/agent/MarkdownMessage.test.tsx`（若已有测试文件则追加）

**Interfaces:**
- Consumes: `MarkdownMessage`

- [ ] **Step 1: 写测试**

在 `frontend/src/features/agent/MarkdownMessage.test.tsx` 追加渲染 fixture（示例）：

```tsx
import { describe, test, expect } from 'vitest';
import { render } from '@testing-library/react';
import { MarkdownMessage } from './MarkdownMessage';

describe('MarkdownMessage', () => {
  test('renders inline code', () => {
    const { container } = render(<MarkdownMessage content="here is `code`" />);
    expect(container.querySelector('code')).not.toBeNull();
  });
});
```

- [ ] **Step 2: 替换文本渲染**

各卡片内部的字符串文本字段（`text` / `prompt` / string 型 `result`）替换为：

```tsx
<MarkdownMessage content={value} />
```

`FinalResultCard`：若 `result` 是对象，保留结构化 JSON 展示；`typeof result === 'string'` 时走 MarkdownMessage。

`ToolCallCard / ArtifactCard / ErrorCard` 不改（结构化卡）。

- [ ] **Step 3: 运行测试**

Run: `cd frontend && npm test -- --run`
Expected: PASS（既有 `Cards.test.tsx` 若有 text-content 断言，可能需要放宽到 `container.textContent.includes(...)` 而非全等）。

- [ ] **Step 4: 提交**

```bash
cd frontend
git add src/features/agent/ThoughtCard.tsx src/features/agent/FinalResultCard.tsx src/features/agent/NeedsInputCard.tsx src/features/agent/MarkdownMessage.test.tsx
git commit -m "feat(ux-consistency): render agent text cards via MarkdownMessage"
```

---

### Task 3.3: OutlineCard 空态占位

**Files:**
- Modify: `frontend/src/features/viewer/OutlineCard.tsx`
- Modify: `frontend/src/features/viewer/OutlineCard.test.tsx`

- [ ] **Step 1: 写测试**

在 `OutlineCard.test.tsx` 追加：

```tsx
import { render } from '@testing-library/react';
import { OutlineCard } from './OutlineCard';

test('compact + empty content shows Slide N + 未编辑大纲', () => {
  const { getByText } = render(
    <OutlineCard
      slide={{ id: 's1', project_id: 'p1', idx: 2, order: 2, layout: 'bullets', title: '', html_path: '', json_path: '', current_version: 0, outline_dirty: false }}
      editable={false}
      dirty={false}
      onPatch={() => {}}
      compact
    />,
  );
  expect(getByText('Slide 3')).toBeTruthy();
  expect(getByText('未编辑大纲')).toBeTruthy();
});
```

- [ ] **Step 2: 实现**

修改 [OutlineCard.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/viewer/OutlineCard.tsx)：

```tsx
const content = resolveContent(slide);
const isEmpty = !content.title
  && !(content.bullets?.length)
  && !content.subtitle
  && !content.content_intent;

if (isEmpty && compact) {
  return (
    <div className="w-full h-full flex flex-col items-center justify-center bg-white p-3 relative">
      <span className="absolute top-3 left-3 px-2 py-0.5 rounded-full bg-background text-[10px] text-text-400 border border-border uppercase">
        {content.layout || slide.layout}
      </span>
      <div className="text-text-400 text-xs">Slide {(slide.order ?? slide.idx) + 1}</div>
      <div className="text-text-400 text-[10px] mt-1">未编辑大纲</div>
    </div>
  );
}

if (isEmpty && !compact) {
  return (
    <div className="relative w-full aspect-video bg-white ring-1 ring-border rounded-md shadow-sm flex flex-col items-center justify-center p-8">
      <span className="absolute top-3 left-3 px-2 py-0.5 rounded-full bg-background text-[10px] text-text-400 border border-border uppercase">
        {content.layout || slide.layout}
      </span>
      <h1 className="text-3xl text-text-400 font-semibold">未命名</h1>
      <p className="text-text-400 text-sm mt-3">使用右侧对话或点击标题开始编辑</p>
    </div>
  );
}

// 其余渲染保留（editable/bullets/chart_intent 等）
```

- [ ] **Step 3: 测试**

Run: `cd frontend && npm test -- --run src/features/viewer/OutlineCard.test.tsx`
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
cd frontend
git add src/features/viewer/OutlineCard.tsx src/features/viewer/OutlineCard.test.tsx
git commit -m "feat(ux-consistency): OutlineCard empty-state placeholder"
```

---

### Task 3.4: PreviewWorkspace 段控接 globalView + 网格「暂无 HTML」徽标

**Files:**
- Modify: `frontend/src/features/viewer/PreviewWorkspace.tsx`
- Modify: `frontend/src/features/viewer/PreviewWorkspace.test.tsx`

- [ ] **Step 1: 更新既有测试断言**

将 `PreviewWorkspace.test.tsx` 里对 `viewByPage` / `setPageView` 的断言，替换为对 `globalView` / `setGlobalView` 的：

```ts
useDeckStore.setState({ globalView: 'outline' });
// 期望：所有页强制显示 outline
useDeckStore.getState().setGlobalView('html');
// 期望：有 html 的页显 iframe，无 html 的页 fallback outline
```

至少三条：默认 html、切 outline 全局生效、无 html 页 fallback。

- [ ] **Step 2: 修改段控**

修改 [PreviewWorkspace.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/viewer/PreviewWorkspace.tsx) 工具栏部分：

```tsx
const { currentPage, previewMode, enterOverview, exitOverview, goNext, goPrev, effectiveView, globalView, setGlobalView } = useDeckStore();
// ...
{previewMode === 'main' && (
  <div className="flex items-center rounded-md border border-border overflow-hidden text-xs">
    <button
      onClick={() => setGlobalView('outline')}
      className={cn(
        'px-2.5 py-1 transition-colors',
        globalView === 'outline' ? 'bg-mode-normal/10 text-mode-normal font-medium' : 'text-text-600 hover:bg-black/5',
      )}
    >
      大纲
    </button>
    <button
      onClick={() => setGlobalView('html')}
      className={cn(
        'px-2.5 py-1 transition-colors border-l border-border',
        globalView === 'html' ? 'bg-mode-normal/10 text-mode-normal font-medium' : 'text-text-600 hover:bg-black/5',
      )}
    >
      HTML
    </button>
  </div>
)}
```

网格分支的 badge：

```tsx
{!slide.html_path && (
  <div className="absolute bottom-2 left-2 bg-amber-500/80 text-white text-[10px] px-1.5 py-0.5 rounded backdrop-blur-sm">
    暂无 HTML
  </div>
)}
```

- [ ] **Step 3: 测试**

Run: `cd frontend && npm test -- --run src/features/viewer/PreviewWorkspace.test.tsx`
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
cd frontend
git add src/features/viewer/PreviewWorkspace.tsx src/features/viewer/PreviewWorkspace.test.tsx
git commit -m "feat(ux-consistency): PreviewWorkspace bind globalView + grid html-missing badge"
```

---

### Task 3.5: historyHydrator + useActiveSession replay 触发

**Files:**
- Create: `frontend/src/features/agent/historyHydrator.ts`
- Create: `frontend/src/features/agent/historyHydrator.test.ts`
- Modify: `frontend/src/features/agent/useActiveSession.ts`

**Interfaces:**
- Consumes: `threadsApi.history`（[threads.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/api/threads.ts)）、`runStore.hydrateTimeline`（Task 2.1）
- Produces: `hydrateFromHistory(entries: HistoryEntry[]): TimelineItem[]`

- [ ] **Step 1: 写测试**

新建 [historyHydrator.test.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/historyHydrator.test.ts)：

```ts
import { describe, expect, test } from 'vitest';
import { hydrateFromHistory, HistoryEntry } from './historyHydrator';

describe('hydrateFromHistory', () => {
  test('sorts by seq and maps types', () => {
    const entries: HistoryEntry[] = [
      { seq: 2, ts: 200, run_id: 'r1', turn: 'agent', type: 'markdown', data: { text: 'reply' } },
      { seq: 1, ts: 100, run_id: 'r1', turn: 'user', type: 'user_turn', data: { text: 'hi' } },
      { seq: 3, ts: 300, run_id: 'r1', turn: 'agent', type: 'final_result', data: { result: { ok: true } } },
    ];
    const items = hydrateFromHistory(entries);
    expect(items).toHaveLength(3);
    expect(items[0].type).toBe('user_turn');
    expect(items[1].type).toBe('markdown');
    expect(items[2].type).toBe('final_result');
  });

  test('drops unknown types', () => {
    const items = hydrateFromHistory([{ seq: 1, ts: 1, run_id: 'r', turn: 'agent', type: 'weird' as any, data: {} }]);
    expect(items).toHaveLength(0);
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- --run src/features/agent/historyHydrator.test.ts`
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 实现 historyHydrator**

新建 [historyHydrator.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/historyHydrator.ts)：

```ts
import type { TimelineItem } from './eventReducer';

export interface HistoryEntry {
  seq: number;
  ts: number;
  run_id: string;
  turn: 'user' | 'agent';
  type: 'user_turn' | 'markdown' | 'info' | 'needs_input' | 'final_result' | 'error';
  data: Record<string, any>;
}

export function hydrateFromHistory(entries: HistoryEntry[]): TimelineItem[] {
  return entries
    .slice()
    .sort((a, b) => a.seq - b.seq)
    .map(toTimelineItem)
    .filter((x): x is TimelineItem => x !== null);
}

function toTimelineItem(e: HistoryEntry): TimelineItem | null {
  const base = { id: `hist_${e.seq}`, timestamp: (e.ts || 0) * 1000 };
  switch (e.type) {
    case 'user_turn':
      return { ...base, type: 'user_turn', text: String(e.data.text ?? '') };
    case 'markdown':
    case 'info':
      return { ...base, type: 'markdown', text: String(e.data.text ?? '') };
    case 'needs_input':
      return { ...base, id: String(e.data.id ?? base.id), type: 'needs_input', prompt: String(e.data.prompt ?? ''), choices: e.data.choices };
    case 'final_result':
      return { ...base, type: 'final_result', result: e.data.result };
    case 'error':
      return { ...base, type: 'error', code: e.data.code, message: String(e.data.message ?? '') };
    default:
      return null;
  }
}
```

- [ ] **Step 4: 触发 replay**

修改 [useActiveSession.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/useActiveSession.ts) —— 因当前是 selector hook（无 useEffect），把 replay 挂钩放在一个新的 `useEffect`。可以在同文件内导出新 hook `useReplayHistory(threadId)`，或直接在 `useActiveSession` 内部用 useEffect 触发一次：

```ts
import { useEffect } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore, RunSession, IDLE_SESSION } from '../../stores/runStore';
import { threadsApi } from '../../api/threads';
import { isDraftId } from '../../lib/draft';
import { hydrateFromHistory } from './historyHydrator';

export function useActiveThreadId(): string | null {
  const activeProjectId = useProjectStore((s) => s.activeProjectId);
  const activeThreadIdByProjectId = useThreadStore((s) => s.activeThreadIdByProjectId);
  if (!activeProjectId) return null;
  return activeThreadIdByProjectId[activeProjectId] ?? null;
}

export function useActiveSession(): RunSession {
  const threadId = useActiveThreadId();
  const sessions = useRunStore((s) => s.sessions);

  useEffect(() => {
    if (!threadId || isDraftId(threadId)) return;
    const current = useRunStore.getState().sessions[threadId];
    if (current && current.timelineItems.length > 0) return;
    threadsApi.history(threadId)
      .then((entries) => {
        if (!entries || entries.length === 0) return;
        useRunStore.getState().hydrateTimeline(threadId, hydrateFromHistory(entries as any));
      })
      .catch((err) => console.warn('history replay failed', err));
  }, [threadId]);

  if (!threadId) return IDLE_SESSION;
  return sessions[threadId] ?? IDLE_SESSION;
}
```

- [ ] **Step 5: 测试**

Run: `cd frontend && npm test -- --run`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
cd frontend
git add src/features/agent/historyHydrator.ts src/features/agent/historyHydrator.test.ts src/features/agent/useActiveSession.ts
git commit -m "feat(ux-consistency): frontend replay history on thread mount"
```

---

## 阶段 4 · 验收与收尾

### Task 4.1: 全量测试

- [ ] **Step 1: 后端**

Run: `cd backend && go test ./...`
Expected: 全绿。

- [ ] **Step 2: 前端**

Run: `cd frontend && npm test -- --run`
Expected: 全绿。

- [ ] **Step 3: 构建**

Run: `cd frontend && npm run build`
Expected: 无错。

若有失败，回到对应 Task 修补。

---

### Task 4.2: 浏览器手工走查

- [ ] **Step 1: 启动**

Run（cwd 项目根）: `./restart.sh --reset`
浏览器打开 `http://localhost:5173`。

- [ ] **Step 2: 走查 7 项症状**

| # | 验证步骤 | 预期 |
|---|---|---|
| 1 | 无 project → 输入 "hello world" 回车 | 消息立刻在 Timeline 右侧气泡回显，然后才看到 agent 响应 |
| 2 | 生成一份大纲后刷新浏览器 | 消息历史全部保留，位置正确，md 渲染完好 |
| 3 | 输入 "生成 PPT" 让 agent 跑完 | 无需手动刷新，预览区自动出现新页 |
| 4 | 无 project → 直接输入 → 侧栏名 | 应为 "未命名 1"，不是 "主线程"；再新建一个应该是 "未命名 2" |
| 5 | 首页切到"大纲" → 翻到第二页 | 第二页仍显示大纲，不需重复切换；无 HTML 页仍显示大纲卡 |
| 6 | 网格模式下未生成 HTML 的空白页 | 显示 "Slide N" + "未编辑大纲"；左下角显示 "暂无 HTML" 徽标 |
| 7 | 用户输入含 `**加粗** \`code\`` | Timeline 中正确渲染加粗与 inline code；agent 的 markdown 亦是 |

- [ ] **Step 3: 修补**

如任何一项不满足，回到对应 Task 修补并回归。

---

### Task 4.3: 收尾提交

- [ ] **Step 1: 检查 git 状态**

Run: `git status`
Expected: working tree clean（若还有零星调整从走查带出的，一并 commit）。

- [ ] **Step 2: 可选：更新 docs/v2/*.md** 中若引用旧行为的段落。

- [ ] **Step 3: push**（用户显式要求时才 push）

```bash
git push
```

---

## 附录 · 遗留清理项（本轮不做）

- `deckStore.viewByPage` / `setPageView` 保留但不再影响决策，M8 可整体清理
- `history.jsonl` 的分片 / 压缩 / 导出，视规模再议
- `nextUntitledName` 目前扫内存 title；未来可存到 project.meta 里，避免删光后再新建仍是"未命名 2"
