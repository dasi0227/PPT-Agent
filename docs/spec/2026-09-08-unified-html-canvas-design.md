# 统一 HTML 画布与 Runtime 预览设计

## 目标

HTML 幻灯片统一以 **1920×1080 CSS px（16:9）** 设计。页面作者只处理这个固定坐标系；编辑器主预览、缩略图、全屏放映和 Chromium 截图均由 Runtime 等比缩放并居中，不再让各页面自行适配浏览器容器。

## 运行契约

- `backend/internal/spec` 定义唯一的规范画布常量，并将它写入每页 Runtime frame。
- Manifest 仅接受 `16:9`，当前不提供未实现的 `4:3` 路径。
- Runtime frame 同时携带画布、主题、页码、章节和公共装饰上下文；前端和渲染器均消费这一帧，而不从页面 HTML 推断。
- 主题继续声明 `--stage-w: 1920` 与 `--stage-h: 1080`，但基础样式以规范画布的固定尺寸渲染，主题不能改变实际画布。

## 预览与渲染

前端 Runtime 在每个容器内建立一个 1920×1080 内画布，使用 `min(containerWidth / 1920, containerHeight / 1080)` 缩放、居中显示。嵌套 iframe 始终以完整设计尺寸加载，所以主预览、缩略图和全屏都使用相同的页面坐标。

前端在加载 `srcdoc` 前按与后端相同的规则补入 Runtime 基础样式和当前主题样式。Chromium 渲染器以 frame 提供的画布建立 viewport，并将公共装饰附着到 `.slide-stage`；页面预览和截图因此共享主题、画布、页码与章节信息。

## 公共装饰

页码、章节标识和标题由 Runtime 放置在设计画布内，位置使用同一组百分比锚点。`style` 文本中的已支持语义词为 `tiny`、`compact`、`muted`、`mono`、`label`；它们只改变 Runtime 装饰样式，不允许页面 HTML 自行生成同类装饰。

## 非目标

本次不增加多比例画布、导出 UI 或页面动画生命周期。导出功能接入时必须使用 Runtime frame 的画布，而不是另设尺寸常量。
