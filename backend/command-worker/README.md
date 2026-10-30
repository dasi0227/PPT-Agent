# Command worker

`run_command` 的独立执行适配器，使用未经修改的
[`@anthropic-ai/sandbox-runtime@0.0.81`](https://github.com/anthropics/sandbox-runtime)。
上游生成 OS 沙箱、匹配文件权限、创建网络代理并检查域名解析结果；本目录只配置上游并管理命令生命周期。

- `policy.mjs`：把应用配置转换为 SRT 配置，不生成 Seatbelt profile，不实现代理。
- `worker.mjs`：每条命令一个进程，调用 `SandboxManager.initialize / wrapWithSandbox / reset`，处理有界输出、超时和取消。
- Go `internal/commandexec`：传输、进程组清理、快照和候选事务；审批仍属于 workflow。
- 只接通 macOS。Node >=22.12.0；缺少依赖、配置无效或启动失败均拒绝执行，没有裸 shell 降级。

开发安装：

```sh
cd backend/command-worker
npm ci --omit=dev --ignore-scripts
```

`scripts/restart.sh --mode web --reset 0` 自动安装锁定依赖后启动服务。
App 构建也会安装并复制完整依赖、许可证；运行时使用 Electron Node，无需用户全局安装 Node、SRT 或 ripgrep。

## 权限配置

网页开发配置为 `backend/config.yaml`；App 配置为
`~/Library/Application Support/PPT-Agent/config.yaml`。省略 `sandbox` 即采用以下默认值：

```yaml
sandbox:
  read_roots: []
  network:
    allowed_domains: []
  execution:
    timeout_seconds: 120
    max_output_bytes: 8388608
```

额外读取目录填绝对路径，例如 `/Users/your-name/Documents/ppt-reference`。
网络按需配置明确域名和端口，例如 `registry.npmjs.org:443`；支持上游的 `*.example.com:443` 形式。
不填端口意味着该域名的所有端口；不接受 IP 地址授权。
配置只在服务启动时装入，改动后重启；模型设置保存会保留此块。
运行中不提供联网提权、任意写目录或关闭沙箱的工具参数。

默认只读项目副本、必要系统/工具链路径及额外 `read_roots`。
真实项目、应用配置/数据和常见凭据目录显式拒读；允许更大的读取目录也不能重新开放这些凭据。
HOME、TMPDIR、包缓存独立且随命令清理；不继承密钥、SSH agent、宿主代理、Shell 初始化变量或 Node 注入变量。
SRT 自带的共享临时写目录通过 `denyWrite` 禁用。

只有 Create 的执行阶段可以写副本，Chat/Ask/Plan 从启动时即只读；独立 scratch 始终可写。
`.git`、`.runtime`、`.run`、`.commit-tmp`、`versions` 禁止写入。
默认不联网；授权域名也不能访问 loopback、宿主地址、私网、link-local 或 metadata 地址。
这些检查由 SRT 执行；不启用外部代理、额外 Unix socket、本地监听、Apple Events 或 weaker isolation。

## 事务和边界

命令在审批前只执行一次，写入形成候选 diff；审批通过后提交该候选，不重新运行命令。
审批绑定源码、实际改动、项目基线和沙箱策略指纹；更改权限/执行限制或升级 worker/锁文件后，重启恢复时丢弃旧候选。
硬链接、扩展属性及特殊文件不属于项目事务格式，发现后拒绝整个候选，不静默丢弃元数据。
stdout/stderr 合计超过上限即终止并返回 `COMMAND_OUTPUT_LIMIT`，保留限额内输出，不保存候选变更。

Go 为 worker 建立独立进程组；worker 收到父进程管道关闭或取消信号后结束执行，退出前关闭 SRT 代理并强制结束整个组。
命令正常结束也清理后台组成员；后台任务持有输出管道不关闭时整条命令失败。
**进程组不是容器：主动 `setsid`/双重 fork 脱离进程组的后代，不能仅靠 SRT 保证回收。**
此类进程仍继承 OS 沙箱；本接入没有新增自研内核规则来禁止它们，不提供完整进程数、磁盘或内存配额。
不得把它当作多租户服务的容器/VM 隔离；若该边界成为产品要求，应另选具备进程命名空间的部署环境。
放行域名后的外部操作发生在命令执行时，项目审批不能撤销网络副作用。

## 手动验收

按项目约定，本次未运行测试、构建或应用验证。建议依次手动检查：

1. 在 `backend/` 运行 `go test ./internal/config ./internal/commandexec ./internal/workflow ./internal/llm`。
2. Chat/Plan 读取副本成功，重定向/创建文件失败；Create 修改展示真实 diff，拒绝不落盘，批准只提交一次。
3. 读取项目外文件、用户凭据、真实项目及应用配置被拒；配置一个额外目录并重启，只开放该目录。目录名含空格、方括号和符号链接也要覆盖。
4. 默认外网和 `127.0.0.1:8787`、`::1`、宿主网卡地址、Unix socket 均不可达；放行某域名后验证对应端口、未授权域名、重定向和 DNS 解析到私网的拒绝行为。
5. 共享 `/tmp/claude` 和宿主缓存不可写；检查 `.git` 等保护路径、外链、硬链接、xattr、特殊文件候选拒绝。
6. 超时、取消、输出洪泛、正常退出后的后台进程、worker/backend 异常退出：无项目写入残留，代理/普通进程组结束；另测主动脱离进程组的已知限制。
7. 挂起等待审批时修改 sandbox 并重启，旧候选失败且源码未重跑；不改策略的恢复只提交原候选。
8. 分别在 Apple Silicon/Intel 机器打包 App，脱离源码目录运行，检查依赖、权限错误和进程退出。
