# Seed 初始化与 Runtime CSS 边界

> 2026-09-25 更新：完整协议见[统一资源设计](2026-09-25-resource-registry-design.md)。

## 结论

- 出厂预置资源位于仓库根目录 `seed/assets/`，包含 `themes/`、`components/`、`skills/` 和 `snippets/`；文件仅保存正文，元信息输入位于 `seed/resources.json`。
- `scripts/init-workroot.sh` 调用统一资源服务，复制缺失正文并登记数据库；已登记资源直接跳过，不覆盖用户已有文件。
- `restart.sh` 仅在数据库不存在时初始化；`--reset` 先清空工作目录再初始化，已有数据库的 `--no-reset` 不补回被删除的预置资源。
- 后端启动过程不再创建或复制 seed 资源。
- Skill 真源统一为 `WORK_ROOT/assets/skills/`；四类资源的名称、描述、标签与启停状态统一存储在 SQLite 的 `resources + tags + resource_tags`。

## Runtime CSS

`backend/internal/runtimeassets/base.css` 是幻灯片运行时依赖，不属于 seed。它定义主题无关的 16:9 舞台、缩放、基础排版和共享结构规则，并继续通过 `/api/v1/runtime/base.css` 提供给浏览器预览与 Chromium 渲染。

Theme CSS 提供变量，Runtime Base CSS 消费变量并建立公共结构，Slide HTML 提供每页内容。三者职责独立。
