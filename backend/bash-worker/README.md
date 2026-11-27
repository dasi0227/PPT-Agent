# Bash worker

`BashGateway` 通过 `worker.mjs --project` 将命令和执行上下文发给 `project-worker.mjs`。worker 建立临时副本，保存文稿基线，并启动独立的 `sandbox.mjs` 执行命令。沙箱退出后，worker 比对文稿、清理副本，通过标准输出返回命令结果和候选变更。审批与写回由 Java 领域层负责。

两端都需要 Node.js 22.12.0 或更高版本。macOS 使用系统 Bash 和 Seatbelt，Windows x64 / ARM64 使用系统自带的 Windows PowerShell 5.1，以及 sandbox-runtime 随包提供的 `srt-win.exe`（Windows 后端为 Alpha）。在仓库根目录执行以下命令安装锁定版本。

```sh
cd backend/bash-worker
npm ci --omit=dev --ignore-scripts
```

macOS 的 `./restart.sh --mode web` 会自动安装上述依赖。

Windows 请使用本地 NTFS 卷。沙箱账户需要能够读取并执行已安装的 `srt-win.exe`。在 `backend/bash-worker` 目录下用 PowerShell 完成一次机器初始化，按提示批准管理员权限。

```powershell
.\node_modules\.bin\srt.cmd windows-install --skip-profile-grant
```

worker 在每次会话中限制宿主用户目录的读取，并开放项目副本、临时工作区和已配置的读取目录。其他位置的读取遵循 Windows 沙箱账户已有的 NTFS 权限。网络过滤和进程回收使用上游的 Windows 后端。[上游 Windows 支持说明](https://github.com/anthropics/sandbox-runtime#windows-alpha)

Java 后端通过 `ppt.sandbox` 配置允许读取的目录、允许访问的域名、执行时限和输出上限。默认以 `backend` 为工作目录，通过 `bash-worker/worker.mjs` 加载入口。`PPT_BASH_WORKER` 和 `PPT_BASH_NODE` 可分别指定入口和 Node.js 可执行文件。
