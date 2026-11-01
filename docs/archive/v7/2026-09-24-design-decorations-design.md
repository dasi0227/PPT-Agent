# 视觉要求预览与页面装饰结构

## 用户确认的结构

`design.json` 的 `chrome` 数组由 `decorations` 固定对象替代。四个键必须完整存在，不允许任意类型或重复实例，每项仅含 `placement` 和字符串 `requirements`。

```json
{
  "decorations": {
    "page_number": { "placement": "bottom-right", "requirements": "" },
    "deck_title": { "placement": "none", "requirements": "" },
    "section_title": { "placement": "none", "requirements": "" },
    "key_message": { "placement": "none", "requirements": "" }
  }
}
```

- 页码初始化为右下角，始终显示，不接受 `none`；其位置可以后续调整。
- 其余项初始化为 `none`，表示暂不展示；不使用空位置、缺字段或额外 enabled 字段表达隐藏。
- `requirements` 为空表示沿用主题默认外观。它不是列表，不保存文本内容，不控制显隐。
- 页码取目录序号；演示标题取 Manifest.title；章节标题取当前一级章节标题；核心信息取当前页 Slide Spec.key_message。没有内容时不渲染对应项。
- `section_title` 替代原 `section_marker`，含义为一级章节标题，不自动切换为子章节标题。

## 外观要求的执行边界

公共渲染器支持有限、明确的外观表达，Agent 根据用户意图组合这些表达：

- `tiny` / `small` / `小字号`：14px；`compact` / `紧凑`：16px。
- `mono` / `等宽`、`sans` / `无衬线`：使用主题字体。
- `bold` / `加粗`、`regular` / `常规字重`：700、400 字重；`label` / `标签` 为 16px 加粗。
- `muted` / `弱化` / `低对比`、`foreground` / `正文色`：使用主题颜色。
- `字号 18px` / `font-size: 18px`：指定字号，范围 12–48px。
- `颜色 #345678` / `color: #345678`：指定十六进制颜色。

不将任意自然语言、HTML 或 CSS 当作可执行代码。未识别的要求不会自动生成图形、边框或特效，提示词禁止 Agent 声称这些效果已生效。显式字号、颜色优先于通用词。Agent 应避免为不同装饰指定同一位置。

## 文档预览

内容要求和视觉要求复用 DocumentSection：大标题 24px，顶层业务字段标题 18px 半粗，正文 14px。视觉要求展示主题、视觉方向、页面装饰，每个标题下方展示正文，不再使用横向 key/value 行，不加水平分隔线。

页面装饰下固定列出页码、演示标题、章节标题、核心信息，子标题为 14px 半粗；每项展示位置及额外要求。`none` 显示为「暂不展示」，空要求显示为「暂无额外要求」。

## 实现边界

同步切换前后端类型、Schema、项目初始化、命名 Patch 路径、Agent 上下文、框选数据、预览协议、渲染诊断及导出。内部装饰选区使用 decoration_targets，主题外观使用 decoration_tokens，公共资源使用 decorations.js。与浏览器名称、侧栏外壳等无关的 Chrome 命名不受影响。

核心信息随 Runtime frame 传递，并参与 frame hash；离线播放快照携带相同数据。编辑器、放映、图片/PDF 和离线 HTML 使用同一装饰渲染器。

直接切换新结构，不保留旧字段双读或兼容分支，不自动迁移已有项目文件。未运行自动化测试和浏览器验证，按项目约定由用户手动验证。
