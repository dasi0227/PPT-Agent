# HTML 生成参考快照与变化上下文

日期：2026-09-25

## 目标与边界

HTML 是最终创作产物；Manifest、Design、Spec 是指导创作的业务参考。Runtime 告知参考相对于该页上次 Agent 创作 HTML 时的累计净变化，由 LLM 结合本次任务决定是否修改 HTML。变化信息不是合规证明、同步状态或强制重写规则。

本设计替代此前 materialization 的文件、状态推导、前端徽标及 Spec/Design 强制同步规则；渲染证据与截图有效性继续独立工作。Outline 不进入生成参考快照或变化区域，但仍用于页面成员、顺序、结构校验及 Runtime Frame。

创作文件按 [目录与管理 UI 设计](2026-09-25-authoring-files-and-management-ui-design.md) 统一采用隐藏 JSON、集中 Spec 和平铺 HTML；人工 HTML 保存入口已移除。Spec 的共享存储不改变单页生成参考语义。

## 存储

SQLite `slides.generation_inputs_json` 为可空 TEXT，包含 `manifest`、`design`、`spec` 三个对象的完整业务内容，不保存 HTML、Outline、主题、时间、版本或 hash。不同页分别保存各自的旧要求，即使共享全局要求也不能共享同一份基线。

不再读写 `materialization.json`，删除其 Schema 和接口类型，不兼容读取、不迁移旧数据，也不主动清理本地历史文件；旧项目的已有 HTML 在没有有效数据库快照时显示未知基线。新列随正常数据库 Schema 初始化添加，历史行保持 NULL。

快照随页面身份进入现有 SQLite 事务、mutation journal 与请求摘要；数据库 receipt 与快照在同一事务提交，恢复用 journal 的摘要和 receipt 判断文件回滚或提交，不在恢复时重新读取参考生成快照。重复调用返回既有结果，迟到 receipt 重放不能覆盖更新的快照。页面删除删除整行，项目 checkpoint 捕获并恢复该列。

## 更新时机

每次模型响应执行工具前保留该请求的参考视图；同一响应按执行顺序将成功提交的参考修改合入候选视图。统一提交入口按规范化的真实文件路径识别 `<slide_id>.html`，覆盖 `mutate_ppt` 和已授权的 `run_command` 编辑，并检查命令改写后的结构和页面权限。

只有实际创建或改变 HTML 字节且工具提交成功，才提交候选参考快照；同内容写入不推进。若当次参考不完整，保持未知，不制造有效快照。参考修改、读取、变化发送、渲染和任务结束均不推进快照；后续刷新磁盘内容只用于新的上下文，不回写已捕获的生成快照。

选择不修改 HTML 时旧基线保留，没有已告知基线或变化流水；因此后续任务仍可能看到同样的净变化。

## 差异与模型上下文

专用区域为 `html_reference_changes/<slide_id>`，覆盖任务范围和明确引用的页面，全稿任务覆盖当前全部页面；无 HTML 的页面不输出，已有 HTML 但快照缺失或无效时仅输出 `{"baseline":"unknown"}`。

每类参考按 JSON Pointer 记录字段变化，示例：

```json
{
  "design": {
    "/direction": {"op":"replace","old":"密集信息图","new":"留白与大字结论"},
    "/layout_preferences": {"op":"replace","old":["网格","留白"],"new":["留白","网格"]}
  },
  "spec": {
    "/layout": {"op":"add","new":"两栏"}
  }
}
```

对象递归比较，JSON Pointer 对 `/` 和 `~` 转义；字符串保留全文，数组整体比较且顺序有效；新增省略 old，删除省略 new，忽略 JSON 排版和对象字段顺序。A→B→A 后净差异为空，不产生历史流水。

区域复用现有增量消息：相同内容不重发，改变后追加新值，差异消失或页面退出范围时追加 `null` 明确清空；压缩丢失区域元信息后，依据当前参考和该页快照重新构造。区域计入上下文预算、请求计量与压缩机制，正常的当前要求及读取工具继续提供完整当前内容。

## 渲染与前端

`RenderProof` 保留 HTML、参考来源、Outline 节点、Frame 和主题等渲染依赖指纹，只用于判断渲染与截图有效性，并随工具结果持久化以支持重放；它不写生成快照或 materialization 文件。参考引起的渲染证据失效保留在内部，模型不收到强制改写 HTML 的失效列表。

完成检查保留 Schema、结构引用、页面权限，以及真实 HTML 改动的静态检查和有效渲染证据；仅修改 Manifest、Design 或 Spec 可以完成任务。引用变化后已经改过的 HTML 如缺少当前渲染证据，可以要求重新渲染，但不能机械要求重写 HTML。

项目内容接口删除 `materialization`，`html_state` 仅为 `missing | available`，`html_hash` 来自真实文件字节并继续服务缓存和 DOM 引用。前端取消同步状态徽标和“待更新”，页面引用只显示“幻灯片已生成／未生成”；设计稿加载和任务状态独立管理。不修改 `DESIGN.md`。

## 验收

已补充定向测试源码，实施阶段只静态复核和格式化，未执行测试、构建或浏览器验证；以下步骤由用户手动执行。

后端：在 `backend/` 执行：

```sh
go test ./internal/spec ./internal/contextengine ./internal/workflow ./internal/service ./internal/projecthistory ./internal/store/sqlite ./migrations ./schemas
```

前端：在 `frontend/` 执行：

```sh
pnpm test -- src/features/deck/selectors.test.ts src/features/viewer/SlideSpecCard.test.tsx src/features/agent/promptMatching.test.ts
pnpm build
```

手动验收场景：

- 不同页面基于不同旧要求生成后修改全局参考，各页变化应使用自己的旧值，未生成页不显示变化。
- 仅修改参考并结束任务应可完成，页面 UI 保持已生成，已有截图按渲染依赖独立判定有效性。
- 同一响应先修改设计为 B、再修改 HTML、最后修改设计为 C 并渲染，保存的生成快照应为 B，变化区域显示 B→C。
- 原样重写 HTML、结构化管理 UI 修改参考、读取和渲染均不更新基线，真正的 Agent HTML 修改提交后才推进。
- 工具失败回滚、重复调用和崩溃恢复不得推进或回退已提交快照，未授权页面的命令编辑应被拒绝。
- A→B→A 应清空旧差异消息，压缩后与重新进入任务时应恢复当前净差异。
- 删除页面与 checkpoint 回退／恢复应同步移除或还原该页快照，旧 materialization 文件不被主动迁移或清理。
