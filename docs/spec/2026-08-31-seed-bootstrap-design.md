# Seed 初始化与 Runtime CSS 边界

## 结论

- 出厂预置资源位于仓库根目录 `seed/assets/`，包含 `themes/`、`components/` 和 `skills/`。
- `scripts/init-workroot.sh` 将缺失的预置文件复制到 `WORK_ROOT/assets/`，不覆盖用户已有文件。
- `restart.sh` 在启动后端前调用初始化脚本；`--reset` 先清空工作目录，`--no-reset` 保留数据并补齐缺失文件。
- 后端启动过程不再创建或复制 seed 资源。
- Skill 真源统一为 `WORK_ROOT/assets/skills/`；资源标签与 Component、Skill、Prompt 停用状态统一存储在 SQLite。

## Runtime CSS

`backend/internal/runtimeassets/base.css` 是幻灯片运行时依赖，不属于 seed。它定义主题无关的 16:9 舞台、缩放、基础排版和共享结构规则，并继续通过 `/api/v1/runtime/base.css` 提供给浏览器预览与 Chromium 渲染。

Theme CSS 提供变量，Runtime Base CSS 消费变量并建立公共结构，Slide HTML 提供每页内容。三者职责独立。
