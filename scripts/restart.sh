#!/usr/bin/env bash
# restart.sh - 重启 Web 或 macOS App；每次重置预置资源，默认保留项目数据并启动 Web。
# 用法:./scripts/restart.sh --reset 0 --mode web
#       ./scripts/restart.sh --reset 1 --mode app
set -euo pipefail

RESET_MODE="0"
RUN_MODE="web"
usage() {
  printf '用法:./scripts/restart.sh [--reset 0|1] [--mode app|web]\n默认:--reset 0 --mode web\n'
}
while [ "$#" -gt 0 ]; do
  case "$1" in
    --reset|--mode)
      if [ "$#" -lt 2 ]; then
        printf '%s 需要一个参数值\n' "$1" >&2
        exit 1
      fi
      case "$1:$2" in
        --reset:0|--reset:1) RESET_MODE="$2" ;;
        --mode:app|--mode:web) RUN_MODE="$2" ;;
        *) printf '无效参数:%s %s\n' "$1" "$2" >&2; usage >&2; exit 1 ;;
      esac
      shift 2
      ;;
    --help|-h) usage; exit 0 ;;
    *) printf '未知参数:%s\n' "$1" >&2; usage >&2; exit 1 ;;
  esac
done

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
RENDERER_DIR="$BACKEND_DIR/render-worker"
COMMAND_WORKER_DIR="$BACKEND_DIR/command-worker"
WORK_ROOT="$HOME/.dasi/ppt"
BACKEND_CONFIG="$BACKEND_DIR/config.yaml"
if [ "$RUN_MODE" = "app" ] && [ -f "$HOME/Library/Application Support/PPT-Agent/config.yaml" ]; then
  BACKEND_CONFIG="$HOME/Library/Application Support/PPT-Agent/config.yaml"
fi
BACKEND_ADDR="$(cd "$BACKEND_DIR" && go run ./cmd/server --print-address "$BACKEND_CONFIG")"
BACKEND_PORT="${BACKEND_ADDR##*:}"
FRONTEND_URL="http://localhost:5173"
LOG_DIR="$ROOT_DIR/tmp/run"
mkdir -p "$LOG_DIR"

STEP_1="[1/5]关闭进程"
STEP_2="[2/5]准备数据"
STEP_3="[3/5]启动后端"
STEP_4="[4/5]启动前端"
STEP_5="[5/5]打开应用"
if [ "$RUN_MODE" = "app" ]; then
  STEP_1="[1/4]关闭进程"
  STEP_2="[2/4]准备数据"
fi

print_result() {
  local label="$1"
  local icon="$2"
  local result="${3:-}"

  printf '%s %s' "$label" "$icon"
  if [ -n "$result" ]; then printf ' %s' "$result"; fi
  printf '\n'
}

wait_for_url() {
  local url="$1"
  local pid="${2:-}"
  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    if [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then return 1; fi
    curl -sf --connect-timeout 1 --max-time 2 "$url" >/dev/null 2>&1 && return 0
    sleep 0.1
  done
  return 1
}

initialize_project() {
  if [ "$RESET_MODE" = "1" ] && ! rm -rf "$WORK_ROOT" 2>"$LOG_DIR/init-resources.log"; then
    print_result "$STEP_2" '❌' '初始化失败:无法清空项目数据（日志:tmp/run/init-resources.log）'
    return 1
  fi
  if ! (
    mkdir -p "$WORK_ROOT" || exit 1
    touch "$WORK_ROOT/.desktop-initializing" || exit 1
    cd "$BACKEND_DIR" || exit 1
    go run ./cmd/init-resources --work-root "$WORK_ROOT" --seed-root "$BACKEND_DIR/seed" || exit 1
    rm -f "$WORK_ROOT/.desktop-initializing"
  ) >"$LOG_DIR/init-resources.log" 2>&1; then
    print_result "$STEP_2" '❌' '初始化失败（日志:tmp/run/init-resources.log）'
    return 1
  fi
  print_result "$STEP_2" '✅' "reset=$RESET_MODE"
}

# Only invoked as a background job. The parent owns all ordered stdout output.
start_frontend() {
  trap - EXIT INT TERM
  frontend_child_pid=""
  frontend_ready=0
  frontend_interrupted=0
  trap 'frontend_interrupted=1; if [ -n "$frontend_child_pid" ]; then kill -TERM "$frontend_child_pid" 2>/dev/null || true; fi' INT TERM
  trap 'if [ "$frontend_ready" != 1 ] && [ -n "$frontend_child_pid" ]; then kill -TERM "$frontend_child_pid" 2>/dev/null || true; wait "$frontend_child_pid" 2>/dev/null || true; fi' EXIT

  cd "$FRONTEND_DIR" || exit 1
  if [ ! -f node_modules/vite/bin/vite.js ]; then
    pnpm install &
    frontend_child_pid=$!
    [ "$frontend_interrupted" = 0 ] || exit 1
    wait "$frontend_child_pid" || exit 1
    frontend_child_pid=""
  fi
  [ "$frontend_interrupted" = 0 ] || exit 1
  # Start Vite directly so the PID file owns the server, not a pnpm wrapper.
  nohup node "$FRONTEND_DIR/node_modules/vite/bin/vite.js" --port 5173 --strictPort >"$LOG_DIR/frontend.log" 2>&1 &
  frontend_child_pid=$!
  printf '%s\n' "$frontend_child_pid" >"$LOG_DIR/frontend.pid"
  [ "$frontend_interrupted" = 0 ] || exit 1
  wait_for_url "$FRONTEND_URL" "$frontend_child_pid" || exit 1
  [ "$frontend_interrupted" = 0 ] || exit 1
  frontend_ready=1
}

start_backend() {
  if ! (
    cd "$RENDERER_DIR" || exit 1
    [ -f node_modules/playwright-core/package.json ] || pnpm install --frozen-lockfile --ignore-scripts || exit 1
    cd "$COMMAND_WORKER_DIR" || exit 1
    npm ci --omit=dev --ignore-scripts --no-audit --no-fund || exit 1
  ) >"$LOG_DIR/backend-dependencies.log" 2>&1; then
    print_result "$STEP_3" '❌' '启动失败:依赖准备失败（日志:tmp/run/backend-dependencies.log）'
    return 1
  fi
  if ! (
    cd "$BACKEND_DIR" || exit 1
    PPT_COMMAND_WORKER="$COMMAND_WORKER_DIR/worker.mjs" nohup go run ./cmd/server >"$LOG_DIR/backend.log" 2>&1 &
    printf '%s\n' "$!" >"$LOG_DIR/backend.pid"
  ); then
    print_result "$STEP_3" '❌' '启动失败（日志:tmp/run/backend.log）'
    return 1
  fi
  # Backend readiness already includes its persistent render worker's health.
  if wait_for_url "http://$BACKEND_ADDR/api/v1/healthz" "$(<"$LOG_DIR/backend.pid")"; then
    print_result "$STEP_3" '✅' "port=$BACKEND_PORT pid=$(<"$LOG_DIR/backend.pid")"
  else
    print_result "$STEP_3" '❌' '启动失败:进程退出或健康检查超时（日志:tmp/run/backend.log）'
    return 1
  fi
}

recorded_process_pid() {
  local file="$1"
  local directory="$2"
  local pid cwd
  [ -f "$file" ] || return 0
  pid="$(<"$file")"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 0
  cwd="$(lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' || true)"
  if [ "$cwd" = "$directory" ]; then
    printf '%s\n' "$pid"
  fi
}

close_running_processes() {
  local pid command cwd
  local pids="" remaining=""
  local deadline
  # Electron 的 SIGTERM 处理会暂停任务并关闭 Go 与渲染子进程。
  for pid in $(ps -axo pid=,command= | awk '
    index($0, "/PPT-Agent.app/Contents/MacOS/PPT-Agent") && $0 !~ /--serve|--type=/ { print $1 }
  '); do
    pids="$pids $pid"
  done
  if [ -f "$WORK_ROOT/.server.lock" ]; then
    for pid in $(lsof -t "$WORK_ROOT/.server.lock" 2>/dev/null || true); do
      command="$(ps -p "$pid" -o command= || true)"
      case "$command" in
        */ppt-agent-server*|*init-resources*) pids="$pids $pid" ;;
        */server|*/server\ *)
          # go run 可直接执行缓存中的 server，不能依赖 /exe/server 路径。
          cwd="$(lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' || true)"
          if [ "$cwd" = "$BACKEND_DIR" ]; then
            pids="$pids $pid"
          else
            printf '工作目录被其他进程 %s 使用\n' "$pid" | tee -a "$LOG_DIR/restart.log" >&2
            return 1
          fi
          ;;
        *)
          printf '工作目录被其他进程 %s 使用\n' "$pid" | tee -a "$LOG_DIR/restart.log" >&2
          return 1
          ;;
      esac
    done
  fi
  for pid in $(lsof -nP -tiTCP:5173 -sTCP:LISTEN 2>/dev/null || true); do
    command="$(ps -p "$pid" -o command= || true)"
    case "$command" in
      *"$FRONTEND_DIR/"*vite*) pids="$pids $pid" ;;
    esac
  done
  for pid in $(recorded_process_pid "$LOG_DIR/backend.pid" "$BACKEND_DIR") \
    $(recorded_process_pid "$LOG_DIR/frontend.pid" "$FRONTEND_DIR"); do
    pids="$pids $pid"
  done
  if [ -n "$pids" ]; then
    # Send one signal per PID before waiting. Go owns task persistence and its
    # child workers; stopping Vite concurrently releases its proxy connections.
    # All entries are numeric PIDs; whitespace splitting also works in macOS Bash 3.
    pids="$(printf '%s\n' $pids | sort -nu)"
    deadline=$((SECONDS + 12))
    for pid in $pids; do
      if kill -0 "$pid" 2>/dev/null; then
        kill -TERM "$pid" 2>>"$LOG_DIR/restart.log" || {
          kill -0 "$pid" 2>/dev/null && return 1
        }
      fi
    done
    while [ -n "$pids" ]; do
      remaining=""
      for pid in $pids; do
        # A reaped child can briefly remain as a zombie. It owns no resources
        # and must not consume the entire shutdown budget.
        if kill -0 "$pid" 2>/dev/null; then
          case "$(ps -p "$pid" -o stat= 2>/dev/null || true)" in
            ''|*Z*) ;;
            *) remaining="$remaining $pid" ;;
          esac
        fi
      done
      [ -z "$remaining" ] && break
      if (( SECONDS >= deadline )); then
        printf '进程未在正常退出期限内结束:%s\n' "$remaining" | tee -a "$LOG_DIR/restart.log" >&2
        return 1
      fi
      pids="$remaining"
      sleep 0.1
    done
  fi
  rm -f "$LOG_DIR/backend.pid" "$LOG_DIR/frontend.pid"
}

ensure_port_free() {
  local port="$1"
  local pids
  pids="$(lsof -nP -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    printf '端口 %s 仍被其他进程占用:%s\n' "$port" "$pids" >&2
    return 1
  fi
}

command -v lsof >/dev/null 2>&1 || { printf '缺少 lsof\n' >&2; exit 1; }
if close_running_processes; then
  print_result "$STEP_1" '✅'
else
  print_result "$STEP_1" '❌' '关闭失败（日志:tmp/run/restart.log）'
  exit 1
fi

ensure_port_free "$BACKEND_PORT"
if [ "$RUN_MODE" = "web" ]; then ensure_port_free 5173; fi

if [ "$RUN_MODE" = "app" ]; then
  initialize_project || exit 1
  case "$(uname -m)" in
    arm64) app_arch="arm64" ;;
    x86_64) app_arch="x64" ;;
    *) printf '不支持的 Mac 架构\n' >&2; exit 1 ;;
  esac
  if (
    cd "$FRONTEND_DIR" || exit 1
    [ -d node_modules ] || pnpm install || exit 1
    cd "$RENDERER_DIR" || exit 1
    [ -d node_modules ] || pnpm install --frozen-lockfile --ignore-scripts || exit 1
    cd "$ROOT_DIR/desktop" || exit 1
    [ -d node_modules ] || npm ci || exit 1
    npm run prepare:local || exit 1
    npm run package || exit 1
  ) >"$LOG_DIR/desktop-build.log" 2>&1; then
    print_result '[3/4]构建 macOS App' '✅'
  else
    print_result '[3/4]构建 macOS App' '❌' '构建失败（日志:tmp/run/desktop-build.log）'
    exit 1
  fi
  APP_PATH="$ROOT_DIR/desktop/out/PPT-Agent-darwin-$app_arch/PPT-Agent.app"
  if open "$APP_PATH" && wait_for_url "http://$BACKEND_ADDR/api/v1/healthz"; then
    print_result '[4/4]打开应用' '✅' "mode=$RUN_MODE"
  else
    print_result '[4/4]打开应用' '❌' '启动失败（日志:~/Library/Logs/PPT-Agent/backend.log）'
    exit 1
  fi
  exit 0
fi

frontend_job=""
startup_ready=0
cleanup_web_startup() {
  local result=$?
  trap - EXIT INT TERM
  if [ "$startup_ready" != 1 ]; then
    # Wait for the launcher to finish handling cancellation before scanning PID
    # files, so it cannot spawn another server after cleanup has already run.
    # A signal may arrive just after the background fork, before assignment.
    frontend_job="${frontend_job:-${!:-}}"
    if [ -n "$frontend_job" ]; then
      kill -TERM "$frontend_job" 2>/dev/null || true
      wait "$frontend_job" 2>/dev/null || true
    fi
    close_running_processes >>"$LOG_DIR/restart.log" 2>&1 || true
  fi
  exit "$result"
}
trap cleanup_web_startup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

start_frontend >"$LOG_DIR/frontend-startup.log" 2>&1 &
frontend_job=$!
initialize_project || exit 1
start_backend || exit 1
if wait "$frontend_job" && wait_for_url "$FRONTEND_URL" "$(<"$LOG_DIR/frontend.pid")"; then
  print_result "$STEP_4" '✅' "port=5173 pid=$(<"$LOG_DIR/frontend.pid")"
else
  print_result "$STEP_4" '❌' '启动失败（日志:tmp/run/frontend-startup.log、tmp/run/frontend.log）'
  exit 1
fi
frontend_job=""
startup_ready=1

if command -v open >/dev/null 2>&1 && open "$FRONTEND_URL"; then
  print_result "$STEP_5" '✅' "mode=$RUN_MODE"
else
  print_result "$STEP_5" '❌' '打开失败:无法调用 open 命令'
  exit 1
fi
