---
id: V2-FRONTEND-ARCH
title: 前端架构 v2 升级
status: draft
owner: frontend
depends_on: [ARCH-FRONTEND, V2-INTERACTION-MODES, V2-MULTI-THREAD, V2-CONTRACTS]
verifies: [V2-G1, V2-G2, V2-G3, V2-G4]
---

# 前端架构 v2 升级

> R0 状态（2026-08-01）：Composer 已从旧四模式/action 映射切换为正交的 `artifact/level/interaction`。本文中的旧 modeMapping 设计已由下述 Artifact Target 实现取代。

本文件给出前端为承载 v2（四模式切换、多 thread 窗口、plan 流水线可视化）所需的目录/store/组件/reducer 升级。
在 v1 三栏工作台之上做**增量改造**，不重写。

## 1. 现状与改造点速查

| 文件 | v1 现状 | v2 改造 |
|---|---|---|
| `api/types.ts` | 旧 RunPayload 混合 action/scope/mode | `RunTarget`、`RunInteraction`、Blueprint 与 revision 类型 |
| `api/runs.ts` | 只有 create/input/cancel | 不变（已够）；新增 thread history 拉取（api/threads.ts） |
| `stores/runStore.ts` | 全局单例 | **按 threadId 分片**（见 [20-multi-thread-windows](20-multi-thread-windows.md#32-runstore按-threadid-分片核心重构)） |
| `stores/projectStore.ts` | threads 仅缓存 | 扩为 thread 打开态/聚焦态管理 |
| `features/agent/CommandComposer.tsx` | 硬编码 generate/current | **蓝图/演示 × 当前页/整份 × 执行/讨论** |
| `features/agent/eventReducer.ts` | 无 plan 分支 | 新增 plan/plan.update 归并；artifact design_spec |
| `features/agent/AgentPanel.tsx` | 单一 timeline | 顶部 Thread 标签页 + 按聚焦 thread 派生 |
| `features/agent/PlanCard.tsx` | 前端 mock | 接真实 plan 状态 |

## 2. 目录结构（增量）

```text
frontend/src/
├── api/
│   ├── threads.ts            # 新增：list/create/history/delete thread
│   ├── blueprints.ts         # Blueprint 聚合与 optimistic revision API
│   └── types.ts              # RunTarget、RunInteraction、Blueprint、materialization
├── stores/
│   ├── runStore.ts           # 重构：sessions: Record<threadId, RunSession>
│   ├── blueprintStore.ts     # project Blueprint 聚合与精确失效
│   ├── projectStore.ts       # 扩：委托 thread 管理或拆出 threadStore
│   └── threadStore.ts        # 新增（可选）：thread 打开态/聚焦态
├── features/
│   ├── agent/
│   │   ├── ThreadTabs.tsx     # 新增：Agent 面板内多 thread 标签
│   │   ├── ModeSwitcher.tsx   # 产物、层级、交互、clarification
│   │   ├── CommandComposer.tsx# 发送纯新协议并解析 stable slide_id
│   │   ├── PlanCard.tsx       # 接真实 plan 事件
│   │   ├── eventReducer.ts    # 扩：plan/plan.update/design_spec
│   │   └── modeMapping.ts     # 新增：意图→RunPayload 纯函数（可单测）
│   └── viewer/
│       ├── SlideBlueprintCard.tsx
│       ├── DesignSpecSummary.tsx
│       └── MaterializationBadge.tsx
```

## 3. 关键模块设计

### 3.1 Artifact Target 构造

把 [10-interaction-modes §4 映射矩阵](10-interaction-modes.md#4-意图--run-字段映射矩阵权威) 落成一个纯函数，便于单测：

```ts
interface TargetInput {
  artifact: 'blueprint' | 'presentation';
  level: 'slide' | 'deck';
  intent: 'apply' | 'consult';
  clarification: 'when_blocked' | 'before_apply' | 'never';
  selectedSlideId?: string;
  instruction: string;
}

function createTargetedRun(input: TargetInput): CreateRunRequest { /* validate stable ID */ }
```

- 单测覆盖四个 artifact × level 组合，以及 consult 与 clarification。
- `current` 只属于 UI，构造请求时必须替换为 store 中的稳定 slide ID。
- 斜杠快捷方式若保留，只填充 target/interaction，不创建第二套协议。

### 3.2 ModeSwitcher.tsx

- 四档分段控件 + Talk/Ask 副开关（互斥、可取消）。
- 受 `smartDefault(projectState)` 初始化；`userTouchedMode` 后不再自动跳。
- 模式色仅用于状态点/边界（sage/violet/teal/indigo/amber）。
- 暴露 `onChange(interactionMode, subMode)`，写入当前聚焦 thread 的 composer 状态。

### 3.3 ThreadTabs.tsx

- 渲染当前 project 的**打开态** thread；每个 tab 显示标题 + run 状态微点。
- 交互：切换聚焦、[+] 新建、× 关闭（可选删除二次确认）、溢出下拉打开历史 thread。
- 数据源：projectStore/threadStore 的打开态；状态点来自 `runStore.getSession(threadId).status`。

### 3.4 runStore 分片（核心）

见 [20-multi-thread-windows §3.2](20-multi-thread-windows.md#32-runstore按-threadid-分片核心重构)。补充实现要点：

- `RunSession` 新增 `plan: PlanState | null`。
- SSE 回调闭包捕获 `threadId`，`set` 时只改 `sessions[threadId]`（用 immer 或手写不可变更新，避免误写别的分片）。
- `getSession(threadId)` 对缺失 key 返回稳定的 idle 空会话常量（避免组件读到 undefined）。

### 3.5 eventReducer v2（新增分支）

在 v1 reducer 基础上新增：

```ts
case 'plan':
  // 创建/替换 PlanItem（该 run 仅一个）
  return upsertPlan(state, { id, title, steps });

case 'plan.update':
  // 按 step_id 更新对应 step 状态；step_id 未命中则忽略（V2-SSE-002）
  return updatePlanStep(state, { id, step_id, status, detail });

case 'artifact':
  // artifact_type==='design_spec' → 渲染 DesignSpecCard（中间产物，弱强调）
  // 其余沿用 v1 归并到最近 tool_call
```

- **plan 与 progress 并存**：progress 更新 RunSummary 进度条；plan 驱动 PlanCard 步骤清单，二者独立。
- token 流式合并沿用 v1（50-100ms 节流建议在 store 层做）。

### 3.6 PlanCard 接真实数据

- 从聚焦 thread 的 `session.plan` 读取 steps；每 step 显示状态图标（pending/in_progress/completed/failed/skipped）。
- 支持折叠；in_progress step 高亮。
- v1 的 mock 数据路径删除。

### 3.7 EmptyState（修复"新建后动不了"的 UX 出口）

- 当聚焦 project 无 slides：中间预览区/Agent 区显示"生成大纲"引导卡（见 [10-interaction-modes §3.1](10-interaction-modes.md#31-首次大纲的显式引导)）。
- 点击 → 锁 Outline 模式 + 聚焦输入框。

## 4. 数据流（端到端）

```text
ModeSwitcher/输入 ──▶ CommandComposer
   │  current → stable slide_id
   ▼
runStore.createRun(activeThreadId, CreateRunRequest)
   │  POST /threads/{id}/runs → { run.id, events_url }
   ▼
runStore.subscribeRun(threadId, runId)  ── EventSource ──▶ sessions[threadId]
   │  每事件 → eventReducer v2 → timelineItems / plan / progress
   ▼
AgentPanel(聚焦 thread) 派生渲染：
   Timeline(卡片) + PlanCard(真实步骤) + RunSummary(progress) + FinalResultCard(done)
```

## 5. 视觉与性能（沿用 v1 规范）

- 低饱和专业配色；模式色只用于状态点/标签/卡片左边界（m7-design-spec §7）。
- SSE 增量 reducer；token 节流合并；稳定 Markdown memo；总览缩略图按需。
- 切 project 关闭该 project 下所有 EventSource；切 thread 不关连接（后台 run 继续）。
- 预览 iframe 翻页仍走 postMessage，不改 src（ARCH-FE-002 不变）。

## 6. 测试要求（在 v1 基础上补）

| 测试 | 覆盖 |
|---|---|
| `modeMapping.test.ts` | 映射矩阵每一行、智能默认、talk/ask 分支 |
| `runStore.multithread.test.ts` | 多 thread 分片隔离、SSE 不串线、getSession 缺省 |
| `eventReducer.test.ts`（扩） | plan 创建、plan.update 命中/未命中、design_spec 归类 |
| `CommandComposer.test.tsx`（扩） | 斜杠不本地改写、模式提交正确 payload |
| `PlanCard.test.tsx` | 五种 step 状态渲染、折叠 |
| `ThreadTabs.test.tsx` | 切换/新建/关闭、状态点 |

## 7. 验收标准（Given-When-Then）

- **AC-V2-FE-001**（`V2-G1`）
  - GIVEN 新建空 project
  - WHEN 渲染工作台
  - THEN 显示 EmptyState 引导，模式默认 Outline，提交发 `kind=outline`

- **AC-V2-FE-002**（`V2-G4`）
  - GIVEN 整套生成的 SSE 流含 plan/plan.update
  - WHEN 前端消费
  - THEN PlanCard 显示真实步骤并随 plan.update 逐步 completed，进度条按 progress 更新

- **AC-V2-FE-003**（`V2-G3`）
  - GIVEN 两个打开的 thread
  - WHEN 各自发起 run
  - THEN 两个 timeline/plan 互不污染，标签状态点独立

## 8. 校验方式

```bash
cd frontend
pnpm test
pnpm tsc --noEmit
pnpm build
```

## 9. 依赖

- [ARCH-FRONTEND](../v1/20-architecture/frontend-structure.md)、[V2-INTERACTION-MODES](10-interaction-modes.md)、[V2-MULTI-THREAD](20-multi-thread-windows.md)、[V2-CONTRACTS](40-api-and-data-contracts.md)
