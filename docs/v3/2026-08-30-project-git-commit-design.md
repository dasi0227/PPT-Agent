# Project Git Commit Design

> 日期：2026-08-30  
> 状态：Accepted，待实现  
> 范围：Project 独立 Git 仓库、右侧 Agent 面板 Commit 快捷入口、Commit LLM、操作事件、Thread Timeline 投影、并发与失败恢复  
> 关联：承接 `2026-08-05-direct-write-refactor-with-budget-gate-tuning-design.md` 延后的 Git 安全网；不恢复已废弃的 Run staging 事务

## 1. 决策

每个 Project 的 `work_dir` 是一个独立 Git 仓库。用户可以从右侧 Agent 面板标题栏直接提交该 Project 当前全部未提交内容：

```text
Project work_dir
  -> Git 收集全部未提交变更
  -> 当前 Composer 模型生成结构化提交说明
  -> 后端执行一次 Git commit
  -> 成功结果进入当前 Thread Timeline
```

Commit 是 Project 级独立操作，不是用户消息，不创建 Run，不进入 Agent Runtime，不产生计划、推理、工具活动或最终回答，也不调用 Runtime 的 `finish`。

Commit 操作使用独立的 `git.commit.*` 操作事件协议。不得把它伪装成 `run.progress`、`tool.started` 或 `tool.completed`，也不得扩展 Run 公共事件 Schema v3 来承载非 Run 任务。

只有后端掌握 Git 与操作状态事实。前端不推断 Git 阶段、不模拟百分比，也不自行拼装提交标题、items、hash 或变更统计。

## 2. 目标与非目标

### 2.1 目标

1. 为每个 Project 提供独立、可审计的 Git 版本历史。
2. 一次提交当前 Project 仓库中全部 tracked 和未忽略的 untracked 变更，不提供文件选择。
3. 使用用户当前 Composer 选择的 LLM Profile 生成规范标题和无序列表 items。
4. 以真实后端边界投影 `staging -> analyzing -> committing` 三阶段进度。
5. 成功 Commit 作为独立系统事件持久关联当前 Thread，并可在历史恢复后重新投影。
6. Commit 与 Run、结构化 mutation 和其他 Project 写操作互斥，确保分析和提交的是同一份文件快照。
7. 任一步骤失败时不产生半提交；模型输出无效时最多尝试三次。

### 2.2 非目标

- 不提供 amend、rebase、merge、push、pull、checkout、branch 创建或远端仓库管理。
- 不让 Agent Runtime 获得 Git 写命令或通用 shell 能力。
- 不提供文件级勾选、暂存区编辑或提交信息手动编辑器。
- 不用 Git 立即替换现有 `versions` 表或现有页面回滚 UI。
- 不把 Commit 变成普通对话消息，也不要求用户发送额外确认。
- 不提交应用数据库、服务日志、临时文件或 Thread JSONL。

## 3. 仓库边界与初始化

### 3.1 仓库根目录

Git 根目录固定为 Project 的权威 `work_dir`：

```text
<work_root>/projects/<project_id>/.git
```

后端从 Store 读取 Project，再使用其 `WorkDir`；客户端不得提交路径。所有 Git 子进程必须显式设置工作目录，禁止接受用户提供的 Git 参数或路径。

### 3.2 新 Project

`ProjectService.CreateProject` 在领域文件创建完成后初始化仓库：

```text
git init --initial-branch=main
git add -A
git commit -m "chore: init project"
```

创建 Project 时自动提交系统生成的项目脚手架，固定标题为 `chore: init project`。该初始化提交不调用 LLM、不创建 Git Commit Operation，也不进入 Thread Timeline。用户首次手动 Commit 只处理初始化完成后的真实创作变更；若没有变更则进入 `empty` 终态。

Git identity 通过子进程环境或命令级参数提供，不写用户全局/本地 Git config：

```text
GIT_AUTHOR_NAME=PPT Agent
GIT_AUTHOR_EMAIL=ppt-agent@local
GIT_COMMITTER_NAME=PPT Agent
GIT_COMMITTER_EMAIL=ppt-agent@local
```

名称和邮箱可以由服务配置覆盖，但不得从模型输出或请求体读取。

### 3.3 既有 Project

开发期不保留旧协议兼容层，但现有本地 Project 可能早于本设计。第一次读取 Commit 能力时，后端执行一次幂等 bootstrap：

1. 若 `work_dir/.git` 存在，验证仓库根目录正是 Project `work_dir`。
2. 若不存在，在该目录初始化 `main` 仓库。
3. 若目录位于其他仓库内部但自身不是仓库根，拒绝提交，不能误提交父仓库。
4. 若仓库处于 detached HEAD、merge、rebase、cherry-pick 或 unresolved conflict 状态，返回安全错误，不尝试自动修复。

### 3.4 `.gitignore`

“提交全部变更”指仓库中全部 tracked 和未忽略的 untracked 项目内容。以下运行数据不属于项目内容版本，必须由初始化模板统一忽略：

```gitignore
threads/
.run/
.commit-tmp/
*.tmp
```

Thread History 被忽略是硬性要求。否则每次对话都会修改 `threads/<thread_id>.jsonl`，导致刚完成 Commit 后仓库仍立即变脏，并把用户对话正文发送给 Commit LLM。

已有 Project bootstrap 时只补齐产品拥有的 ignore 条目，不覆盖用户已有 `.gitignore` 内容。

## 4. 产品交互

### 4.1 入口

右侧 Agent 面板标题栏增加 Git Commit 图标按钮：

- 位于“折叠面板”按钮左侧；
- 使用 `GitCommitHorizontal` 图标和绿色强调色；
- tooltip 为“提交项目版本”；
- 只有存在当前 Project 和当前 Thread 时可用；
- active Run、active Commit 或 Project 写操作期间禁用。

点击 Commit 不发送 Composer 文本。输入框内容、选区和焦点不被清空。

### 4.2 提交期间

Commit 开始后：

- Commit、发送、Polish 按钮禁用；
- Composer 文本仍可编辑；
- 不允许创建新 Run、steering 或其他 Project mutation；
- Project 读取、页面浏览和 Timeline 展开仍可使用。

进度条占满 Timeline 可用宽度。三个标签与圆点中心严格对齐：

```text
整理变更 -------- 生成说明 -------- 写入版本
```

视觉规则：

- pending：灰色空心圆与灰色连线；
- active：绿色空心圆；
- completed：绿色实心圆，已经经过的连线填充绿色；
- 不显示百分比、预计时间或循环动画进度；
- 前端只根据最新 `git.commit.progress.phase` 更新，不使用计时器推进状态。

阶段展示文案固定由前端映射，服务端只发送枚举：

| phase | UI 文案 |
| --- | --- |
| `staging` | 整理变更 |
| `analyzing` | 生成说明 |
| `committing` | 写入版本 |

### 4.3 无变更

后端确认仓库没有可提交内容后发送 `git.commit.empty`。前端：

1. 移除进度条；
2. 恢复所有被禁用控件；
3. 不新增 Timeline 项；
4. 通过应用级 Toast portal 在整个 viewport 顶部中央显示：

```text
当前项目没有可提交的变更
```

该 Toast 使用中性样式，不使用错误红色，也不持久化到 Thread。

### 4.4 成功

成功后在当前 Thread Timeline 追加一个 Commit 项。默认折叠，使用绿色 Commit 图标。

折叠态：

```text
fix: 优化增长图表标签布局
2026-08-30 14:29 | main 8af42d9 | 3 files +46 -18
```

元信息分为三个视觉组，使用低对比度细竖线分隔：

1. 本地化后的完整日期时间；
2. branch 与 short commit hash；
3. 文件数、insertions 和 deletions。

展开态只显示模型生成的 `items` 无序列表：

```text
- 强化第三页季度增长趋势的视觉层级
- 调整数据标签位置并消除内容重叠
```

不显示逐文件列表、底部 branch/hash 重复信息、复制按钮或额外解释。

### 4.5 失败

失败后移除进度条、恢复控件，并在当前 Thread Timeline 追加持久失败项：

```text
项目版本提交失败
2026-08-30 14:31 | 提交失败，请重新尝试或手动提交    [Retry]
```

失败项：

- 使用与成功项相同的 Commit 图标，仅改为红色描边、浅红背景和红色状态；
- 不显示 branch、hash、文件数或增删统计，因为没有产生可确认的新版本；
- 不显示第二行错误详情；
- 右侧使用 `RotateCw` 图标重试按钮，tooltip 为“重新提交”；
- 不展示 Provider 原始错误、模型输出、文件路径、Git stderr 或内部命令。

重试创建新的 Commit Operation。旧失败项保持在 Timeline 中，不原地改写为成功。

### 4.6 Toast 定位

所有 Toast 使用现有应用级 Toast 容器或 portal，定位基于整个 viewport，不相对于右侧 Agent 面板。讨论阶段的右栏独立 HTML 仅用于聚焦交互，不代表最终定位容器。

## 5. 操作状态机

### 5.1 状态

```text
accepted
  -> staging
      -> empty
      -> analyzing
          -> committing
              -> completed
  -> failed
```

终态为：

```text
empty | completed | failed
```

每个 Operation 恰好进入一个终态。`completed` 只能发生在 Git Commit 已成功且元信息读取完成后。

### 5.2 阶段真实边界

#### `staging`

在以下条件全部成立后发送：

1. Project、Thread 和 LLM Profile 已验证；
2. Thread 属于 Project；
3. 当前 Project 没有 active Run、active Commit 或其他写操作；
4. 已获得 Project 独占写锁；
5. 仓库状态允许提交。

发送后，后端开始检查工作区、构建隔离 index，并执行等价于 `git add -A` 的全量暂存。

#### `analyzing`

只在以下动作成功后发送：

1. 全量暂存成功；
2. staged diff、numstat 和文件状态已读取；
3. 确认至少存在一个可提交变更；
4. Commit LLM 输入已完成安全裁剪。

发送后开始调用当前 Profile。模型输出格式无效时最多重试三次，整个重试过程仍停留在 `analyzing`，不重复制造阶段事件。

#### `committing`

只在以下动作成功后发送：

1. 模型返回且只返回一个 `git_commit` tool call；
2. `title` 和 `items` 通过结构、长度和字符校验；
3. 即将执行唯一一次真实 `git commit`。

不得在模型请求开始前发送 `committing`，也不得把模型重试显示为 Git 写入。

#### `completed`

在以下动作全部完成后发送：

1. `git commit` 返回成功；
2. branch、short hash、committed timestamp 和统计读取成功；
3. Commit Operation 终态与 Thread 关联记录已持久化。

前端收到后将三节点全部填绿，并立即用成功 Commit 项替换 live progress。

## 6. API 与事件协议

### 6.1 创建 Operation

```http
POST /api/v1/projects/:project_id/git-commits
Content-Type: application/json
```

请求：

```json
{
  "thread_id": "thread_123",
  "model": "Kimi K3",
  "client_request_id": "req_123"
}
```

规则：

- route `project_id` 是 Project 身份真相；
- `thread_id` 必须属于该 Project；
- `model` 必须精确匹配 LLM Registry Profile name；
- `client_request_id` 用于同一 Thread 内创建操作幂等；
- 请求不接受 commit message、文件列表、路径、Git 参数或 diff。

响应：

```json
{
  "id": "gco_123",
  "project_id": "pro_123",
  "thread_id": "thread_123",
  "status": "accepted",
  "events_url": "/api/v1/git-commits/gco_123/events"
}
```

HTTP 状态为 `202 Accepted`。创建成功后后台执行操作，客户端立即订阅事件。

创建前置失败使用常规 API Error：

| code | HTTP | 含义 |
| --- | ---: | --- |
| `PROJECT_NOT_FOUND` | 404 | Project 不存在 |
| `THREAD_NOT_FOUND` | 404 | Thread 不存在 |
| `THREAD_PROJECT_MISMATCH` | 422 | Thread 不属于 Project |
| `MODEL_PROFILE_NOT_FOUND` | 422 | Profile 不存在 |
| `MODEL_TOOL_CALL_UNSUPPORTED` | 422 | 当前 Profile 不支持 tool call |
| `RUN_ACTIVE` | 409 | Project 有 active Run |
| `GIT_COMMIT_ACTIVE` | 409 | Project 已有 Commit Operation |
| `PROJECT_WRITE_ACTIVE` | 409 | Project 有其他互斥写操作 |

### 6.2 查询 Operation

```http
GET /api/v1/git-commits/:operation_id
```

返回权威状态、当前 phase 以及终态结果。该接口用于页面恢复、SSE 异常后的 reconciliation 和测试，不返回 diff、Prompt 或 Provider 原始数据。

### 6.3 订阅事件

```http
GET /api/v1/git-commits/:operation_id/events
Accept: text/event-stream
```

Commit SSE 使用独立 Schema v1：

```text
id: <operation 内连续递增 seq>
event: git.commit.progress
data: <JSON>
```

支持 `Last-Event-ID` 续传。事件必须先持久化，再向订阅者扇出。

这不是 Run SSE，不使用 `run_id`，不写入现有 `run_events`，也不加入 `PublicEventTypes`。

### 6.4 `git.commit.progress`

```json
{
  "schema_version": 1,
  "operation_id": "gco_123",
  "project_id": "pro_123",
  "thread_id": "thread_123",
  "occurred_at": "2026-08-30T06:29:00.000Z",
  "phase": "analyzing"
}
```

`phase` 只允许：

```text
staging | analyzing | committing
```

Progress 是可替换实时状态，不投影为持久 Timeline 项。

### 6.5 `git.commit.empty`

```json
{
  "schema_version": 1,
  "operation_id": "gco_123",
  "project_id": "pro_123",
  "thread_id": "thread_123",
  "occurred_at": "2026-08-30T06:29:01.000Z"
}
```

`empty` 是正常终态，不带 error，不写入 Thread Timeline。

### 6.6 `git.commit.completed`

```json
{
  "schema_version": 1,
  "operation_id": "gco_123",
  "project_id": "pro_123",
  "thread_id": "thread_123",
  "occurred_at": "2026-08-30T06:29:08.000Z",
  "commit": {
    "title": "fix: 优化增长图表标签布局",
    "items": [
      "强化第三页季度增长趋势的视觉层级",
      "调整数据标签位置并消除内容重叠"
    ],
    "branch": "main",
    "hash": "8af42d9",
    "files_changed": 3,
    "insertions": 46,
    "deletions": 18,
    "committed_at": "2026-08-30T06:29:08.000Z"
  }
}
```

统计来自 Git，不由模型生成。`occurred_at` 和 `committed_at` 使用 RFC 3339 UTC；前端按用户本地时区显示 `YYYY-MM-DD HH:mm`。

### 6.7 `git.commit.failed`

```json
{
  "schema_version": 1,
  "operation_id": "gco_123",
  "project_id": "pro_123",
  "thread_id": "thread_123",
  "occurred_at": "2026-08-30T06:29:08.000Z",
  "error": {
    "code": "COMMIT_MESSAGE_INVALID",
    "message": "提交失败，请重新尝试或手动提交",
    "retryable": true
  }
}
```

公共 error 必须安全、稳定、可本地化。内部 Cause、Git stderr、Provider body 和无效模型输出只进入受控日志，并关联 `operation_id`。

## 7. 后端设计

### 7.1 服务边界

新增 `GitCommitService`，职责包括：

1. 验证 Project、Thread、Profile 与并发条件；
2. 创建和恢复 Git Commit Operation；
3. 管理 Project 独占写锁；
4. 调用固定 Git 子命令；
5. 构造 Commit LLM 上下文并验证 tool call；
6. 持久化操作、事件和 Thread Timeline 投影；
7. 对外只返回安全错误。

它依赖：

```text
Store
LLM Registry
Project Lock Manager
Git Executor
Commit Event Bus
Clock / ID generator
```

它不依赖 Agent Runtime、Run Bus、Checkpoint、Prompter、PPT Tool Registry 或通用 `run_command`。

### 7.2 Git Executor

新增专用窄接口，不复用面向 Agent 的 `commandexec`：

```go
type GitCommitExecutor interface {
    Bootstrap(ctx context.Context, workDir string) error
    Status(ctx context.Context, workDir string) (GitStatus, error)
    StageAll(ctx context.Context, workDir, operationID string) (StagedChangeSet, Cleanup, error)
    Commit(ctx context.Context, workDir string, staged StagedChangeSet, message CommitMessage) (GitCommitResult, error)
}
```

实现只允许代码内固定的 Git 子命令和参数。任何请求字段、模型输出或文件内容都不得成为可解释的命令片段。

不得执行：

- `git reset --hard`；
- `git checkout --`；
- `git clean`；
- `git push`；
- `git commit --no-verify`；
- shell `-c`、重定向或字符串拼接命令。

Git hooks 正常执行。hook 拒绝 Commit 时操作失败，不绕过 hook。

### 7.3 隔离暂存与回滚

Commit 分析不得破坏真实 index。后端使用 operation-scoped 临时 index：

```text
<work_dir>/.git/ppt-agent/index-<operation_id>
```

流程：

1. 从当前 `HEAD` tree 初始化临时 index；unborn branch 使用空 tree；
2. 设置 `GIT_INDEX_FILE` 指向临时 index；
3. 在临时 index 上执行 `git add -A`；
4. 从同一个临时 index 读取 staged diff 和 numstat；
5. 使用同一个临时 index 执行 `git commit`；
6. 成功后使真实 index 与新 `HEAD` 一致；
7. 失败或取消时删除临时 index，真实 index 和工作区保持不变。

该方案优先于“先修改真实 index，再尝试恢复”，避免模型失败、请求取消或进程异常留下用户不可见的暂存状态。

服务启动时清理没有对应 running Operation 的陈旧临时 index。若发现 Commit 已创建但 index 尚未对齐，按 Operation 记录中的 commit hash 完成幂等 reconciliation，不重复创建 Commit。

### 7.4 Commit LLM

Commit 使用请求中的当前 Profile。Profile 必须支持 tool calls；不静默切换到默认模型或其他 Provider。

新增独立文件化 Prompt：

```text
backend/prompts/git_commit/
```

Prompt 只负责根据权威 staged change set 生成提交说明，不包含 Runtime 模式、计划、资源工具、完成门控或用户对话历史。

模型只获得：

- Project 标题；
- staged 文件状态与 numstat；
- staged textual diff，按确定性预算裁剪；
- 二进制文件只提供路径和统计，不发送正文；
- 固定 Commit 规范与 tool schema。

不得发送：

- Thread History；
- Composer 草稿；
- Provider 配置、密钥和 HTTP body；
- `.git` 内容；
- ignored 文件；
- 应用数据库、运行日志或内部 trace。

### 7.5 `git_commit` tool

Commit LLM 只暴露一个工具：

```json
{
  "name": "git_commit",
  "description": "生成当前 staged change set 的 Git 提交说明",
  "parameters": {
    "type": "object",
    "additionalProperties": false,
    "required": ["title", "items"],
    "properties": {
      "title": {
        "type": "string",
        "minLength": 1,
        "maxLength": 72
      },
      "items": {
        "type": "array",
        "minItems": 1,
        "maxItems": 6,
        "items": {
          "type": "string",
          "minLength": 1,
          "maxLength": 160
        }
      }
    }
  }
}
```

标题优先遵循：

```text
feat: <核心改动概述>
fix: <核心修复概述>
refactor: <核心重构概述>
docs: <文档改动概述>
test: <测试改动概述>
chore: <维护改动概述>
```

正文由后端确定性格式化：

```text
<title>

- <item 1>
- <item 2>
```

有效响应必须满足：

1. 恰好一个 tool call；
2. tool name 必须为 `git_commit`；
3. 不接受正文文本作为降级结果；
4. `title` 和 `items` 通过 schema、去空白、控制字符和长度校验；
5. items 去重后仍至少一项；
6. 不包含伪造 hash、branch、文件统计或执行承诺。

无效时使用同一 Profile 重新生成，最多三次模型生成尝试。第三次仍无效则终止为 `COMMIT_MESSAGE_INVALID`。Provider Adapter 自身的网络重试不计入这三次结构修复尝试。

### 7.6 Git 命令顺序

固定执行语义：

```text
validate repository state
emit staging
build isolated index with git add -A
read staged diff and stats
if empty -> emit empty
emit analyzing
generate and validate git_commit tool call, max 3 attempts
emit committing
git commit using isolated index and fixed identity
read branch/hash/timestamp/stats
persist completed operation and Thread projection
emit completed
```

`committing` 后只允许一次真实 `git commit`。模型重试永远发生在该阶段之前。

### 7.7 并发

Commit 使用与 Run 和 Project mutation 相同的 Project 写锁：

- active Run 时不能创建 Commit；
- active Commit 时不能创建 Run、steering 或 Project mutation；
- 同一 Project 同时最多一个 Commit；
- 不同 Project 可以并行 Commit；
- 读取接口不受影响。

前端禁用是体验层约束，后端锁和状态检查才是权威保证。

### 7.8 持久化

新增独立持久化模型：

```text
git_commit_operations
  id
  project_id
  thread_id
  client_request_id
  model_profile
  status
  phase
  result_json
  error_json
  created_at
  updated_at

git_commit_events
  operation_id
  seq
  event_type
  payload
  created_at
```

约束：

- `(thread_id, client_request_id)` 唯一；
- 同一 Operation `seq` 连续递增；
- 每个 Operation 恰好一个终态事件；
- progress 与终态事件先落库再扇出；
- `completed` 的 `result_json` 是 Thread Timeline Git Commit 项的权威来源；
- `empty` 不进入 Thread 历史；
- `failed` 进入当前 Thread，供刷新后继续显示和重试。

`ThreadService.History` 将终态 `completed` / `failed` Operation 按 `occurred_at` 合并到 Thread History 投影。Git Commit 项不写入被 `.gitignore` 忽略的 Thread JSONL，避免 Commit 完成后再次制造工作区变更。

## 8. 前端设计

### 8.1 API

新增：

```text
frontend/src/api/gitCommits.ts
```

职责：

- 创建 Operation；
- 查询 Operation；
- 订阅并严格解析 Commit SSE；
- 校验 `schema_version`、事件名、phase 和终态 payload；
- 未知事件仅在开发环境 warning。

不得复用 `SSEEvent` / `parsePublicEvent` 强行解析，因为现有类型要求 `run_id` 且只表达 Run。

### 8.2 Store

Commit 状态按 `project_id` 保存，而不是只按 Thread：

```ts
interface ProjectCommitSession {
  operationId: string | null;
  sourceThreadId: string | null;
  status: 'idle' | 'creating' | 'running' | 'empty' | 'completed' | 'failed';
  phase: 'staging' | 'analyzing' | 'committing' | null;
  lastEventId?: string;
  streamClose: (() => void) | null;
}
```

原因：

- Git 仓库属于 Project；
- 同一 Project 的多个 Thread 共享同一工作区；
- Commit 期间所有 Thread 都不能启动新的 Project 写操作；
- 成功/失败 Timeline 项只插入 `sourceThreadId`。

浏览器刷新后若存在未终态 Operation，先调用 GET Operation，再从 `lastEventId` 订阅；终态则从 Thread History 恢复，不重复追加。

### 8.3 Timeline 类型

新增两种 Timeline 投影：

```ts
type GitCommitItem = {
  type: 'git_commit';
  operationId: string;
  status: 'completed';
  title: string;
  items: string[];
  branch: string;
  hash: string;
  filesChanged: number;
  insertions: number;
  deletions: number;
  committedAt: number;
};

type GitCommitFailureItem = {
  type: 'git_commit';
  operationId: string;
  status: 'failed';
  occurredAt: number;
  retryable: boolean;
};
```

Live SSE reducer 与 History hydrator 必须生成相同 ID 和字段，保证刷新前后视觉一致。

### 8.4 组件

建议组件边界：

```text
AgentPanelHeader
  GitCommitButton

Timeline
  GitCommitProgress
  GitCommitEvent
  GitCommitFailure

AppToastPortal
```

`GitCommitEvent` 的展开状态仅为本地 UI 状态，不持久化。按钮使用现有 Lucide 图标和 tooltip 组件，不新增手绘 SVG。

### 8.5 控件互斥

以下条件任一成立时禁用 Commit：

- 无 active Project；
- 无 active Thread；
- active Run；
- active Commit；
- Project mutation pending；
- 当前 Profile 不支持 tool call；
- 网络请求正在创建 Operation。

active Commit 时：

- Send disabled；
- Polish disabled；
- Commit disabled；
- Composer textarea 保持 enabled；
- Thread 切换允许，但该 Project 其他 Thread 的发送仍禁用。

## 9. 错误分类

| 内部场景 | 公共 code | 用户投影 | retryable |
| --- | --- | --- | --- |
| Git 不可用 | `GIT_UNAVAILABLE` | 提交失败，请重新尝试或手动提交 | false |
| 仓库状态冲突 | `GIT_REPOSITORY_CONFLICT` | 提交失败，请重新尝试或手动提交 | false |
| staged change set 读取失败 | `GIT_STAGE_FAILED` | 提交失败，请重新尝试或手动提交 | true |
| Profile 调用失败 | `PROVIDER_UNAVAILABLE` | 提交失败，请重新尝试或手动提交 | true |
| 三次模型结构校验失败 | `COMMIT_MESSAGE_INVALID` | 提交失败，请重新尝试或手动提交 | true |
| Git hook 或 commit 失败 | `GIT_COMMIT_FAILED` | 提交失败，请重新尝试或手动提交 | true |
| 服务进程中断 | `COMMIT_INTERRUPTED` | 提交失败，请重新尝试或手动提交 | true |

UI 统一显示安全文案。详细错误只进入后端日志，日志必须过滤凭据和 diff 正文。

若 Git Commit 已成功但后续投影步骤失败，Operation reconciliation 必须通过已记录的 commit hash 恢复 `completed`，不得创建第二个 Commit，也不得向用户显示“未产生版本”。

## 10. 安全

1. Git 只在 Store 返回的 Project `work_dir` 运行。
2. Git 参数由代码固定；模型输出只作为 commit message 数据传入，不作为命令。
3. 不调用 shell，不执行用户 hooks 之外的任意程序，不绕过 hooks。
4. `.git`、ignored 文件、Thread History、密钥和本地配置不进入模型上下文。
5. diff 输入有文件数、单文件字节数、总字节数和 token 上限；超限使用确定性摘要，不无限扩张请求。
6. 二进制文件不读取正文。
7. 公共事件不包含 diff、Prompt、Provider reasoning、原始 tool args、绝对路径或 stderr。
8. Commit message 删除 NUL 和控制字符，限制标题/items 长度，并使用参数数组传递给 Git。
9. Operation 日志以 `operation_id` 关联，不记录 commit message 之外的用户项目正文。

## 11. 测试

### 11.1 后端单元测试

- 新 Project 初始化为独立 `main` 仓库，并以固定 `chore: init project` 创建脚手架基线提交，父仓库不会被使用。
- 初始化提交不调用 LLM、不创建 Operation、不进入 Thread Timeline；创建后无修改的首次手动 Commit 返回 `empty`。
- bootstrap 幂等补齐 ignore 条目且不覆盖已有 `.gitignore`。
- `threads/` 变化不会使 Project 仓库变脏。
- `StageAll` 包含 tracked、deleted、renamed 和 untracked 文件。
- 无变更产生 `staging -> empty`，不调用 LLM。
- 正常流程严格产生 `staging -> analyzing -> committing -> completed`。
- phase 只在对应真实动作成功后发送。
- 模型第一次/第二次非法、第三次合法时只创建一个 Commit。
- 三次非法返回 `COMMIT_MESSAGE_INVALID`，不产生 Commit，真实 index 不变。
- Provider 失败、Git add 失败、hook 拒绝、commit 失败均清理临时 index。
- completed 的 hash、branch、时间和统计来自 Git。
- active Run / Commit / mutation 正确互斥。
- 同一 `client_request_id` 幂等返回同一 Operation。
- SSE seq 连续，Last-Event-ID 只续发未消费事件。
- 服务重启 reconciliation 不重复 Commit。
- Thread History 可恢复 completed 与 failed，empty 不出现。

### 11.2 后端集成测试

使用临时真实 Git 仓库与 fake LLM Provider：

1. 创建 Project；
2. 修改多个项目文件并新增文件；
3. fake Provider 返回一个合法 `git_commit` tool call；
4. 订阅完整 SSE；
5. 验证 Git log、commit body、clean status、统计和 Thread History；
6. 再次 Commit 验证 `empty`。

不得依赖开发机器全局 Git identity。

### 11.3 前端测试

- Commit 按钮位置、tooltip、禁用条件和绿色状态。
- Commit 不发送 Composer 草稿、不创建用户消息。
- active Commit 时发送、Polish、Commit 禁用，textarea 可编辑。
- progress 三节点由 phase 驱动，圆点与标签中心对齐。
- completed 组件默认折叠、元信息顺序和竖线分组正确。
- 展开只显示 items，不显示文件列表。
- empty 只显示 viewport 顶部中央 Toast，不新增 Timeline 项。
- failed 使用同一 Commit 图标红色变体，只显示日期、失败提示和重试按钮。
- 重试创建新 Operation，旧失败项保留。
- SSE 重连和 History hydration 不重复 Timeline 项。
- Project 多 Thread 下互斥状态同步。

### 11.4 回归测试

- 现有 Run SSE Schema v3、Run Store 和 Timeline reducer 行为不变。
- Polish 仍不创建 Run 或历史，但 active Commit 时前端禁用入口。
- Project mutation、Run 创建和 steering 在 active Commit 时由后端拒绝。
- Go 全量测试、前端 test、lint、typecheck 和 build 全部通过。

## 12. 实施顺序

### Phase 1：Git 仓库与执行器

- Project 初始化和 bootstrap；
- `.gitignore`；
- 专用 Git executor；
- 隔离 index 与真实 Git 集成测试。

### Phase 2：Commit Operation 与 LLM

- 持久化表和 Store；
- Project 写锁互斥；
- Prompt registry 和 `git_commit` tool；
- 三次结构修复；
- Operation 状态机和安全错误。

### Phase 3：API 与事件

- 创建/查询接口；
- Commit SSE v1、事件校验、持久化和续传；
- Thread History 终态投影；
- 重启 reconciliation。

### Phase 4：前端

- API parser 和 Project Commit Store；
- Header Commit 入口与控件互斥；
- 全宽三节点进度条；
- empty Toast；
- completed/failed Timeline 组件；
- History hydration 与重试。

### Phase 5：验证

- 前后端单元与集成测试；
- 多 Thread 和 active Run 互斥测试；
- 无变更、模型三次非法、Git hook 失败、刷新重连和服务重启场景；
- 删除所有临时原型依赖，生产组件不得引用 `tmp/`。

## 13. 验收标准

1. 每个 Project 是独立 Git 仓库，Git 操作绝不越过 Project `work_dir`。
2. Commit 一次包含全部 tracked 和未忽略的 untracked 项目变更，不提供文件选择。
3. Commit 不创建 Run、不发送用户消息、不进入 Runtime 公共事件。
4. `staging`、`analyzing`、`committing` 只在真实后端边界触发，前端不模拟阶段。
5. 模型必须通过唯一 `git_commit` tool call 返回标题和 items，非法输出最多尝试三次。
6. 任意失败不损坏工作区或真实 index，不产生重复 Commit。
7. 无变更只显示应用级中性 Toast，不产生 Timeline 项。
8. 成功项显示 `日期时间 | branch + hash | 文件数 +增 -删`，展开后只显示 items。
9. 失败项使用同一 Commit 图标的红色变体，只显示日期、统一失败提示和右侧重试按钮。
10. active Commit 与 Run、steering 和 Project mutation 后端互斥；textarea 仍可编辑。
11. completed 与 failed 在刷新后可从当前 Thread History 恢复，progress 和 empty 不持久展示。
12. 公共 API、SSE、日志和错误不泄露 diff 正文、内部路径、Prompt、Provider 原始内容或凭据。
13. 现有 Run SSE Schema v3 和 Agent Runtime 行为保持不变。
14. 全部新增测试和既有回归测试通过。
