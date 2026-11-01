---
id: V2-INTERACTION-MODES
title: 四交互模式：呈现、切换与映射
status: superseded-by-r0
owner: frontend
depends_on: [V2-BACKGROUND, AGENT-CMD-INDEX]
verifies: [V2-G1, V2-G2]
---

# 四交互模式：呈现、切换与映射

> 本文保留为 R0 前的设计历史，不再是实现契约。当前权威协议见
> [40-api-and-data-contracts](40-api-and-data-contracts.md)：产物为蓝图/演示，层级为当前页/整份，
> 交互为执行/讨论；repo 是独立资产能力，不是 PPT target。

本文件定义 v2 前端如何把 v1 的 `kind + scope + mode` 折叠为用户可感知的**四档交互模式**，如何切换、
如何智能默认、如何精确映射到后端 Run 字段。这是修复"新建后动不了"与"四类交互无法切换"的核心设计。

## 1. 四档交互模式

| 模式 | 面向用户的语义 | 后端 kind | 后端 scope | 作用对象 |
|---|---|---|---|---|
| **Outline（大纲）** | 创建/编辑整份 PPT 的大纲结构 | `outline`（创建）/ `edit`（后续微调，见 §5） | `overview`(编辑时) | slide-json[] 大纲 |
| **Page（单页）** | 创建/编辑某一页的视觉与内容 | `generate`（重生成该页）/ `edit`（改该页） | `current` / `page` | 单页 html |
| **Overview（全局）** | 跨页/全局：换主题、统一样式、批量调整 | `edit` | `overview` | 公共层优先，必要时跨页 |
| **Repo（仓库）** | 管理个人资产（layout/component/theme/fx） | `edit` | `repo` | 仓库资产 |

> 关键点：**模式是前端概念**，是对后端两个正交维度（kind × scope）的**用户友好折叠**。后端契约不变。

### 1.1 为什么要折叠

v1 让用户直接面对 `/current /page /overview /repo` + `kind` 组合，心智负担高且前端从未接出。v2 用四个显式档位
覆盖 95% 场景，斜杠命令降级为高级用户的快捷方式（仍由后端 `command.Parse` 权威解析）。

## 2. 前端呈现：模式切换器（Mode Switcher）

在 `CommandComposer` 顶部放一个**分段切换器**（segmented control），四档 + 状态点：

```text
┌─────────────────────────────────────────────────────────────┐
│  ● 大纲 Outline   ○ 单页 Page   ○ 全局 Overview   ○ 仓库 Repo │  ← 模式切换器
├─────────────────────────────────────────────────────────────┤
│  [ 输入指令…（占位符随模式变化）                          ]  │  ← 输入区
│  上下文提示：当前第 3 页 · normal 模式         [Talk][Ask] ⏎ │  ← 上下文条 + 副模式
└─────────────────────────────────────────────────────────────┘
```

- **主切换器**：Outline / Page / Overview / Repo，互斥单选。
- **副模式开关**：Talk / Ask 作为可叠加的行为修饰（对应 `mode=talk/ask`），默认 normal。
  - Talk：只说不做（讨论方案）。Ask：遇歧义提问。
  - 二者互斥；再次点击取消回到 normal。
- **上下文条**：Page 模式显示当前锁定页号；其余模式显示作用域说明。
- **模式色**（仅用于状态点/边界，遵循 v1 视觉规范）：normal=sage、talk=indigo、ask=amber、repo=teal、overview=violet。

### 2.1 占位符文案（随模式变化，写给终端用户）

| 模式 | 占位符 |
|---|---|
| Outline（空项目） | 描述你要做的 PPT 主题，例如：给投资人讲我们的 AI 产品，8 页 |
| Outline（已有大纲） | 调整大纲结构，例如：把第 3、4 页合并；在结尾加一页总结 |
| Page | 编辑当前第 {n} 页，例如：把标题改大一号、配一张示意图 |
| Overview | 全局调整，例如：主色改成品牌蓝；所有页统一留白 |
| Repo | 管理资产，例如：把霓虹卡片组件圆角调大；新建一个深色主题 |

## 3. 智能默认（Smart Default）

模式切换器的**初始选中项**由项目状态推断，降低"选错模式"的概率。派生自 `projectStore` 的 slides/大纲状态：

```text
项目无大纲(slides 为空)             → 默认 Outline，且引导"先生成大纲"
项目有大纲、但当前页无 html         → 默认 Page（触发该页 generate）
项目有大纲、当前页有 html          → 默认 Page（edit 当前页）
用户手动切过模式                   → 尊重用户选择，不再自动跳（本次会话内记忆）
```

> 智能默认只影响**初始高亮**，用户永远可手动切换。这满足 `V2-G1`：新建后第一步天然落在 Outline，不再撞 `BAD_STATE`。

### 3.1 首次大纲的显式引导

空项目时，在预览区/Agent 区显示一个**空状态卡**：

```text
┌──────────────────────────────────────┐
│  ✨ 还没有内容                          │
│  先让 Agent 生成一份大纲，再逐页构建。   │
│                                        │
│  [ 描述主题并生成大纲 ]  ← 聚焦到 Outline │
└──────────────────────────────────────┘
```

点击后：模式锁 Outline，输入框聚焦，提交即发 `kind=outline`。

## 4. 意图 → Run 字段映射矩阵（权威）

前端根据 `(交互模式, 副模式, 项目状态, 当前页)` 组装 `RunPayload`。下表是**唯一事实源**，前端实现必须与之一致。

| 交互模式 | 副模式 | 项目状态 | → kind | → scope | → mode | → page_index | 其它字段 |
|---|---|---|---|---|---|---|---|
| Outline | normal | 无大纲 | `outline` | current | normal | — | `brief`,`slide_count`,`language` |
| Outline | normal | 有大纲 | `edit` | overview | normal | — | 指令为大纲级调整（见 §5 说明） |
| Outline | talk | 任意 | `command` | current | talk | — | `command=talk` |
| Outline | ask | 无大纲 | `outline` | current | ask | — | ask 先澄清再生成 |
| Page | normal | 当前页无 html | `generate` | current | normal | 当前页 | `theme`（可空） |
| Page | normal | 当前页有 html | `edit` | current | normal | 当前页 | — |
| Page（指定页） | normal | — | `edit`/`generate` | page | normal | 指定页 | — |
| Page | ask | — | `command`→edit | page/current | ask | 目标页 | `command=ask` |
| Page | talk | — | `command` | current | talk | — | `command=talk` |
| Overview | normal | 有页 | `edit` | overview | normal | — | service 就地补 `page_count` |
| Overview | talk | — | `command` | current | talk | — | `command=talk` |
| Repo | normal | — | `edit` | repo | normal | — | — |
| Repo | talk | — | `command` | current | talk | — | `command=talk` |

> 注：`kind=command` 是 talk/ask 的载体（v1 `service.buildCommandRunner` 以 `Mode` 为准路由）。前端发送时
> `kind=command`、`command=talk|ask`、`mode=talk|ask` 三者一致即可，后端已做冲突校验（`applyRawCommand`）。

### 4.1 斜杠命令（高级快捷，保留）

用户仍可在输入框行首打斜杠命令，前端**不本地重复解析语义**，而是：

1. 检测到行首 `/` → **不覆盖** payload 的 scope/mode（留空），把整行作为 `instruction` 发送。
2. 后端 `applyRawCommand`（`run_handler.go`）用 `command.Parse` 做权威解析并回填字段。
3. 前端切换器可选择"跟随"最终解析结果（见 §6 回显）。

这样前端只需一条规则："行首有 `/` 就把语义交给后端"，避免前后端双解析漂移（对齐 v1 `AGENT-CMD` 设计）。

## 5. Outline 编辑的语义说明（重要澄清）

v1 后端**没有独立的"大纲编辑" runner**：`kind=outline` 只做首次生成（`outline.Runner` 用 `submit_outline` 落库）。
对"已有大纲的结构调整"，v2 有两种可选实现，本文档推荐 **方案 A（前端先行、后端后补）**：

- **方案 A（v2 前端可先落地）**：Outline 模式在"有大纲"时，退化为 **Overview 编辑**（`kind=edit, scope=overview`），
  由用户自然语言驱动跨页/结构调整。**不阻塞前端交付**。
- **方案 B（后端增强，列入 V2-M4 可选项）**：新增 `outline edit` 能力（重跑 `submit_outline` 或提供 `patch_outline` 工具），
  支持增删页、重排序。契约见 [40-api-and-data-contracts](40-api-and-data-contracts.md#outline-编辑可选)。

> 前端实现按方案 A 映射即可；若后端落地方案 B，仅需把"有大纲的 Outline"改指向新 kind，切换器 UX 不变。

## 6. 状态机与回显

### 6.1 CommandComposer 本地状态

```ts
interface ComposerState {
  interactionMode: 'outline' | 'page' | 'overview' | 'repo';
  subMode: 'normal' | 'talk' | 'ask';
  targetPageIndex: number | null;   // Page 模式：null=当前页，number=指定页
  userTouchedMode: boolean;          // 用户是否手动切过（关掉智能默认）
  text: string;
}
```

### 6.2 提交流程（伪代码）

```text
onSubmit():
  1. thread = 确保当前有活跃 thread（多窗口见 20-multi-thread-windows）
  2. if text 行首以 '/' 开头:
        payload = { kind: 'edit'(占位), instruction: text }  // 语义交后端
     else:
        payload = mapModeToPayload(interactionMode, subMode, projectState, currentPage)
        payload.instruction = text
  3. runStore.createRun(thread.id, payload)     // 按 thread 隔离
  4. 清空输入
```

### 6.3 后端回显与前端同步

后端 `run.started` 事件返回权威 `{ kind, scope, mode }`。前端收到后：
- 更新该 thread 的 runStore（scope/mode）。
- 若与切换器不一致（用户打了斜杠命令），切换器**跟随回显**到解析后的模式（一次性，不再反向覆盖）。

## 7. 边界与错误友好化

| 场景 | v1 行为 | v2 前端处理 |
|---|---|---|
| 空项目发 Page/Overview | 后端 `BAD_STATE` | 前端**前置拦截**：智能默认已引导 Outline；若用户强切，给内联提示"请先生成大纲" |
| Page 指定越界页 | 后端 400 `invalid page_index` | 前端用 slides 数量前置校验，禁用提交并提示 |
| Repo 无资产可改 | 后端 runner 处理 | 前端在 Repo 模式提供"新建资产"引导 |
| `/page` 缺页号 | 后端 `ErrPageNeedsIndex` | 前端斜杠场景交后端报错，展示错误卡 |

## 8. 验收标准（Given-When-Then）

- **AC-V2-MODE-001**（`V2-G1`）
  - GIVEN 一个新建的空 project
  - WHEN 打开工作台
  - THEN 模式切换器默认高亮 Outline，输入并提交后发出 `kind=outline`，不出现 `BAD_STATE`

- **AC-V2-MODE-002**（`V2-G2`）
  - GIVEN 项目已有大纲与页
  - WHEN 用户把切换器切到 Overview 并提交"主色改品牌蓝"
  - THEN 发出 `kind=edit, scope=overview`，`run.started` 回显 `scope=overview`

- **AC-V2-MODE-003**（`V2-G2`）
  - GIVEN 用户在 Page 模式
  - WHEN 输入 `/repo 把卡片圆角调大`
  - THEN 前端不本地改写 scope，后端解析为 `scope=repo`，切换器回显跟随为 Repo

- **AC-V2-MODE-004**（`V2-G1`）
  - GIVEN 当前页尚无 html
  - WHEN Page 模式提交"配一张示意图"
  - THEN 发出 `kind=generate, scope=current, page_index=当前页`

## 9. 校验方式

```bash
cd frontend
pnpm test   # CommandComposer.test.tsx：覆盖映射矩阵各行、智能默认、斜杠不覆盖
pnpm tsc --noEmit
```

## 10. 依赖

- [V2-BACKGROUND](00-background-and-goals.md)、[AGENT-CMD-INDEX](../v1/50-agent/commands/README.md)、[40-api-and-data-contracts](40-api-and-data-contracts.md)
