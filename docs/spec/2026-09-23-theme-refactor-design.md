# HTML 主题契约与运行时重构

日期：2026-09-23。状态：代码已实现；自动测试与视觉验收待用户执行。

## 目标与边界

同一份 HTML 在不改写内容、布局和页面脚本的情况下，仅切换主题 CSS，即可呈现明显不同的字体、配色、背景、边框和容器材质。预置主题仅保留 Editorial Serif、Blueprint、Bold Signal，新项目默认 Editorial Serif。固定 1920×1080 画布、隔离 iframe、现有主题入口继续使用；不添加 T 快捷键。

本次一次性使用新协议，不转换历史 HTML，也不为退役主题回退到默认主题。`DESIGN.md` 未修改。

## 权威文件与职责

| 职责 | 权威位置 | 说明 |
| --- | --- | --- |
| 创作合同 | `backend/prompts/core/html.md` | 唯一的角色语义、变量含义、局部增强及 Canvas 事件说明 |
| 公共角色与必需变量 | `backend/internal/designsystem/tokens.go` | 上下文选择器列表与主题资产完整性校验共用 |
| 基础视觉消费 | `backend/internal/runtimeassets/base.css` | 公共角色消费语义变量，不规定页面内容和布局模板 |
| 三个主题 | `seed/assets/themes/*/theme.css` | 字体、配色、容器与背景特征；每个主题一份 CSS |
| 共用示例 | `backend/internal/runtimeassets/examples/` | cover/content/chart；仓库真实预览和 Agent 组合参考使用同一份文件 |
| 公共装饰 | `backend/internal/runtimeassets/chrome.js` | 编辑器、截图和离线播放器共用，保持在页面 iframe 外 |
| 字体与资源快照 | `backend/internal/runtimeassets/` | 编译期嵌入；字体二进制不进入模型上下文或 worker JSON |

Runtime 负责画布、缩放、隔离和外观资源。Theme 负责字体、文字层级、颜色和材质。Agent 负责内容、盒子组合、阅读顺序、Grid/Flex、宽度与位置，可以通过局部类做适度增强。

公共角色覆盖标题/小标题/正文/辅助文字、普通/轻量/描边/强调容器、指标、引用、表格、标签、分隔线、SVG 数据系列及流程节点。主题上下文使用 public_roles 表示公共角色，不使用暗示选择器白名单的 allowed_selectors。没有强制卡片、固定 DOM 层级或整页模板。自定义布局类与公共视觉类组合；不新增审美写入阻断。

样式顺序统一为：字体 → base.css → theme.css → 页面样式。局部样式可覆盖布局所需属性；创作合同要求保留主题变量和共享类的所有权。组件职责于 2026-09-24 调整：组件保留独立设计，主题接入为显式选择，组件仓库使用中性隔离预览，不加载 PPT 主题。以 [组件边界与仓库预览设计](2026-09-24-component-boundaries-and-preview-design.md) 为准。

## 主题方向

| 主题 ID | 视觉方向 |
| --- | --- |
| editorial-serif | 奶油纸色、深褐文字、锈红强调；中文衬线标题、细线、轻容器 |
| blueprint | 深蓝底、暖黄标注、低对比网格、虚线节点、无柔和投影 |
| bold-signal | 近黑底、白字与明黄、粗重标题、硬边与偏移硬阴影 |

三套主题的画布尺寸相同，不强制改变内容顺序或布局。标题尺度、字重、行高等由主题定义，必要的页面特例在局部类中调整。

## 字体

内置 Noto Sans SC、Noto Serif SC 和 JetBrains Mono 可变 TTF，完整保留中文覆盖。字体文件、OFL 许可证、来源与 SHA-256 记录在 `backend/internal/runtimeassets/fonts/`。当前三份字体共约 41 MiB；这是完整中文字体带来的仓库和离线包体积成本。

预览通过本地只读接口加载；字体准备器只在完整加载成功后安装 FontFace，失败重试会创建新对象，不会继续复用 rejected FontFace。截图 worker 从冻结的资源目录读取相同字节。HTML 包的共享 fonts.css 内嵌 data URL，避免 `file://` 与 opaque iframe 的字体跨域问题；字体只打包一次，不重复到每一页或 JSON。许可证单独保留。

## 数据与接口

`Theme` 新增 `style_hash` 和 `appearance`。`style_hash` 是实际 CSS 文件字节的 SHA-256；名称不再参与前端字体猜测。

`ProjectContentSnapshot` 和 `RuntimeFrameContext` 新增外观描述：

```json
{
  "appearance": {
    "hash": "sha256:…",
    "theme_css_url": "/api/v1/themes/blueprint/css?v=…",
    "chrome_tokens": {
      "--color-caption": "…",
      "--color-fg": "…",
      "--font-sans": "…",
      "--font-mono": "…"
    }
  }
}
```

外观哈希覆盖主题 CSS、基础样式、字体资源、字体准备器、主题桥和公共装饰实现。页面框架哈希继续覆盖页序、数量、主题、章节和装饰配置，并纳入 appearance。相同 ID 的 CSS 修改会使截图和物化证明失效，HTML 正文哈希不变。

项目快照的 `hashes.appearance` 让现有可见项目轮询识别外观变化；主题不可用时 appearance 为 null，并返回 theme_error，页面提示重新选择主题。不会选择隐式替代主题。

主题 CSS 的 `v` 参数校验实际字节版本；如果加载期间 CSS 已被修改，返回冲突，避免把新文件误认为旧指纹。渲染完成时也核对实际帧哈希，拒绝外观已变化的渲染证明。

新增只读端点：

- `/api/v1/runtime/fonts.css`
- `/api/v1/runtime/font-loader.js`
- `/api/v1/runtime/theme-bridge.js`
- `/api/v1/runtime/fonts/:name`
- `/api/v1/runtime/theme-examples/:name`，name 为 cover/content/chart，响应 `{ "html": "…" }`

主题保存继续使用原接口。没有增加 Agent 主题写入工具。HTML 读取接口直接返回按正文哈希绑定的原始 HTML，资源注入由 Runtime 完成。

## 无刷新切换

同一页且 HTML 字节相同，复用已有 iframe。外观更新时：

1. 生成递增请求号，立即取消旧请求的提交资格。
2. 外层准备字体；内层以非活动 stylesheet 加载基础 CSS 和目标主题 CSS，保持原画面。
3. 内层字体与 CSS 就绪后替换主题 link，页面 CSS 始终位于其后。
4. 派发 `window` 上的 `ppt:themechange`，detail 包含 themeId 和 appearanceHash，供 Canvas 重绘。
5. 主题桥返回成功，外层按同一外观更新公共装饰，并通知 React。
6. 对当前正文的已有选择重新解析 DOM，更新位置、computed style 和装饰快照；不可靠的目标要求重新选择。

消息：外层 `themeApplying` / `themeApplied` / `themeApplyFailed`；内部桥 `ppt-theme-v1`，包含请求号和 slide_id。消息仅接受对应 iframe 的 source。过期完成事件不能更新当前外观状态。

失败保留旧主题，可在原 iframe 内重试，不重跑页面脚本、不触发主动重播。主题保存成功通知写为“已保存…画布将加载新外观”；预览失败另行提示，不把保存成功等同于应用成功。

## 渲染与导出

截图器使用外层画布 + 内层页面 iframe，执行共享 chrome.js，去掉原先重复的固定灰色、系统字体和定位样式。页面内容诊断来自内层 iframe，截图与公共装饰来自外层画布。

导出创建冻结资源副本，帧数据携带外观指纹。PNG/PDF worker 接收资源目录路径及 CSS 快照，不接收字体二进制 JSON。HTML 包携带基础 CSS、主题 CSS、字体 CSS、许可证及共享装饰代码；页面按相同顺序加载，页面局部样式最后生效。

## 预置替换

新增显式脚本，普通 `init-workroot.sh` 仍只补充缺失文件，不覆盖用户资源：

```sh
python3 scripts/replace-theme-presets.py --work-root /Users/wyw/.dasi/ppt
```

本次已对该实际 WORK_ROOT 执行。脚本只覆盖三个预置主题及五个预置组件，移除 swiss-modern / corporate-clean / warm-pastel / tokyo-night / xiaohongshu-white 五个退役主题目录。保留全部项目、附件、技能与其他自定义资源。历史项目保留原主题引用，由用户重新选择；不自动重写页面。

资源被编译进后端，修改内置字体、base、chrome 或示例后需要重启使用新构建的后端。主题 CSS 位于 WORK_ROOT，可原位更新；现有轮询会发现内容指纹变化。

## 验收与测试交付

本次按项目约定未执行构建、自动测试或浏览器验证。下面命令由用户执行；新增断言集中于竞态、失败恢复、外观失效和离线资源，未锁定主题 CSS 数值。

后端：

```sh
cd backend
go test ./internal/runtimeassets ./internal/runtimehtml ./internal/spec ./internal/service ./internal/httpapi ./internal/export ./internal/workflow ./internal/contextengine ./internal/pptmutation
```

前端：

```sh
cd frontend
pnpm tsc
pnpm test src/features/viewer/themeBridge.test.ts src/features/viewer/slideRuntime.test.ts src/features/viewer/IsolatedSlidePreview.test.tsx src/features/viewer/previewProtocol.test.ts src/features/viewer/runtimeFrame.test.ts src/features/repository/RepositoryPages.test.tsx src/stores/projectStore.test.ts
```

人工验收：

1. 重启前后端，确认仓库只出现三个预置主题，新项目默认 Editorial Serif。
2. 在主题仓库分别查看封面、内容、图表；相同 HTML 下三个主题有明显差异，中文长标题、正文、表格和 SVG 清晰且不溢出。
3. 让 Agent 生成对比、流程、数据三种页面，确认布局可不同，公共视觉由主题承担，局部增强使用变量。
4. 在有计数器或进行中动画的页面切换主题，确认当前页和脚本状态保留；连续切换最终与最新选择一致。
5. 手动阻断主题 CSS 或字体请求后切换，确认旧画面保留、显示失败；解除阻断后点重试，脚本状态仍保留。
6. 选择标题或容器后切换主题，确认选框与样式快照更新；无法匹配时显示重新选择提示。
7. 比较主题仓库、主画布、缩略图、全屏、HTML 包及 PNG/PDF：字体、颜色、页码和装饰一致；断网打开 HTML 包也可播放。
8. 保持主题 ID 不变，修改 WORK_ROOT 的 CSS：正文哈希不变，旧截图变为过期；退役主题项目明确提示选择现有主题。
