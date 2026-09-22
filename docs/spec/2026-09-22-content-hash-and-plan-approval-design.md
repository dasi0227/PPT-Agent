# 内容指纹与计划审批简化设计

## 决定与适用范围

本文替代旧文档中创作文件、HTML、渲染图片与 Plan 的递增 revision 约定。内容历史由项目 checkpoint 管理；内容一致性使用 hash；确有并发职责的 Scope、模型设置、会话命名和项目历史 revision 保留。scene_revision 继续作为历史现场切换标识，不扩大本次重构范围。

作者文件与 materialization Schema 一次性切换至 5.0。前后端同步采用新协议，不保留旧结构兼容层，不迁移历史开发数据。已有本地开发项目及旧 checkpoint 不保证可继续使用。

## 创作内容与写入校验

- manifest、outline、design、slide spec 删除 revision，保留 Schema version、稳定身份与创建/更新时间。
- ResourceHash 规范化 JSON 的字段顺序和格式，排除根级 created_at / updated_at；业务字段变化才改变内容指纹。
- read_ppt 返回当前内容及 content_hash；mutate_ppt 与 HTTP mutation 使用可选 expected_hash 做内容前置校验。大纲 UI 写操作携带读取快照里的 outline hash。
- 资源快照提供 hashes（manifest、outline、design、spec:<slide_id>），mutation 返回 hashes；不把 hash 再写入作者文件。
- HTML 的内容身份由实际文件字节计算。页面快照提供 html_hash，即使没有有效渲染证明也可以识别和预览当前 HTML。
- 相同内容的写入不改文件、不刷新时间戳、不产生页面失效清单。已提供但不匹配的 expected_hash 拒绝写入；RunSession 原有文件前像校验、项目锁和持久化 receipt 继续保留。

## 渲染、引用与缓存

- materialization.artifact 仅保留 hash；source 中 manifest_revision / spec_revision 改为 manifest_hash / spec_hash，保留 outline_node_hash、design_content_hash、组合来源 hash 和 frame.context_hash。
- 来源组合 hash 使用 manifest/spec 的规范化内容指纹、单页语义大纲指纹与原有设计内容投影，更新时间和 JSON 格式变化不使 HTML 过期。
- HTML 内容不符为 unknown；manifest/spec/当前大纲节点不符为 spec_stale；设计内容不符为 design_stale；运行时页框不符为 frame_stale。
- SQLite slides 删除 current_version；上下文 revision 索引、引用 revision、工具变更目标 revision 和渲染图片 revision 一并删除。上下文引用与截图沿用已有 hash 校验。
- 前端 HTML 缓存使用 project_id + slide_id + html_hash；DOM 标记、预览父子窗口通信和 checkpoint 恢复引用统一绑定当前 HTML hash，不再依赖上一次渲染证明的编号。
- 修改主题后重新读取权威内容快照，以更新 design hash，前端不自行合成文件元数据。

## Plan 审批

- Plan 删除 revision / approved_revision，保留执行时的 approved_content_hash 校验。
- 每次完整新提案分配唯一 approval_id，随 Plan 存入 Runtime checkpoint；恢复继续使用原 ID，替换提案生成新 ID。
- 审批 interaction_id 直接使用 approval_id。回复只需匹配 interaction_id、plan_id 与决策；相同回复重放维持幂等，不再次投递执行。
- 新提案作废同一 Plan 的旧待审批请求。批准后计划正文锁定，步骤进度更新不创建新审批。
- 前端 Plan 投影使用已有的 run_id + SSE event sequence 排序，删除独立计划修改计数。审批通过后的输入模式恢复仍以服务端执行状态为准。

## 验证

覆盖规范化 hash、元数据/格式不影响渲染来源、重复写入不失效、旧 hash 拒绝写入、HTML 引用过期、审批 ID 恢复与替换、旧审批拒绝、事件排序及计划执行恢复。更新受协议变化影响的原有测试。

不使用浏览器或计算机自动化；主预览、DOM 标记、主题切换及历史恢复交互由用户手动验收。
