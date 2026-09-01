---
id: V2-MULTI-THREAD
title: 多对话窗口（Thread 标签页）
status: draft
owner: frontend
depends_on: [V2-BACKGROUND, ADR-0010, API-REST]
verifies: [V2-G3]
---

# 多对话窗口（Thread 标签页）

需求 2：一个 presentation 支持多个独立对话窗口，每个窗口独立交互。本文件给出前端实现方案——
**Agent 面板内多 Thread 标签页**（用户确认的形态），以及为支撑它所需的 store 隔离重构。

## 1. 后端能力现状（无需改造）

Thread 语义在 v1 已完整实现，v2 前端直接复用：

| 能力 | 端点 | 说明 |
|---|---|---|
| 列出项目的 thread | `GET /projects/{id}/threads` | 返回 Thread[]（含 title/status/时间戳） |
| 新建 thread | `POST /projects/{id}/threads` | body 可带 `{title}`，返回新 Thread |
| 读取 thread 历史 | `GET /threads/{id}/history` | 恢复对话（关闭再打开接着聊） |
| 删除 thread | `DELETE /threads/{id}` | 不影响 PPT 产物（Thread 隔离历史、共享产物） |
| 在 thread 下发起 run | `POST /threads/{id}/runs` | 每次交互挂在具体 thread 下 |

隔离模型（v1 ADR-0010）：**Project → Thread → Run 三层**。Thread 隔离对话历史，但共享同一 project 的 slide 产物。
这正是"多个对话窗口协作同一份 PPT"的语义基础。

## 2. 前端形态：Agent 面板内 Thread 标签页

右侧 Agent 面板顶部新增一行 **Thread 标签页**（JetBrains 风格），一次聚焦一个 thread，切换零重载：

```text
┌───────────────────────────────────────────┐
│ Agent                          [normal ●]  │  ← 面板头
├───────────────────────────────────────────┤
│ ▸ 主线程 ×  内容打磨 ×  配色实验 ×    [ + ] │  ← Thread 标签页（本文件核心）
├───────────────────────────────────────────┤
│                                             │
│   ← 当前聚焦 thread 的 Timeline（独立）     │
│                                             │
├───────────────────────────────────────────┤
│   模式切换器 + 输入区（作用于当前 thread）  │
└───────────────────────────────────────────┘
```

- **一次聚焦一个**：切换 tab 即切换该 thread 的 timeline / run 状态 / 输入上下文。
- **切换零重载**：各 thread 的状态常驻内存，切换只换视图指针（不重新拉历史，除非首次打开）。
- **独立运行**：thread A 正在 running 时，切到 thread B 可并行发起新 run，互不干扰。
- **标签状态点**：每个 tab 右侧显示该 thread 的 run 状态微点（running=脉冲、needs_input=amber、error=red）。
- **[+] 新建**：调用 `POST /projects/{id}/threads`，可选命名（默认"新对话 N"）。
- **关闭 ×**：仅从视图移除标签（前端"打开态"），可选是否 `DELETE`（二次确认，因删历史不可逆）。

### 2.1 打开态 vs 存在态

区分两个概念，避免"标签太多"：
- **存在态**：该 project 下后端所有 thread（`GET .../threads`）。
- **打开态**：用户在 Agent 面板顶部**当前展开为标签**的 thread 子集（前端 UI 状态）。

未打开的历史 thread 通过面板的"历史/更多"下拉进入（点击后加入打开态）。

## 3. Store 重构：从单例到按 thread 隔离（P0 前端改造）

v1 `runStore` 是**全局单例**，无法并存多对话。v2 拆成两层：

### 3.1 `threadStore`（新增/扩展 projectStore）

```ts
interface ThreadUIState {
  // 存在态：来自后端
  threadsByProjectId: Record<string, Thread[]>;
  // 打开态：每个 project 当前展开的标签顺序
  openThreadIdsByProjectId: Record<string, string[]>;
  // 每个 project 当前聚焦的 thread
  activeThreadIdByProjectId: Record<string, string | null>;

  loadThreads(projectId: string): Promise<void>;
  openThread(projectId: string, threadId: string): void;
  closeThread(projectId: string, threadId: string): void;
  createThread(projectId: string, title?: string): Promise<string>;
  setActiveThread(projectId: string, threadId: string): void;
  ensureActiveThread(projectId: string): Promise<string>; // 无则自动建"主线程"
}
```

### 3.2 `runStore`：按 threadId 分片（核心重构）

把当前所有全局字段收进一个 `Record<threadId, RunSession>`：

```ts
interface RunSession {
  activeRunId: string | null;
  status: 'idle' | 'running' | 'done' | 'error' | 'needs_input';
  mode: string;
  scope: RunScope;
  timelineItems: TimelineItem[];
  pendingInput: { id: string; prompt: string; choices?: string[] } | null;
  progress: { stage: string; current: number; total: number } | null;
  eventSourceClose: (() => void) | null;
  plan: PlanState | null;         // v2 新增，见 30-agent-pipeline-v2
}

interface RunStoreV2 {
  sessions: Record<string, RunSession>;   // key = threadId

  createRun(threadId: string, payload: RunPayload): Promise<void>;
  subscribeRun(threadId: string, runId: string, lastEventId?: string): void;
  replyNeedsInput(threadId: string, runId: string, replyTo: string, content: string): Promise<void>;
  cancelRun(threadId: string, runId: string): Promise<void>;
  getSession(threadId: string): RunSession;   // 缺省返回 idle 空会话
}
```

**关键约束**：
- 所有 action 首参为 `threadId`；SSE 回调 `set` 只更新对应 `sessions[threadId]` 分片，绝不串写别的 thread。
- 组件通过 `getSession(activeThreadId)` 派生视图（对齐 v1 `ARCH-FE-004` 单一来源）。
- 切换聚焦 thread **不关闭**其它 thread 的 EventSource——后台 run 继续推进，标签状态点实时更新。

## 4. SSE 多路复用

多个 thread 可能同时有活跃 run，各自持有独立 `EventSource`：

```text
thread A ── EventSource(runA) ──▶ sessions[A]
thread B ── EventSource(runB) ──▶ sessions[B]   （并行，互不阻塞）
```

约束：
- 每个 run 一条连接，事件只写入自己的 session 分片。
- 终态（done/error）后关闭该连接（沿用 v1 逻辑）。
- **切换 project 时**：关闭当前 project 下所有 thread 的连接（避免离开后仍串事件，对齐 m7-design-spec §6.1"取消旧项目的非当前 SSE 订阅"）；重新进入时按打开态恢复订阅活跃 run。
- 断线重连带 `Last-Event-ID`，按 thread 各自续传。

## 5. 生命周期与默认行为

| 事件 | 行为 |
|---|---|
| 首次进入 project | `loadThreads` → 若为空则 `ensureActiveThread` 自动建"主线程"并打开；否则打开最近活跃 thread |
| 发起交互但无活跃 thread | `ensureActiveThread` 兜底（替代 v1 `threads[0]` 硬取） |
| 新建 thread | 立即加入打开态并聚焦；空 timeline |
| 关闭标签 | 仅移出打开态；若关的是当前聚焦，则聚焦到相邻标签 |
| 切换 project | 保存各 project 打开态；恢复上次聚焦 thread |

## 6. 与交互模式的关系

- **模式切换器属于"当前聚焦 thread"**：每个 thread 可独立记忆自己的 interactionMode/subMode（例如 thread A 专做 Page 编辑、thread B 专做 Overview 调色）。
- 提交时用当前聚焦 thread 的 `threadId` + 该 thread 的模式映射。

## 7. 验收标准（Given-When-Then）

- **AC-V2-THREAD-001**（`V2-G3`）
  - GIVEN 一个 project 已有 1 个 thread
  - WHEN 点击 [+] 新建第 2 个 thread
  - THEN 两个 thread 标签并存，切换互不清空各自 timeline

- **AC-V2-THREAD-002**（`V2-G3`）
  - GIVEN thread A 有一个 running 的 run
  - WHEN 切到 thread B 并发起另一个 run
  - THEN 两个 run 各自 SSE 推进，A 的状态点仍显示 running，B 独立展示

- **AC-V2-THREAD-003**（`V2-G3`）
  - GIVEN 切换到另一个 project
  - WHEN 返回原 project
  - THEN 原打开态标签与聚焦 thread 恢复，活跃 run 按 Last-Event-ID 续传

- **AC-V2-THREAD-004**
  - GIVEN 进入一个从无 thread 的 project
  - WHEN 首次发起交互
  - THEN 自动创建"主线程"并在其下发起 run（不再 `threads[0]` 取 undefined）

## 8. 校验方式

```bash
cd frontend
pnpm test    # runStore 分片：多 thread 状态隔离；ensureActiveThread；SSE 不串线
pnpm tsc --noEmit
# e2e（可选）：新建第2 thread → 两 run 并行 → 状态点独立
```

## 9. 依赖

- [V2-BACKGROUND](00-background-and-goals.md)、[ADR-0010](../v1/90-decisions/0010-project-thread-run-isolation.md)、[API-REST](../v1/40-api/rest-endpoints.md)、[50-frontend-architecture-v2](50-frontend-architecture-v2.md)
