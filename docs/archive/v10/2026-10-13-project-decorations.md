# 视觉要求与人工装饰配置分离

日期：2026-10-13

状态：实现已落地；构建、测试与运行验证按项目约定交由用户手动完成。

## 设计理由

`design` 是 Agent 创作时参考的视觉要求，不再承担页面装饰的实际设置。装饰与项目选用的 theme 一样，由人工配置；Runtime 继续统一绘制，创作 HTML 不重复实现这些元素。

原 `key_message` 装饰直接使用页面的 `spec.core`。本次将该装饰替换为项目共用 logo，保留 `spec.core` 的内容规划职责；页码、演示标题、章节标题的来源和其余绘制规则不变。

## 数据结构

`.design.json` 只包含两个必需的字符串数组，单组最多 32 条、单条最多 600 字符。默认值均为 `[]`。

```json
{
  "requirements": [],
  "prohibitions": []
}
```

`edit_design` 只修改这两组视觉需求与视觉限制。传入数组整体替换原数组，省略的字段保留原值，空数组清除对应列表。视觉要求变更不直接重写 HTML。

项目数据库保存 `decorations_json` 和可空的 `logo_json`，与现有 `theme` 并列。默认装饰设置如下：

```json
{
  "slide_pagination": "bottom-right",
  "deck_title": "none",
  "section_title": "top-left",
  "logo": "none"
}
```

- 位置沿用六个固定位置；页码、演示标题、章节标题和标志均允许 `none`（不显示）。
- 每个非 `none` 位置只能由一个装饰占用，即使对应内容暂时为空或 logo 尚未上传。
- 页码来自 outline 页序，演示标题来自 `manifest.title`，章节标题来自 outline 顶层章节。
- 上传 logo 不自动改变其显示位置或其他装饰设置。

## Logo 存储与显示

复用现有图片验证能力，接受 PNG/JPEG，最多 10 MiB、40,000,000 像素。数据库保存图片内容哈希、格式、原始文件名、字节数和原图尺寸，不存图片二进制。

当前图片保存于 `<work_root>/projects/<project_id>/branding/logo.png` 或 `logo.jpg`。该目录位于 Agent 创作目录 `artifacts` 外；创作工具不能修改它。

同一项目只有一张当前图片。重新上传替换当前图片；跨格式替换删除旧文件，文件或数据库保存失败恢复原图片和引用。清除操作删除当前文件并将数据库引用置空。历史快照与导出冻结素材保留各自对应的图片。

Runtime 在指定位置使用 **128 × 64 CSS 像素**的固定显示框，坐标基于 1920 × 1080 画布；图片等比缩放、保持完整，不拉伸或裁切。未上传或位置为 `none` 时不绘制。

## 人工后端接口

以下接口不注册为 Agent 工具；本轮不增加前端设置入口。

| 方法与路径（均位于 `/api/v1`） | 请求与返回 |
| --- | --- |
| `GET /projects/:id/decorations` | 返回完整四项位置配置。 |
| `PATCH /projects/:id/decorations` | 请求为非空位置更新对象；合并后校验，返回更新后的项目。 |
| `GET /projects/:id/logo` | 返回当前图片元数据；未上传时返回 `null`。 |
| `PUT /projects/:id/logo` | `multipart/form-data` 的 `file` 必须且只能包含一张图片；返回更新后的项目。 |
| `GET /projects/:id/logo/content` | 返回当前图片；可通过 `v` 指定内容哈希，不匹配时拒绝返回其他版本。 |
| `DELETE /projects/:id/logo` | 清除当前图片，返回更新后的项目；重复清除为正常无变化。 |

配置写入遵循项目历史门禁及项目写锁，存在未结束 Run 或 Git 提交时不修改。项目详情和内容快照都提供独立的装饰信息；图片元数据与图片内容分别读取。

## 上下文与渲染

- 主 Agent 的 `project_decorations` 只读模块披露当前位置、图片是否存在和固定占位尺寸，供 HTML 预留空间；Reviewer 同样获得这份信息。
- 预览、截图与离线播放继续共用 `decorations.js`，装饰位于 slide iframe 外。截图等待 logo 解码完成。
- 截图复用依据为 HTML、实际引用素材、实际 theme 样式和可见装饰内容及位置。隐藏标题、`spec.core`、视觉指导本身不使相同像素失效。
- manifest、outline、spec 和 design 的内容证据仍独立保存、校验，截图复用不等于这些内容已经满足需求。
- 导出冻结装饰配置与 logo 文件，离线 HTML 包包含自己的 logo，不依赖在线 API。
- 项目历史捕获与恢复包含 `branding`，同时恢复对应数据库配置和图片。

## 提示词与前端适配

更新 `backend/prompts/tools.json`、`core/reference.md`、`core/output.md` 和 `playbook/html.md`，删除 Agent 修改装饰的说明，记录新的视觉需求、视觉限制与人工 logo 边界。其余审定提示词保持原状。

前端仅完成协议适配：design 编辑改用两组数组，删除原装饰编辑控件；预览从项目快照读取装饰与 logo，批注和语义名称同步切换到 logo。保留已有预览与全屏修改，不新增 logo 或装饰设置界面。

## 直接切换与人工验证

数据库格式从 6 切换到 7，旧数据库按现有机制拒绝加载，需使用新的工作目录；不执行旧数据迁移，不保留旧字段、兼容读取或兼容写入。

已有测试夹具已按新结构更新，装饰位置、图片替换与回滚、可见装饰缓存及离线 logo 用例供人工执行。人工核验应覆盖：上传、跨格式替换、失败保留原图、清除、位置冲突拒绝、历史切换，以及预览/PNG/PDF/离线 HTML 的同图同位置展示。
