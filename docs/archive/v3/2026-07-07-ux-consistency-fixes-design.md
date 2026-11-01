# UX 一致性修复 · 设计文档

**日期**：2026-07-07
**里程碑**：`ux-consistency-fixes`
**提交前缀**：`feat(ux-consistency):`
**依赖**：M6/M7（Phase 3 已完成的多模态切换）

---

## 0 · 背景

大纲/页面多模态切换全部落地后，浏览器手工走查暴露 7 个跨端 UX 缺陷。这些缺陷分布在 SSE 事件总线、前端 store 生命周期、Timeline 渲染、Preview 视图规则等多层，各自独立但耦合严重。本轮把它们收敛到 **3 个横切层 + 4 处局部改动**，一次性修完。

### 用户反馈的 7 个问题（编号沿用本 spec 全程）

| # | 症状 | 根因 | 影响面 |
|---|---|---|---|
| 1 | 输入没有立即回显 | `handleSubmit` 从未把 user turn 推进 `timelineItems`，Timeline 只渲染 SSE 事件 | 交互响应硬约束 |
| 2 | 刷新后会话历史丢失 | `history.jsonl` 后端接口存在但**从未 append**；前端也没调 `GET /threads/:id/history` | 数据持久化硬约束 |
| 3 | 需手动刷新才能看到最新产物 | SSE `done` 分支只更新 status，未触发 `loadProjectSlides` | 自动刷新硬约束 |
| 4 | 默认会话叫"主线程" | `ensureActiveThread` 硬编码 `'主线程'`；`ThreadTabs.handleNew` 用 `新对话 N` | 命名规范 |
| 5 | 大纲/HTML 切换不全局 | `viewByPage[slideId]` 按页记忆，无全局字段 | 全局状态硬约束 |
| 6 | 网格中空白页视觉黑洞 | 未生成 HTML 且大纲内容为空时 OutlineCard 全白，无提示 | 视图逻辑硬约束 |
| 7 | 输入/回复不支持 Markdown | 用户 turn 未渲染；`thought/final_result/needs_input` 卡片纯文本 | 渲染硬约束 |

### 用户决策快照

- 范围：**全 7 项打包一次修完**
- 会话持久化：**后端 append + 前端 replay**
- 切换按钮语义：**全局一个开关，统一优先**
- 会话默认名：**未命名 N，全局递增**
- 网格空态：区分「未编辑大纲」（OutlineCard 空态占位）与「暂无 HTML」（右下角徽标）
- Markdown 覆盖：**文本内容全量 md**（tool_call / artifact / error 保留结构化卡）
- 跨端方案：**方案 A · Bus + runStore 钩子**

---

## 1 · 总体架构

### 1.1 三大横切层

**横切 1 · SSE 事件总线扩展（后端）**
`run.Bus.Publish` 在 `store.AppendEvent` 成功后追加副作用 `historyWriter.Append`。所有 SSE 事件必经此处，Runner 不感知，天然覆盖全部管线。

**横切 2 · 前端 store 生命周期钩子**
`runStore` 在两个位点做副作用挂钩：
- `createRun` 头部同步插入 `user_turn` timeline item（#1）
- SSE `done|error` 时统一 `loadProjectSlides(activeProjectId)`（#3）
- 新增 `hydrateTimeline` action，供 `useActiveSession` 首次进入 thread 时 replay history（#2）

**横切 3 · 消息渲染入口**
- Timeline 新增 `user_turn` 类型（#1），右对齐气泡走 `MarkdownMessage`（#7）
- `ThoughtCard / FinalResultCard / NeedsInputCard` 文本字段全部走 `MarkdownMessage`（#7）
- `ToolCallCard / ArtifactCard / ErrorCard` 保留结构化不变

### 1.2 四个局部改动

- **#4**：`threadStore.nextUntitledName(pid)` 工具函数 + 两个调用点替换
- **#5**：`deckStore.globalView` 新字段 + `effectiveView` 逻辑重写
- **#6**：`OutlineCard` 空态占位 + `PreviewWorkspace` 网格「暂无 HTML」徽标
- **#2 前端侧**：`historyHydrator.ts` 纯函数 + `useActiveSession` 触发 replay

---

## 2 · 后端 · SSE 事件持久化与 History 回放契约

### 2.1 落盘位置

在 [backend/internal/run/bus.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/run/bus.go) 的 `Publish` 方法里，在 `store.AppendEvent` 成功后追加：

```go
if hw != nil && isWhitelisted(e.Event) {
    _ = hw.Append(ctx, threadID, toHistoryEntry(e))  // 失败不阻塞主流程
}
```

放在 Bus 层的理由：所有事件必经此处，Runner 不感知，避免散落数十处 append。

### 2.2 写入白名单

| 事件 | 落盘 | 备注 |
|---|---|---|
| `run.started` | ✅ | 携带 `user_input`（来自 payload.instruction），落 `turn=user, type=user_turn` |
| `token` | ✅ | agent 增量文本，同 run 内合并成 `type=markdown` |
| `info` | ✅ | 独立行 |
| `needs_input` | ✅ | 独立行 |
| `done` | ✅ | `type=final_result`，data 含 result |
| `error` | ✅ | 独立行 |
| `thought` | ❌ | 中间产物，重放意义低 |
| `tool_call / tool_result / artifact` | ❌ | 过程调试信息，jsonl 膨胀风险 |
| `plan / plan.update / progress` | ❌ | 运行时快照，可从 last event 状态恢复 |

### 2.3 jsonl 每行 schema

```json
{
  "seq": 42,
  "ts": 1720345678,
  "run_id": "r_abc",
  "turn": "user",
  "type": "user_turn",
  "data": { "text": "...", "mode": "normal", "scope": "current" }
}
```

- `seq`：复用 `run_events` 已有的 sequence，全 thread 单调
- `turn`：`"user" | "agent"`
- `type`：`user_turn | markdown | info | needs_input | final_result | error`
- `data`：与 SSE data 一致的字段（前端 hydrator 复用 reduce 逻辑）

### 2.4 新增组件

**`backend/internal/run/history_writer.go`**

```go
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
```

**默认实现 `FSHistoryWriter`**：
- 依赖 `store.GetThread(threadID)` 拿 `WorkDir + HistoryPath`
- `os.OpenFile(path, O_APPEND|O_CREATE|O_WRONLY, 0644)`
- 每 thread 一个 `sync.Mutex`（`map[string]*sync.Mutex` + `sync.RWMutex`），串行化写入
- 编码：`json.Marshal(entry) + "\n"`

### 2.5 History 读回

`GET /threads/:id/history` 已存在（[thread_handler.go](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/backend/internal/httpapi/thread_handler.go#L83-L93)）。改动点：
- `ThreadService.History` 里损坏行 `json.Unmarshal` 失败改为 **跳过 + log warn**（当前是整体报错），保证部分历史可读
- 输出按 `seq` 排序（当前依赖 append 顺序，加显式排序防边界）

### 2.6 wiring

- `NewBus(store, historyWriter)` 构造函数注入
- `cmd/server/main.go` 或对应 bootstrap 里构造 `FSHistoryWriter(threadStore)` 并注入 Bus

### 2.7 边界

- 删除 thread 时 history.jsonl 一并删除（已有逻辑 `removeHistory` 保留）
- history.jsonl 每 thread 独立，无迁移；旧 project 首次 GET 返回空数组
- 不做压缩、不做分片（单 thread 通常 < 1MB）
- **append 失败不阻塞 SSE 主流程**：写入错误落 log + metrics，事件仍推给前端

---

## 3 · 前端 Store 责任线

### 3.1 runStore（#1 / #3 / #2 前端接线）

**新增 action 与副作用**：

```ts
createRun: async (threadId, payload) => {
  // (1) 立即插入 user_turn，不等后端 200
  const userItem: UserTurnItem = {
    id: `user_${Date.now()}_${Math.random().toString(36).slice(2)}`,
    type: 'user_turn',
    text: payload.instruction,
    timestamp: Date.now(),
  };
  updateSession(threadId, (prev) => ({ timelineItems: [...prev.timelineItems, userItem] }));

  // (2) 原有 flush / status 逻辑
  patchSession(threadId, { status: 'running', pendingInput: null, mode, scope, progress: null, plan: null });
  // ⚠️ 注意：不再 `timelineItems: []` 清空，改为保留刚插入的 user_turn

  // (3) POST /runs + subscribeRun
  const run = await runsApi.create(threadId, payload);
  // ...
}
```

**关键变更**：`createRun` 中原来的 `timelineItems: []` 清空要移除——user_turn 已经在 timeline 里了，清空会抹掉。改为仅清 `pendingInput/plan/progress`。

**done/error 触发 reload**：

```ts
if (event.event === 'done' || event.event === 'error') {
  get().sessions[threadId]?.eventSourceClose?.();           // 保持现有关闭逻辑
  patchSession(threadId, { eventSourceClose: null });
  const pid = useProjectStore.getState().activeProjectId;
  if (pid) useProjectStore.getState().loadProjectSlides(pid);   // 新增：自动刷新 slides
}
```

**新增 hydrateTimeline**：

```ts
hydrateTimeline: (threadId: string, items: TimelineItem[]) => {
  const existing = get().sessions[threadId]?.timelineItems ?? [];
  if (existing.length > 0) return;  // 防重
  patchSession(threadId, { timelineItems: items });
}
```

### 3.2 threadStore（#4）

**新增纯函数 `nextUntitledName`**：

```ts
nextUntitledName: (projectId: string) => {
  const all = [
    ...(get().threadsByProjectId[projectId] || []),
    ...(get().draftThreadsByProjectId[projectId] || []),
  ];
  const pattern = /^未命名 (\d+)$/;
  const maxN = all.reduce((max, t) => {
    const m = (t.title || '').match(pattern);
    return m ? Math.max(max, parseInt(m[1], 10)) : max;
  }, 0);
  return `未命名 ${maxN + 1}`;
}
```

**调用点替换**：
- `ensureActiveThread`：`createDraftThread(projectId, '主线程')` → `createDraftThread(projectId, get().nextUntitledName(projectId))`
- `ThreadTabs.handleNew`：`新对话 ${allThreads.length + 1}` → `nextUntitledName(activeProjectId)`

**序号单调**：删除不回收序号，因 `all` 是当前存在的 title 扫描——若把「未命名 3」删了，下次新建仍是「未命名 4」（历史最大 +1）。

### 3.3 deckStore（#5）

**新增字段**：`globalView: 'outline' | 'html'`（默认 `'html'`）

**effectiveView 逻辑重写**：

```ts
effectiveView: (slideId: string, hasHtml: boolean) => {
  const g = get().globalView;
  if (g === 'outline') return 'outline';
  return hasHtml ? 'html' : 'outline';   // html 全局但当页无 → fallback outline
}
```

`viewByPage` 字段保留（Phase 3 测试依赖），但不再参与决策。`setPageView` action 保留为 no-op（或标记 deprecated）以保持 API 兼容。

**新增 action**：`setGlobalView(view: PageView)` → `set({ globalView: view })`

### 3.4 前端 replay（#2）

**新文件 `frontend/src/features/agent/historyHydrator.ts`**：

```ts
export function hydrateFromHistory(entries: HistoryEntry[]): TimelineItem[] {
  return entries
    .sort((a, b) => a.seq - b.seq)
    .map((e) => toTimelineItem(e))
    .filter((x): x is TimelineItem => x !== null);
}

function toTimelineItem(e: HistoryEntry): TimelineItem | null {
  switch (e.type) {
    case 'user_turn':   return { id: `hist_${e.seq}`, type: 'user_turn', text: e.data.text, timestamp: e.ts * 1000 };
    case 'markdown':    return { id: `hist_${e.seq}`, type: 'markdown', text: e.data.text, timestamp: e.ts * 1000 };
    case 'info':        return { id: `hist_${e.seq}`, type: 'markdown', text: e.data.text, timestamp: e.ts * 1000 };
    case 'needs_input': return { id: e.data.id || `hist_${e.seq}`, type: 'needs_input', prompt: e.data.prompt, choices: e.data.choices, timestamp: e.ts * 1000 };
    case 'final_result':return { id: `hist_${e.seq}`, type: 'final_result', result: e.data.result, timestamp: e.ts * 1000 };
    case 'error':       return { id: `hist_${e.seq}`, type: 'error', code: e.data.code, message: e.data.message, timestamp: e.ts * 1000 };
    default: return null;
  }
}
```

**触发位点 `useActiveSession`**：

```ts
useEffect(() => {
  if (!threadId || isDraftId(threadId)) return;
  const session = useRunStore.getState().sessions[threadId];
  if (session && session.timelineItems.length > 0) return;
  threadsApi.history(threadId)
    .then((entries) => {
      if (entries.length === 0) return;
      useRunStore.getState().hydrateTimeline(threadId, hydrateFromHistory(entries));
    })
    .catch((err) => console.warn('history replay failed', err));
}, [threadId]);
```

---

## 4 · 前端 · 渲染与交互组件

### 4.1 Timeline user_turn 卡（#1 / #7）

**`eventReducer.ts` 类型扩展**：

```ts
export interface UserTurnItem extends BaseTimelineItem {
  type: 'user_turn';
  text: string;
}
export type TimelineItem = ... | UserTurnItem;
```

**Timeline.tsx switch 分支**：

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

Agent 消息保持左对齐 + `MarkdownMessage`（现有实现不变）。

### 4.2 其他文本卡 md 化（#7）

- `ThoughtCard`：内部 `text` 字段 → `<MarkdownMessage content={text} />`（当前用途是可视化 agent 思考，md 提升可读性）
- `FinalResultCard`：`result` 若是字符串 → md；若是对象 → 保留结构化展示
- `NeedsInputCard`：`prompt` → md（LLM 常在提示里带列表/代码块）
- `ToolCallCard / ArtifactCard / ErrorCard`：不改，保留结构化

### 4.3 OutlineCard 空态占位（#6）

**新增 `isEmpty` 判定**：

```tsx
const isEmpty = !content.title
  && !(content.bullets?.length)
  && !content.subtitle
  && !content.content_intent;
```

**compact（网格场景）+ isEmpty**：

```tsx
<div className="w-full h-full flex flex-col items-center justify-center bg-white p-3 relative">
  <span className="absolute top-3 left-3 px-2 py-0.5 rounded-full bg-background text-[10px] text-text-400 border border-border uppercase">
    {content.layout || slide.layout}
  </span>
  <div className="text-text-400 text-xs">Slide {(slide.order ?? slide.idx) + 1}</div>
  <div className="text-text-400 text-[10px] mt-1">未编辑大纲</div>
</div>
```

**主区（compact=false）+ isEmpty**：显示 `未命名` 大标题 + 中央引导语 `使用右侧对话或点击标题开始编辑`。

### 4.4 网格「暂无 HTML」徽标（#6 补充）

`PreviewWorkspace` 网格分支里，`!slide.html_path` 时在**左下角**追加：

```tsx
<div className="absolute bottom-2 left-2 bg-amber-500/80 text-white text-[10px] px-1.5 py-0.5 rounded backdrop-blur-sm">
  暂无 HTML
</div>
```

与现有右下角页码 badge 左右并列，视觉上一眼分辨"这页为什么和别的不同"。

### 4.5 PreviewWorkspace 工具栏

段控从 `viewByPage[currentSlide.id]` 改为 `globalView`：

```tsx
<button onClick={() => setGlobalView('outline')}>大纲</button>
<button onClick={() => setGlobalView('html')}>HTML</button>
```

HTML 按钮**不再 disable**（因为是全局切换）；退化的页面独立降级到 outline（由 `effectiveView` 兜底）。

---

## 5 · 数据流与错误处理

### 5.1 端到端时序

```
用户回车
  ↓
CommandComposer.handleSubmit
  ↓ (1) runStore.createRun 头部：insert user_turn        ← #1
  ↓ (2) draft flush（保持原有）
  ↓ (3) runsApi.create → POST /threads/:id/runs
  ↓
后端 RunEngine
  ↓ Bus.Publish('run.started', {user_input, mode, scope})
  ↓   → historyWriter.Append(turn=user, type=user_turn)  ← #2 落盘
  ↓ Bus.Publish('token' | 'info' | 'needs_input' | ...)
  ↓   → historyWriter.Append(白名单)
  ↓ Bus.Publish('done', {result})
  ↓   → historyWriter.Append(final_result)
  ↓
前端 SSE onMessage
  ↓ reduceSSEEvent → timelineItems 更新
  ↓ done → loadProjectSlides(activeProjectId)             ← #3

—— 页面刷新 ——

App mount → loadProjects → selectProject → loadThreads
  ↓
useActiveSession useEffect(threadId)
  ↓ if (session.timelineItems.length === 0 && !isDraft)
  ↓ threadsApi.history → hydrateFromHistory → runStore.hydrateTimeline  ← #2 回放
```

### 5.2 一致性与并发

- **user_turn 去重**：前端直插用 client id (`user_{ts}_{rand}`)；后端 replay 也生成 user_turn。`hydrateTimeline` 内置 `if (existing.length > 0) return`，保证正常运行时前端 in-memory 优先，只有刷新场景才 replay。
- **history append 并发**：per-thread mutex，串行化。运行时一个 thread 最多一个活跃 run（RUN_ACTIVE 互斥），实际竞争极少。
- **loadProjectSlides 抖动**：短时间多次 `done` 场景幂等——fetchClient 并发调用只是多几次 GET，可接受。
- **globalView 与 viewByPage**：`viewByPage` 不再参与决策，保留字段不清空以避免破坏 Phase 3 已有测试；未来 M8 可以整体清理。

### 5.3 错误处理

| 场景 | 策略 |
|---|---|
| history.jsonl append 失败 | log + 不阻塞 SSE 主流程；当次可用，重启后丢失几行 |
| history.jsonl 损坏行 | 跳过 + log warn；保证部分历史可读 |
| replay 空数据 | 不改 state，避免 UI 抖动 |
| replay 失败（网络/500） | console.warn；timeline 保持空态，用户可继续新 run |
| globalView 切 HTML 但当前页无 HTML | `effectiveView` fallback outline；工具栏 HTML 按钮不 disable |

### 5.4 测试策略

**后端**：
- `history_writer_test.go`：并发 append 顺序、per-thread 互斥、删损坏行跳过
- `thread_history_e2e_test.go`：跑一次 outline run，GET /threads/:id/history 断言含 user_turn + final_result
- `thread.go` History 单测：损坏行跳过 + 排序

**前端**：
- `runStore.test.ts`：`createRun 立即插入 user_turn`、`done 触发 loadProjectSlides`、`hydrateTimeline 幂等`
- `eventReducer.test.ts`：`user_turn` 类型正确
- `threadStore.test.ts`：`nextUntitledName 单调递增`、`ensureActiveThread 空 project 用「未命名 1」`
- `deckStore.test.ts`：`globalView='outline' 覆盖 hasHtml`、`globalView='html' 无 html 时 fallback outline`
- `historyHydrator.test.ts`：给 jsonl fixture，断言输出与 reduceSSEEvent 序列基本一致
- 手工：浏览器走查 7 个原始症状全部消除

---

## 6 · 里程碑分解

### 阶段 1：后端 · history 持久化（#2）
- 新增 `backend/internal/run/history_writer.go` + `_test.go`
- 修改 `bus.go`：`Publish` 后按白名单 append；构造函数注入 writer
- 修改 `engine.go`：`run.started` 事件 data 携带 `user_input`
- 修改 `service/thread.go`：History 损坏行跳过 + 按 seq 排序
- 修改 wiring（`cmd/server/main.go` 或等价 bootstrap）
- 新增 `backend/internal/httpapi/thread_history_e2e_test.go`
- 提交：`feat(ux-consistency): backend history append via bus`

### 阶段 2：前端 store · 三合一（#1 / #3 / #4 / #5）
- 修改 `frontend/src/stores/runStore.ts`：user_turn 头插 + done reload + hydrateTimeline
- 修改 `frontend/src/stores/threadStore.ts`：`nextUntitledName` + 替换 `'主线程'`
- 修改 `frontend/src/features/agent/ThreadTabs.tsx`：`handleNew` 用 `nextUntitledName`
- 修改 `frontend/src/stores/deckStore.ts`：`globalView` + `setGlobalView` + `effectiveView` 重写
- 更新对应三份 test
- 提交：`feat(ux-consistency): frontend store hooks for turn/reload/naming/global-view`

### 阶段 3：前端 · 渲染与 replay（#1 / #6 / #7 / #2 前端）
- 修改 `frontend/src/features/agent/eventReducer.ts`：`UserTurnItem` 类型
- 修改 `frontend/src/features/agent/Timeline.tsx`：`user_turn` case
- 新增 `frontend/src/features/agent/historyHydrator.ts` + `.test.ts`
- 修改 `frontend/src/features/agent/useActiveSession.ts`：useEffect 触发 replay
- 修改 `ThoughtCard.tsx / FinalResultCard.tsx / NeedsInputCard.tsx`：文本走 MarkdownMessage
- 修改 `frontend/src/features/viewer/OutlineCard.tsx`：`isEmpty` 分支占位
- 修改 `frontend/src/features/viewer/PreviewWorkspace.tsx`：段控接 `globalView` + `暂无 HTML` 徽标
- 提交：`feat(ux-consistency): frontend render user turn, md, grid placeholders, replay`

### 阶段 4：验收与收尾
- 全量 `go test ./...` + `npm test`
- 浏览器手动走查 7 项原始症状
- 更新 `README.md` / `docs/v2/*` 中相关章节（若有）
- 提交（若有文档更新）：`feat(ux-consistency): docs & manual verification`

---

## 7 · 非目标（本轮不做）

- 大纲/HTML 的分屏对比预览（Phase 3 已否决）
- 会话历史压缩、分片、导出、搜索
- 用户 turn 的 markdown 编辑器（当前 Enter 直发，Shift+Enter 换行）
- viewByPage 字段的最终清理（留给 M8）
- 多设备同步（history.jsonl 只服务本机）

---

## 8 · 术语

- **user_turn**：Timeline 中代表用户单次发言的 item 类型
- **history.jsonl**：每 thread 一个的 append-only 文件，存 SSE 事件白名单
- **hydrate**：将后端 history 记录还原为前端 timelineItems 的过程
- **globalView**：deckStore 中新增的全局大纲/HTML 视图偏好
