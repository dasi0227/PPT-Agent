#!/usr/bin/env bash
# restart.sh - 重启 Web 或 macOS App；默认保留数据并启动 Web。
# 用法：./restart.sh --reset 0 --mode web
#       ./restart.sh --reset 1 --mode app
set -euo pipefail

RESET_MODE="0"
RUN_MODE="web"
usage() {
  printf '用法：./restart.sh [--reset 0|1] [--mode app|web]\n默认：--reset 0 --mode web\n'
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
        *) printf '无效参数：%s %s\n' "$1" "$2" >&2; usage >&2; exit 1 ;;
      esac
      shift 2
      ;;
    --help|-h) usage; exit 0 ;;
    *) printf '未知参数：%s\n' "$1" >&2; usage >&2; exit 1 ;;
  esac
done

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
RENDERER_DIR="$BACKEND_DIR/render-worker"
WORK_ROOT="$HOME/.dasi/ppt"
BACKEND_PORT="8787"
if [ "$RUN_MODE" = "web" ] && [ -f "$BACKEND_DIR/.env" ]; then
  configured_backend_port="$(sed -n 's/^[[:space:]]*PORT[[:space:]]*=[[:space:]]*//p' "$BACKEND_DIR/.env" | tail -n 1 | tr -d '[:space:]')"
  BACKEND_PORT="${configured_backend_port:-$BACKEND_PORT}"
fi
if [[ ! "$BACKEND_PORT" =~ ^[0-9]+$ ]] || (( BACKEND_PORT < 1 || BACKEND_PORT > 65535 )); then
  printf 'backend/.env 中的 PORT 必须是 1 到 65535 之间的整数\n' >&2
  exit 1
fi
BACKEND_ADDR="127.0.0.1:$BACKEND_PORT"
FRONTEND_URL="http://localhost:5173"
LOG_DIR="$ROOT_DIR/.run"
mkdir -p "$LOG_DIR"

RESULT_COLUMN=36
STEP_1="【1/6】关闭可能在运行的前后端进程"
STEP_2="【2/6】准备项目数据 reset=$RESET_MODE"
STEP_3="【3/6】启动 Chromium 渲染"
STEP_4="【4/6】启动后端 port=$BACKEND_PORT pid=xxxx"
STEP_5="【5/6】启动前端 port=5173 pid=xxxx"
STEP_6="【6/6】打开浏览器"
if [ "$RUN_MODE" = "app" ]; then
  STEP_1="【1/4】关闭正在运行的 App 与 Web 服务"
  STEP_2="【2/4】准备项目数据 reset=$RESET_MODE"
fi

display_width() {
  local text="$1"
  local width=0
  local index
  local character

  for ((index = 0; index < ${#text}; index++)); do
    character="${text:index:1}"
    case "$character" in
      [A-Za-z0-9\ /\[\]=:_-]) width=$((width + 1)) ;;
      *) width=$((width + 2)) ;;
    esac
  done
  printf '%s' "$width"
}

print_result_tail() {
  local label="$1"
  local result="$2"
  local width
  local padding

  width="$(display_width "$label")"
  padding=$((RESULT_COLUMN - width))
  [ "$padding" -lt 0 ] && padding=0
  printf '%*s→ %s\n' "$padding" "" "$result"
}

print_result() {
  local label="$1"
  local result="$2"

  printf '%s' "$label"
  print_result_tail "$label" "$result"
}

wait_for_url() {
  local url="$1"
  for _ in $(seq 1 60); do
    curl -sf "$url" >/dev/null 2>&1 && return 0
    sleep 1
  done
  return 1
}

stop_process() {
  local pid="$1"
  kill -0 "$pid" 2>/dev/null || return 0
  kill -TERM "$pid" 2>>"$LOG_DIR/restart.log" || return 1
  local attempt
  for ((attempt = 0; attempt < 35; attempt++)); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 1
  done
  printf '进程 %s 未在正常退出期限内结束\n' "$pid" >&2
  return 1
}

stop_recorded_process() {
  local file="$1"
  local directory="$2"
  local pid cwd
  [ -f "$file" ] || return 0
  pid="$(<"$file")"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 0
  cwd="$(lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' || true)"
  if [ "$cwd" = "$directory" ]; then
    stop_process "$pid" || return 1
  fi
  rm -f "$file"
}

close_running_processes() {
  local pid command
  # Electron 的 SIGTERM 处理会暂停任务并关闭 Go 与渲染子进程。
  for pid in $(ps -axo pid=,command= | awk '
    index($0, "/PPT-Agent.app/Contents/MacOS/PPT-Agent") && $0 !~ /--serve|--type=/ { print $1 }
  '); do
    stop_process "$pid" || return 1
  done
  if [ -f "$WORK_ROOT/.server.lock" ]; then
    for pid in $(lsof -t "$WORK_ROOT/.server.lock" 2>/dev/null || true); do
      command="$(ps -p "$pid" -o command= || true)"
      case "$command" in
        */ppt-agent-server*|*/exe/server*|*init-resources*) stop_process "$pid" || return 1 ;;
        *) printf '工作目录被其他进程 %s 使用\n' "$pid" >&2; return 1 ;;
      esac
    done
  fi
  for pid in $(lsof -nP -tiTCP:5173 -sTCP:LISTEN 2>/dev/null || true); do
    command="$(ps -p "$pid" -o command= || true)"
    case "$command" in
      *"$FRONTEND_DIR/"*vite*) stop_process "$pid" || return 1 ;;
    esac
  done
  stop_recorded_process "$LOG_DIR/backend.pid" "$BACKEND_DIR" || return 1
  stop_recorded_process "$LOG_DIR/frontend.pid" "$FRONTEND_DIR" || return 1
}

ensure_port_free() {
  local port="$1"
  local pids
  pids="$(lsof -nP -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    printf '端口 %s 仍被其他进程占用：%s\n' "$port" "$pids" >&2
    return 1
  fi
}

command -v lsof >/dev/null 2>&1 || { printf '缺少 lsof\n' >&2; exit 1; }
if close_running_processes; then
  print_result "$STEP_1" "关闭成功 ✅"
else
  print_result "$STEP_1" "关闭失败 ❌（日志：$LOG_DIR/restart.log）"
  exit 1
fi

ensure_port_free "$BACKEND_PORT"
if [ "$RUN_MODE" = "web" ]; then ensure_port_free 5173; fi

if [ "$RESET_MODE" = "1" ] && ! rm -rf "$WORK_ROOT"; then
  print_result "$STEP_2" "初始化失败 ❌：无法清空 $WORK_ROOT"
  exit 1
fi
if [ "$RUN_MODE" = "web" ] && { [ ! -f "$WORK_ROOT/db/ppt.db" ] || [ -f "$WORK_ROOT/.desktop-initializing" ]; }; then
  mkdir -p "$WORK_ROOT"
  touch "$WORK_ROOT/.desktop-initializing"
  if ! "$ROOT_DIR/scripts/init-workroot.sh" "$WORK_ROOT"; then
    print_result "$STEP_2" "初始化失败 ❌：无法初始化 $WORK_ROOT"
    exit 1
  fi
  rm -f "$WORK_ROOT/.desktop-initializing"
fi
if [ "$RESET_MODE" = "1" ]; then
  print_result "$STEP_2" "数据已重置 ✅"
else
  print_result "$STEP_2" "保留现有数据 ✅"
fi

if [ "$RUN_MODE" = "app" ]; then
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
    print_result '【3/4】构建 macOS App' '构建成功 ✅'
  else
    print_result '【3/4】构建 macOS App' "构建失败 ❌（日志：$LOG_DIR/desktop-build.log）"
    exit 1
  fi
  APP_PATH="$ROOT_DIR/desktop/out/PPT-Agent-darwin-$app_arch/PPT-Agent.app"
  if open "$APP_PATH" && wait_for_url "http://$BACKEND_ADDR/api/v1/healthz"; then
    print_result '【4/4】打开 App' '启动成功 ✅'
  else
    print_result '【4/4】打开 App' "启动失败 ❌（日志：$HOME/Library/Logs/PPT-Agent/backend.log）"
    exit 1
  fi
  exit 0
fi

if (
  cd "$RENDERER_DIR" || exit 1
  [ -d node_modules ] || pnpm install --frozen-lockfile --ignore-scripts || exit 1
  pnpm health
) >"$LOG_DIR/renderer.log" 2>&1; then
  print_result "$STEP_3" "启动成功 ✅"
else
  print_result "$STEP_3" "启动失败 ❌：Chromium 健康检查未通过（日志：$LOG_DIR/renderer.log）"
  exit 1
fi

if (
  cd "$BACKEND_DIR" || exit 1
  PORT="$BACKEND_PORT" nohup go run ./cmd/server >"$LOG_DIR/backend.log" 2>&1 &
  echo $! >"$LOG_DIR/backend.pid"
); then
  backend_pid="$(<"$LOG_DIR/backend.pid")"
  printf -v backend_step '【4/6】启动后端 port=%s pid=%-5s' "$BACKEND_PORT" "$backend_pid"
else
  print_result "$STEP_4" "启动失败 ❌：无法启动后端进程（日志：$LOG_DIR/backend.log）"
  exit 1
fi
if wait_for_url "http://$BACKEND_ADDR/api/v1/healthz"; then
  print_result "$backend_step" "启动成功 ✅"
else
  print_result "$backend_step" "启动失败 ❌：健康检查超时（日志：$LOG_DIR/backend.log）"
  exit 1
fi

if (
  cd "$FRONTEND_DIR" || exit 1
  [ -d node_modules ] || pnpm install || exit 1
  VITE_API_PROXY_TARGET="http://$BACKEND_ADDR" nohup pnpm dev --port 5173 --strictPort >"$LOG_DIR/frontend.log" 2>&1 &
  echo $! >"$LOG_DIR/frontend.pid"
); then
  frontend_pid="$(<"$LOG_DIR/frontend.pid")"
  printf -v frontend_step '【5/6】启动前端 port=5173 pid=%-5s' "$frontend_pid"
else
  print_result "$STEP_5" "启动失败 ❌：无法启动前端进程（日志：$LOG_DIR/frontend.log）"
  exit 1
fi
if wait_for_url "$FRONTEND_URL"; then
  print_result "$frontend_step" "启动成功 ✅"
else
  print_result "$frontend_step" "启动失败 ❌：页面健康检查超时（日志：$LOG_DIR/frontend.log）"
  exit 1
fi

if command -v open >/dev/null 2>&1 && open "$FRONTEND_URL"; then
  print_result "$STEP_6" "打开成功 ✅"
else
  print_result "$STEP_6" "打开失败 ❌：无法调用 open 命令"
  exit 1
fi
