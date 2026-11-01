# Electron 桌面运行接入

日期：2026-10-04。目标：当前 Mac 双击 `.app` 即可使用现有 PPT-Agent。用户已授权自动运行与界面验收。

实施状态：已接入 Electron 44.5.1 并成功编译 React 与 Go，产物为本机 ARM64 `.app`，使用 ad-hoc 本地签名。已通过实际打包应用的启动、模型创作与修改、预览、导出、持久化及退出恢复验收。

## 运行结构

Electron 主进程启动已编译 Go 服务，读取专属子进程的就绪信号并等待健康检查，再加载同源前端。Go 在 `127.0.0.1:8787` 同时提供前端构建、现有 HTTP API 与 SSE；React 路由刷新返回 `index.html`，未知 API 和缺失静态资源仍返回错误。复用现有协议、隔离 iframe 和交互，不增加前端业务分支。

采用固定地址保持 Electron localStorage 的 origin 稳定。监听端口与工作目录锁在数据库打开和恢复之前取得；端口占用不附着已有服务，不自动杀死网页开发服务。现有项目与资源保留在 `~/.dasi/ppt`。

## 资源与配置

应用资源中包含 Go server、首次初始化资源命令、前端构建、seed、render-worker 与实际 Playwright 文件，避免 pnpm 符号链接指回仓库。Go 已内嵌 prompt、字体与运行时样式；不额外复制仓库文件。

首次资源初始化与常驻后端共用子进程管理；初始化失败或被中断时退出不等待已经结束的进程。工作目录中的 `.desktop-initializing` 在初始化成功后才移除，防止新数据库已创建但资源尚未注册时被误判为完成。下次启动重试已有幂等初始化操作；初始化命令与 server 共用工作目录锁，不与运行中的服务并发写入。

render-worker 通过 Electron 可执行文件的 `ELECTRON_RUN_AS_NODE=1` 模式运行，该环境变量仅注入渲染子进程，避免影响通过文件打开操作启动的 Electron 编辑器；Chromium 复用当前 Mac 安装的 Chrome / Chromium / Edge。显式补齐 Homebrew 与系统 PATH，确保 Finder 启动能调用 Git。当前阶段依赖本机已有浏览器与 Git，不宣称独立跨设备分发。

模型配置独立放在 `~/Library/Application Support/PPT-Agent/config.yaml`，仅首次准备时复制已有配置；打包产物不包含 API Key。Electron 默认用户数据持久化在同目录，日志独立存放在 `~/Library/Logs/PPT-Agent/`。首次没有配置时通过原生文件选择器指定 YAML。

## 生命周期与权限

单实例运行；关闭窗口保留界面和任务，Dock 恢复同一窗口；退出应用先请求 Go 暂停运行、关闭 HTTP 服务并执行依赖清理，再退出 Electron。Go 监听父进程 stdin 断开，即使 Electron 崩溃也执行退出清理。退出超过 25 秒后才强制停止后端，并记录日志。

Renderer 不启用 Node，不暴露 preload / IPC，启用 context isolation、sandbox 和 webSecurity；拒绝 webview 和新窗口，顶层导航限于应用 origin，外部链接仅允许 HTTP(S)。本机文件操作继续使用既有同源限制，导出使用原生保存对话框。

## 验收边界

交付本机 `.app`、可重复执行的构建脚本和补充手动验收清单。2026-10-04 已通过 Playwright Electron 接口验证实际 `.app`，以系统 PATH 和仓库外工作目录启动；Computer Use 控制通道超时，用户明确授权使用 Playwright。

- 创建临时项目，真实 MiMo 模型生成一页 HTML、渲染并审查；追加修改后保存内容及画布、缩略图更新正常。
- 修复静态目录的 `index.html` 重定向被 SPA 回退覆盖的问题，隔离预览正常；未知 API 与缺失静态资源继续返回 404。
- PNG ZIP 包含 1920×1080 图片，PDF 为一页 16:9，均已渲染检查；HTML 演示包在独立 Chrome 中以本地文件打开正常。
- 导出经过实际 Electron DownloadItem 并落盘，原生保存对话框的 Downloads 默认路径正确。自动验收指定临时保存路径，未自动操作原生对话框。
- 模型名称通过设置界面保存至 App 专用 YAML，重启后保留；测试名称已恢复。项目、草稿、主题在退出重开后保留。
- 任务运行中关闭窗口继续执行；重复 LaunchServices 启动恢复同一窗口；关闭菜单与 Dock activate 恢复正常，菜单注册 Cmd+W。
- 运行中退出后任务以 `paused / server_shutdown` 保留，重开后状态正确；退出时包含 Go、worker、浏览器在内的 11 个应用进程全部消失。
- 最终通过 macOS LaunchServices 正常启动，无调试启动参数，健康检查返回 200；原项目保留，临时项目已删除，测试草稿及设置修改已清理。

运行证据位于本地忽略目录 `tmp/desktop-acceptance/`，不纳入版本控制。详见 `desktop/README.md`。
