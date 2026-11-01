# run_command 接入 sandbox-runtime

2026-10-30 用户确认接入。本设计替代 2026-10-16 shell 设计中的自建 Seatbelt、无限宿主读取、默认联网和无限输出约定。

## 模块边界

| 模块 | 处理 |
| --- | --- |
| `commandexec/executor.go` | 删除手写 profile 和直接调用 sandbox-exec；改为 Node worker 传输与生命周期适配 |
| `command-worker` | 新增独立包，固定 SRT 0.0.81；仅设置权限配置、调用公开 API 和管理进程/输出 |
| `commandexec/policy.go` | 保留 Bash 语法检查、源码摘要、显式 Git commit/push 业务限制；不作为 OS 安全边界 |
| `commandexec/workspace.go` | 保留副本、差异、受保护资源、符号链接与事务处理；候选元数据检查拒绝无法提交的硬链接和 xattr |
| workflow | 保留候选审批、内容校验、回滚与恢复；候选摘要增加策略指纹，恢复和提交前校验 |
| config / service | 启动时装入 sandbox 配置，经依赖注入传递；模型设置保存保留完整配置 |
| desktop / restart | 安装锁定依赖并打包 worker；App 复用 Electron Node；运行时不下载 |

沙箱文件访问和网络边界全部由上游实现。没有 fork、复制上游代码、自研网络代理、SBPL 生成器或无沙箱回退。
每条命令使用一个 worker，避免 SRT 全局单例在并发命令间共享策略、cwd 或代理。
前端工具参数、审批交互及结果结构不变；新增错误码沿用现有错误展示。

## 生效策略

- 默认离线；只读副本、必要系统/工具链及明确配置的额外目录。
- Create + executing 才可写副本；Chat/Ask/Plan 在 OS 执行层只读，scratch 可写。
- 拒读真实项目、应用数据/配置和常见凭据；受保护资源不可写。
- SRT 的共享写目录收紧到单命令 scratch；独立 HOME、TMPDIR 和缓存，进程环境使用白名单。
- 网络使用 strictAllowlist、私有地址拒绝和 SRT DNS 结果检查，禁止开放本机服务、Unix socket、Apple Events 或 weaker isolation。
- 默认时限 120 秒，stdout/stderr 合计 8 MiB；超限终止，不提交候选。
- 权限配置、worker 实现及锁文件纳入策略指纹；旧候选不迁移、不重跑。
- 不增加底层资源配额或容器能力；主动脱离进程组的后代回收是已知边界。

配置示例见 [config.example.yaml](../../backend/config.example.yaml)。安装步骤、权限细节、已知边界和完整手动验收项见 [command-worker README](../../backend/command-worker/README.md)。
第三方版本和许可证见 [THIRD_PARTY_NOTICES](../../backend/command-worker/THIRD_PARTY_NOTICES.md)。

## 验证状态

按项目约定，未执行测试、构建、应用启动或自动化验证。已调整与新增针对权限边界、输出上限、环境清理、策略变更后审批失效和配置保存的回归用例，留待用户手动运行。
