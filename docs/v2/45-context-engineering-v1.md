---
id: V2-CONTEXT-ENGINEERING-V1
title: Context Engineering v1 运行时契约
status: implemented
owner: agent
depends_on: [V2-CONTRACTS, V2-AGENT-PIPELINE]
---

# Context Engineering v1

`backend/internal/contextengine` 是四种 Artifact Target 的唯一上下文入口。输入只包含服务端解析的
run/thread/project identity、`WorkSpec`、Blueprint v2 与 token budget；输出为类型化
`ContextPack` 和可审计 `ContextManifest`。

Profile 固定为 `blueprint/deck`、`blueprint/slide`、`presentation/deck` 和
`presentation/slide`。slide profile 只携带目标页完整 Blueprint；其他页只保留确定性摘要。
deck profile 默认不携带任意完整 HTML。Presentation HTML 先生成确定性 `HTMLSummary`，
需要正文时使用当前 Run manifest 绑定的 opaque `ContextRef`。

预算以稳定 token 估算为单位，依次裁剪低相关资产、远端页面摘要、完整 HTML、结构摘要和早期
可见对话。WorkSpec、目标 artifact、安全 policy 与 revision 永不裁剪；JSON/Blueprint
不会被截成无效片段。

Thread Memory 存在 `threads/<thread-id>.memory.json`，只在成功 Run 后原子更新。失败或取消
不提交；损坏文件安全重建。Memory 有长度上限且不保存 chain-of-thought。History 同样不再
持久化 thought 事件。

`read_context_ref` 只接受 `ref_id` 与 `detail=summary|structure|full`。它校验 run、thread、
project、revision、hash、detail capability 和剩余预算，并返回结构化错误：
`CONTEXT_REF_NOT_FOUND`、`CONTEXT_REF_FORBIDDEN`、`CONTEXT_REF_STALE`、
`CONTEXT_BUDGET_EXCEEDED`。
