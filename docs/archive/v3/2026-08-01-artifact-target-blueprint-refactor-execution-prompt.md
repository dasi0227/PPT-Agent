# Artifact Target / Blueprint 全栈重构执行 Prompt

> 用途：将本文件全文交给负责代码实现的 Agent。
> 本 Prompt 要求 Agent 自主规划并一次性完成实现，不在常规技术选择上向用户提问。

## 启动 Prompt

你现在是本项目的高级全栈架构师与实现负责人。请在当前仓库中自主制定计划并一次性完成“旧 kind/scope/mode 协议到 artifact/level/interaction 协议”的前后端完整重构。

这不是设计讨论任务，而是代码实现、迁移、测试与文档更新任务。除非出现无法通过代码、测试、Git 历史和本文档判断的外部业务冲突，否则不要停下来询问用户，也不要要求用户逐步确认。你应先完成只读调查，再制定内部计划，随后持续实现、验证、修正，直至满足 Definition of Done。

### 一、开始前必须完整阅读

1. `docs/superpowers/specs/2026-08-01-artifact-target-blueprint-refactor-design.md`
2. `docs/superpowers/specs/2026-08-01-agent-runtime-upgrade-roadmap-design.md`
3. 仓库根目录及相关子目录中的 `AGENTS.md`
4. 当前 README、架构文档、API 文档、前后端模型定义、数据库迁移、Run/Agent Loop、工具注册、事件流、前端聊天区与预览区代码
5. 最近至少 10 条 Git commit，并重点确认以下已完成能力不可回退：
   - outline → generate / render 的核心闭环
   - 预览安全隔离
   - deck controls 与 history replay

第一份设计文档是本次实现的产品与协议真相源。第二份路线图用于理解长期边界；本次只实现其中 R0“领域语言统一”，不要顺手实现 R1–R7。

### 二、本次重构的唯一目标

彻底消除旧协议中 `kind = outline | generate | edit | command` 同时混合“产物类型、操作动作、交互方式”的问题，替换为相互正交的目标和交互协议：

```text
target.artifact = blueprint | presentation
target.level    = slide | deck

interaction.intent        = apply | consult
interaction.clarification = when_blocked | before_apply | never
```

四种核心执行组合必须全部可用：

| artifact | level | 产品含义 |
|---|---|---|
| blueprint | deck | 创建或修改整份演示的目标、叙事、目录层级、页面顺序 |
| blueprint | slide | 创建或修改某一页的语义蓝图 |
| presentation | deck | 根据蓝图物化整份 HTML 演示，或对整份演示做全局修改 |
| presentation | slide | 生成或修改某一页 HTML |

必须遵守以下语义边界：

- `generate`、`edit` 不再是公共 `kind`。它们只允许成为 Runner 内部根据当前物化状态推导出的操作，例如 `materialize`、`revise`、`rebuild`。
- `render` 不是 Agent kind。它继续表示把 HTML 以安全方式送入预览运行时的 HTTP/Runtime 能力。
- `talk` 对应 `interaction.intent = consult`，只回答、分析和建议，不修改项目。
- `ask` 不再是 kind 或 mode；它对应 clarification policy。只有确实缺失阻塞信息时才产生 `needs_input`。
- `repo` 不属于 PPT 编辑目标的 level/scope。MVP 中 repo 只是受控的资产来源，Agent 仅可搜索、读取、引用或挂载已有组件和资源。
- `current` 只能作为前端临时选择；创建 Run 前必须解析成稳定的 `slide_id`。后端公共协议不得出现 `current`。

### 三、目标持久化模型

将 Blueprint 实现为逻辑聚合，而不是一个巨大 JSON，也不要引入元素级 AST。

目标项目结构：

```text
project/
├── deck.json
├── design/
│   └── design-spec.json
├── slides/
│   └── <stable-slide-id>/
│       ├── slide.json
│       └── index.html
└── ...
```

#### 1. deck.json

只负责全局语义、轻量目录和顺序，至少包含：

- `schema_version`
- `revision`
- `project_id`
- `title`
- `goal`
- `audience`
- `language`
- `core_thesis`
- `narrative_arc`
- `sections[]`
  - `id`
  - `number`
  - `title`
  - `subsections[]`
    - `id`
    - `number`
    - `title`
- `outline_order[]`：稳定 `slide_id`

`sections/subsections` 表示目录中的一级模块和二级模块，例如 `01` 与 `1.1`，不是页面内部复杂 content parts。

#### 2. design/design-spec.json

只负责全局视觉约束，不重复页面业务内容。至少包含设计文档定义的：

- `schema_version`
- `revision`
- 画布与比例
- 色彩、字体、间距、圆角、阴影
- 默认版式原则、视觉基调和可复用设计 token

#### 3. slides/<stable-slide-id>/slide.json

每一页都有独立语义蓝图，至少包含：

- `schema_version`
- `revision`
- `slide_id`
- `section`
- `subsection`
- `role`
- `title`
- `key_message`
- `content.summary`
- `content.points[]`
- `visual_intent.archetype`
- `visual_intent.description`
- `visual_intent.asset_queries[]`
- `speaker_notes`

约束：

- `key_message` 是该页希望观众记住的一句话。
- `content` 保持语义级，不设计细碎 `parts[]`。
- `visual_intent` 描述表达意图，不描述绝对坐标和 DOM 节点。
- `slide_id` 必须稳定，不得因为排序或标题变化而改变。
- 页面 HTML 仍以 `index.html` 为最终可预览、可播放产物。

### 四、Revision 与派生状态

不要继续用单个 `outline_dirty` 布尔值表达所有同步关系。实现设计文档中的 revision 模型：

- deck blueprint revision
- per-slide blueprint revision
- design-spec revision
- presentation materialization metadata

系统必须能派生并向前端返回至少以下状态：

```text
not_materialized
fresh
blueprint_stale
design_stale
unknown
```

revision 更新必须遵守：

- 只有成功写入对应语义文件后才递增。
- consultation 不递增。
- 单页蓝图修改不应错误地让其他无关页面变旧。
- 全局 design revision 更新应使相关 presentation 派生为 `design_stale`。
- 失败或取消的 Run 不得伪造成功 revision。

### 五、后端实现要求

#### 1. 领域类型

在 Go 后端引入强类型模型，命名可按现有代码风格调整，但必须表达：

```go
type Artifact string // blueprint | presentation
type Level string    // slide | deck
type Intent string   // apply | consult
type ClarificationPolicy string // when_blocked | before_apply | never

type RunTarget struct {
    Artifact Artifact
    Level    Level
    SlideID string
}

type RunInteraction struct {
    Intent        Intent
    Clarification ClarificationPolicy
}
```

避免在核心服务和 Runner 中继续传递无约束字符串。实现集中校验及可识别的领域错误。

#### 2. 新建 Run API

新协议请求体以设计文档为准，核心形态为：

```json
{
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "slide-stable-id"
  },
  "interaction": {
    "intent": "apply",
    "clarification": "when_blocked"
  },
  "instruction": "把第 3 页改成对比结构"
}
```

校验要求：

- 继续通过现有 `/threads/:threadId/runs` 路径确定 thread；project 归属由后端从 thread 解析，不在请求体中接受可伪造的 `project_id`。
- `slide` 必须带有效稳定 `slide_id`。
- `deck` 不得误带 `current`；是否允许携带空 `slide_id` 按设计文档实现。
- 枚举非法值返回结构化 4xx，而不是进入 Runner 后失败。
- `consult` 禁止持久化修改工具。
- `before_apply` 在执行写操作前发出一次结构化确认需求。
- `when_blocked` 只在信息确实不足以继续时触发。
- `never` 要基于合理默认值继续；若继续会造成高风险不可逆结果，则安全失败。

#### 3. RunnerResolver 与四类 Runner

实现显式 Runner 路由，不要把所有分支继续堆在单个巨大 switch 中：

```text
blueprint/deck
blueprint/slide
presentation/deck
presentation/slide
```

每类 Runner 均需：

- 独立系统指令或清晰可组合的 prompt policy。
- 只拿到该目标所需上下文。
- 只获得允许的 capabilities。
- 输出统一事件协议。
- 通过服务层写文件和更新 revision，不直接散落文件写逻辑。

至少建立以下服务边界：

- `BlueprintService`
- `PresentationService`
- `PPTMutationService`
- `RevisionService` 或等价聚合
- `RunnerResolver`
- `LegacyRunAdapter`

可以根据现有目录结构调整文件位置，不必为了名称进行空洞分层，但职责必须清晰且可测试。

#### 4. Capability 权限

用“能力”替代旧 `Tool.Scopes`：

```text
read_blueprint
write_blueprint
read_presentation
write_presentation
read_design_spec
write_design_spec
search_assets
read_assets
mount_assets
render_preview
```

最低要求：

- `consult` 只能获得只读能力。
- blueprint Runner 不应随意写 presentation。
- presentation Runner 可以读取 blueprint 与 design spec。
- repo 资产能力不得隐式开放任意仓库写入和命令执行。
- 写操作必须保留现有事务性/原子性保护；若现有实现不足，在不扩张到 R4 的前提下至少保证“临时文件 + 原子替换”或等价安全写入。

#### 5. 事件与结果

升级 `run.started`、`run.completed`/`run.failed` 等结构，让前端能读到：

- 标准化 target
- interaction
- operation（Runner 内部推导）
- affected resources
- before/after revisions
- materialization status
- 可恢复错误或 `needs_input`

不要暴露模型原始 chain-of-thought。可对用户展示简短、产品化的 action summary、decision summary 和 tool outcome。

#### 6. 兼容迁移

实现有限期兼容适配层：

```text
outline + overview -> blueprint/deck
outline + page/current -> blueprint/slide
generate + overview -> presentation/deck
generate + page/current -> presentation/slide
edit + overview -> presentation/deck
edit + page/current -> presentation/slide
mode=talk -> intent=consult
mode=normal -> intent=apply
mode=ask -> clarification=before_apply
```

要求：

- 兼容逻辑集中在 `LegacyRunAdapter` 或等价入口，不污染新领域层。
- 对旧 `current` 必须在适配入口解析成稳定 `slide_id`。
- 旧 `command` 不进入新 Runner；继续在命令解析层转换成新 target/interaction，或返回明确的弃用错误。
- 对被转换的旧请求写结构化 deprecation log/metric。
- 新前端只发送新协议。
- 在本次重构中保留旧 API 的可用性，除非现有文档明确允许 breaking change；删除适配层必须留到独立后续版本。

#### 7. 数据迁移

为已有项目提供幂等迁移：

- 从旧 outline/project metadata 生成 `deck.json`。
- 为已有页面生成稳定 `slide_id`，不得使用可变数组下标作为长期身份。
- 生成各页 `slide.json`，无法确定的内容使用明确、安全的默认值。
- 从现有主题/样式信息生成 `design/design-spec.json`。
- 记录 `schema_version` 和 revision。
- 重复执行迁移不得产生重复页面、改变稳定 ID 或无意义增加 revision。
- 遇到部分缺失/损坏数据时输出可诊断错误，并尽可能保持旧项目可读。

不得直接删除旧数据。若需要改名或迁移，先保留可回滚来源，并在测试中覆盖旧项目打开与迁移。

### 六、前端完整适配

#### 1. 类型与 API

- 删除新代码对 `KindOutline/Generate/Edit/Command`、旧 scope、旧 mode 的依赖。
- 建立与后端一致的 `RunTarget`、`RunInteraction`、Blueprint 与 revision 类型。
- API client 只构造新请求。
- 所有 `"current"` 在发请求前解析成 store 内稳定 `slide_id`。
- 对后端结构化错误和 `needs_input` 提供一致处理。

#### 2. Composer / 右侧对话栏

将用户面向的选择设计为“我想修改什么”，而不是暴露技术 kind：

- 产物：蓝图 / 演示
- 范围：当前页 / 整份
- 交互：执行 / 讨论
- 澄清策略原则上使用产品默认值；不要制造频繁的人为确认

默认建议：

```text
artifact = presentation
level = slide（有选中页时）
intent = apply
clarification = when_blocked
```

切换到整份时使用 `deck`；切换到讨论时使用 `consult`。UI 文案可以本地化，但底层枚举不可漂移。

斜杠命令若保留，必须只是填写 target/interaction 的快捷方式，不再形成第二套协议。

#### 3. Blueprint UI

替换当前“把一堆字段贴到白板上”的大纲预览，至少实现：

- `DeckDirectoryPanel`：展示一级 section、二级 subsection 和页面归属。
- `SlideBlueprintCard`：突出 role、title、key message、内容摘要、视觉意图。
- `DesignSpecSummary`：展示全局风格摘要，不混入页面内容。
- 页面选择与稳定 `slide_id` 联动。
- 修改后根据 revision/materialization 状态展示“未生成 / 已同步 / 蓝图有更新 / 风格有更新 / 状态未知”。

不要在本轮建设自由拖拽式元素编辑器，也不要把 slide blueprint 变成 DOM/图层树。

#### 4. 状态刷新

修正所有依赖旧 `outline_dirty` 的刷新逻辑：

- Run 完成后按 `affected resources` 精确失效。
- blueprint/deck 变更刷新目录及受影响页面摘要。
- blueprint/slide 只刷新目标页和必要的 deck 汇总。
- presentation 变更刷新对应预览。
- design-spec 变更刷新全局风格摘要并更新派生 stale 状态。
- history replay 必须能恢复新 target、interaction、operation 与结果摘要。

不得破坏已完成的预览 sandbox/独立 Origin 安全策略。

### 七、测试要求

先理解现有测试风格，再补齐以下测试。禁止通过删除、跳过或放宽关键断言来“通过”。

#### 后端单元测试

- 所有枚举的合法与非法值。
- 四种 target 组合路由正确。
- slide 缺失/无效 `slide_id` 被入口拒绝。
- consult 无法获取写 capability。
- clarification 三种策略行为。
- LegacyRunAdapter 的所有映射。
- `current` 能解析为稳定 ID，无法解析时返回明确错误。
- Blueprint 文件读写、schema validation、revision 增长。
- materialization status 推导。
- 数据迁移幂等性与稳定 ID。
- Runner 失败/取消不错误提交 revision。

#### 后端集成测试

- 新协议创建 Run、事件流、完成结果。
- blueprint/deck 修改。
- blueprint/slide 修改。
- presentation/deck 物化。
- presentation/slide 生成与修改。
- 旧请求仍通过适配层工作并产生 deprecation 信号。
- `/render` 及安全预览相关测试继续通过。
- 非法 capability 与 repo 写入尝试被拒绝。

#### 前端测试

- Composer 默认值与四种组合切换。
- `current` 到 stable `slide_id` 解析。
- 新 API payload。
- Blueprint 目录、页面卡片、设计摘要渲染。
- revision/materialization 状态标签。
- consult 与 apply 的交互差异。
- `needs_input` UI。
- history replay。
- 已有 deck controls 与预览安全行为不回归。

#### 端到端验收

至少覆盖：

1. 新建/打开旧项目并完成幂等迁移。
2. 生成整份 deck blueprint。
3. 修改单页 blueprint，并确认没有误改其他页。
4. 从 blueprint 物化整份 presentation。
5. 修改当前页 HTML presentation 并安全预览。
6. 修改全局 design spec 后显示 `design_stale`。
7. consult 询问建议且磁盘无项目内容变化。
8. history 中能清楚识别目标、操作、影响文件和 revision。

### 八、实施顺序

请自主建立并维护执行计划，建议按以下依赖顺序推进：

1. 基线调查：工作树、最近 commits、现有测试、模型/API/存储/Runner/UI 数据流。
2. 新领域类型、JSON schema、validation、revision/status 推导。
3. 幂等项目数据迁移与兼容读。
4. 新 Run API、LegacyRunAdapter、数据库迁移。
5. RunnerResolver、四类 Runner、capability policy。
6. 事件流与结果结构。
7. 前端类型、API client、store。
8. Composer 与 Blueprint UI。
9. history replay、状态失效与预览联动。
10. 单元、集成、前端、E2E 测试。
11. README、架构文档、API 示例、迁移说明更新。
12. 全量验证与残留旧术语审计。

如果当前代码结构使上述顺序不合理，可以调整，但不得省略目标。

### 九、Git 与工程纪律

- 开始前执行 `git status --short`，识别并保留用户已有修改。
- 不得使用 `git reset --hard`、`git checkout --` 或删除用户改动。
- 不得提交 `.run` 日志、SQLite 运行产物、临时预览文件、截图、缓存和密钥。
- 检查 `.gitignore` 是否继续覆盖运行产物。
- 只修改本需求相关文件。
- 每完成一个可独立验证的迁移阶段，创建语义清楚的小提交；不得把全部改动压成无法审查的巨型提交。
- 建议提交分组：
  1. domain model + JSON schema + migration
  2. run protocol + compatibility adapter + runners
  3. frontend composer + blueprint views + state
  4. tests + docs + legacy cleanup
- 提交前运行格式化、静态检查与对应测试。
- 最后运行仓库支持的全量测试；若某项因环境依赖无法运行，保留错误证据并运行可行的最接近验证，不得谎报通过。

### 十、禁止事项

- 不要将 `generate` 或 `edit` 换个名字后继续作为公共 kind。
- 不要把 `command`、`talk`、`ask` 塞回 target。
- 不要保留新旧两套前端状态源长期并行。
- 不要把 repo 当作第五种 PPT scope。
- 不要使用数组下标作为永久 slide ID。
- 不要引入复杂元素级 JSON AST 或 `content.parts[]`。
- 不要在本轮重写完整 Agent Runtime、接入多模型路由、Durable Queue 或完整 Eval 平台。
- 不要为了重构破坏 `/render`、预览隔离、history replay、deck controls 和现有可工作的生成链路。
- 不要向用户暴露或持久化模型原始 chain-of-thought。
- 不要把失败测试标记为跳过来完成任务。

### 十一、阻塞处理规则

默认自主决策，依据优先级为：

1. 本 Prompt 与 `artifact-target-blueprint-refactor-design.md`
2. 现有已验证测试和 API 行为
3. 最近 commits 体现的架构方向
4. 仓库编码约定
5. 最小复杂度、可迁移、可回滚的工程判断

只有当同一个外部阻塞连续出现至少三次，且你已经尝试了至少三种安全、在范围内的替代方案，仍无法取得任何实质进展时，才停止并请求用户输入。请求时必须提供：

- 精确阻塞点
- 已尝试的方案及证据
- 剩余选项
- 你的推荐默认选项

普通命名、目录、内部接口、迁移细节和测试修复不属于需要询问用户的事项。

### 十二、Definition of Done

仅当以下条件全部满足，任务才算完成：

- 新前端不再发送旧 kind/scope/mode。
- 后端核心领域与 Runner 不再依赖旧 kind/scope/mode。
- 四种 artifact × level 组合均有可测试执行路径。
- talk/ask 已分别归位为 intent 与 clarification policy。
- repo 已从 PPT target 中移除并受 capability 约束。
- `current` 已被限制在前端，并在请求前解析为稳定 `slide_id`。
- deck、design spec、per-slide blueprint 已按 schema 分离并可持久化。
- revision 和 materialization status 正确工作。
- 旧项目可幂等迁移。
- 旧请求通过集中适配层兼容，且产生弃用信号。
- Blueprint UI 不再是字段堆叠白板，而能表达目录、单页主旨和全局风格。
- 所有新增与既有关键测试通过。
- 全仓残留旧术语经过审计；允许的残留仅存在于 legacy adapter、migration、兼容测试和迁移文档。
- 文档包含新协议示例、目录结构、迁移策略和兼容期限建议。
- 工作树中没有运行产物、调试日志、数据库文件或无关改动。

### 十三、最终汇报格式

完成后一次性给出：

1. 最终实现结果，而不是只汇报计划。
2. 关键架构变化：领域模型、Runner、存储、前端和兼容策略。
3. 数据迁移与向后兼容结果。
4. 测试命令、通过数量和未运行项。
5. 新增提交列表。
6. 尚存风险与明确后续项；后续项应映射到 Runtime 路线图 R1–R7，不得冒充本轮已完成。
7. 关键文件的可点击路径。

现在开始：先只读调查并形成内部计划，然后持续执行到 Definition of Done。不要仅输出建议、伪代码或待办清单。
