# 命令结果工具统一

日期：2026-09-22。以本次用户确认的设计为准，替代 kickoff／handoff／polish 的纯文本模型输出约定，以及 compact_context 的 summary 字段。时间线布局、进度和取消行为继续遵循 2026-09-21 的命令时间线设计。

## 模型输出协议

| 命令 | 结果工具 | title | content |
| --- | --- | --- | --- |
| kickoff | kickoff_thread | 启动任务标题 | 完整 Markdown 启动简报 |
| handoff | handoff_thread | 交接主题标题 | 完整 Markdown 交接简报 |
| polish | polish_instruction | 本次指令优化概述 | 完整润色后的纯文本指令 |
| compact | compact_context | 当前任务、阶段或决策概述 | 完整 Markdown 工作上下文摘要 |

四个命令沿用单次模型调用，每次只注册对应的结果工具，不进入 Agent Run 工具循环。工具只提交生成结果，kickoff／handoff 不自动创建会话，polish 不自动应用到输入框。commit 和 rename 的工具协议不变。

title 与 content 均必填。title 是最多 48 个 Unicode 字符的非空单行纯文本，不含控制字符、HTML 或 Markdown 标题／列表标记；提示词建议中文标题 6–24 字，避免泛化命令名、命令前缀和句末句号。content 保留各命令既有内容职责和预算。kickoff／handoff 最多 40,000 字符，polish 最多 8,000 字符，compact 继续使用 4,000 输出 Token 预算。

后端共用 schema 构造和结果解析，要求恰好调用一次正确工具，参数仅含 title 与 content。纯文本输出、额外工具调用、错误工具名、缺失或非法字段均作为生成失败处理，不提取首行标题，不接受旧 summary 或 polished_instruction 结果协议，也不为非法 compact 标题生成替代值。无需调用模型时的空压缩区域仍保留既有处理。

## 存储与前后端协议

- BriefingVersion 新增 title，每个版本独立保存标题和正文；修订参考最近两个版本的 title、content 和全部反馈，生成完整替代结果。前端继续只展示最新版本。
- polish 请求仍使用 instruction；响应改为 title、content、changed、model_execution、prompt_version。命令状态、结果及重试输入写入统一活动表，详见 [时间线持久化设计](2026-09-22-timeline-persistence-design.md)。
- compact 的结果对象、数据库列、HTTP 响应、SSE 事件、历史恢复及前端类型统一使用 content。注入继续运行的 Agent 时，仅将 content 放入既有 context_summary 容器，title 不进入压缩正文。
- 数据库通过 0014 调整现有表结构；不为历史简报生成标题，不添加旧格式双读或历史事件回填逻辑。

## Timeline 与操作

四种文本结果均直接使用 title 展示摘要标题，content 展示详情。简报复制和新建会话草稿、polish 复制与应用操作只使用正文。polish 后续重试以最新 content 作为 instruction；生成及修订标题不混入正文。保留已有样式、阶段、取消、失败重试和长内容展开机制。

## 验证交接

按项目约定，未执行自动化验证。已更新现有接口、存储、事件、历史恢复和组件用例，补充统一结果解析及 polish 标题／正文分离的回归用例，交由用户运行。

手动关注：四个命令的标题与详情；简报带反馈及空反馈重试、刷新后恢复最新标题；polish 应用前不改草稿且应用后不包含概述标题；自动和手动 compact 的实时事件、历史恢复及后续任务继续执行；模型返回非法工具结果时不保存或应用内容。
