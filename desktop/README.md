# PPT-Agent macOS

当前阶段面向本机 Apple Silicon / Intel 架构构建，应用内复用 React 页面与 Go 服务。

## 构建

先安装现有前端和 render-worker 依赖，再在 `desktop/` 执行：

```sh
npm ci
npm run prepare:local
npm run package
```

产物：`desktop/out/PPT-Agent-darwin-<arm64|x64>/PPT-Agent.app`。可从 Finder 双击，或拖到“应用程序”和 Dock。应用已包含前端构建、Go 二进制、预置资源、render-worker 与 Playwright；渲染使用 Electron 自带 Node 和本机已安装的 Chrome / Chromium / Edge，Git 从 Homebrew 或系统路径定位。

本机需要已有 Chrome / Chromium / Edge 和 Git；不需要启动终端、Vite 或 Go。首次切换前退出正在运行的网页开发服务（默认端口 8787）。端口冲突时 App 会失败退出，不接管或杀死其他进程。

## 通过脚本切换运行模式

在项目根目录执行：

```sh
./restart.sh --reset 0 --mode app
./restart.sh --reset 0 --mode web
```

默认 `--reset 0 --mode web`。`0` 保留数据，`1` 清空双方共用的 `~/.dasi/ppt` 后重新初始化；模型配置与 Electron 界面偏好保留。旧的无值 `--reset` 和 `--no-reset` 不再支持。

App 模式重新打包当前代码并打开 `.app`；Web 模式启动 Go 与 Vite 并打开浏览器。切换前正常退出已有 App 和本项目服务，等待后台清理完成；其他进程占用端口时中止。脚本修改仅做静态检查，按用户要求未执行启动或重置验收。

## 数据与生命周期

- 项目、数据库、资源保持在 `~/.dasi/ppt`；新工作目录自动初始化预置资源，已有数据不覆盖。
- 模型配置位于 `~/Library/Application Support/PPT-Agent/config.yaml`。`prepare:local` 仅在不存在时复制本地配置，密钥不会装进 `.app`。后续模型设置保存到此文件。
- 界面偏好和草稿由 Electron 保存到同一用户数据目录；本地页面固定使用 `http://127.0.0.1:8787`，重启不会因端口变化丢失浏览器存储。
- 日志位于 `~/Library/Logs/PPT-Agent/backend.log`。
- 红色关闭按钮 / Cmd+W 隐藏窗口并保留任务；点击 Dock 恢复。Cmd+Q / 菜单退出暂停任务、关闭服务与渲染进程。
- 父进程意外结束时，Go 通过 stdin 关闭检测执行退出清理；单实例锁与工作目录锁防止重复服务。
- 首次资源初始化中断后，下次启动继续初始化；启动失败的子进程也能正常退出，不让 App 持续挂起。

## 手动验收

2026-10-04 已自动验收本机 ARM64 打包应用：实际模型创作与修改、预览及缩略图、PNG/PDF/HTML 导出、设置与草稿持久化、窗口隐藏与恢复、重复启动、运行中退出与暂停恢复。导出文件已落盘并检查，自动验收使用临时保存路径；下列清单也可用于后续人工回归。

1. 停止原网页开发服务，双击 `.app`，确认独立窗口自动出现且现有项目可见。
2. 创作或继续对话，让 Agent 修改并保存 HTML，确认预览、缩略图、文件打开和各格式导出。
3. 输入未发送草稿、修改偏好，Cmd+Q 后重开，确认项目、草稿及设置保留。
4. 任务运行时关闭窗口，点击 Dock 恢复；再在运行中 Cmd+Q，确认后台进程退出，重开后任务状态正确。
5. 重复双击只恢复现有窗口；网页服务占用端口时启动 App，应明确失败且不影响原服务。

开发启动：先 `npm run build`，再 `npm start`。每次代码修改后重新打包。此阶段仅生成本地签名的 `.app`，不包含公证、自动更新或公开分发流程。
