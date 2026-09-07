#!/usr/bin/env bash
# restart.sh - 清理旧进程、初始化工作目录、启动前后端并打开浏览器。
# 用法：./restart.sh              （交互式：会询问是否重置数据）
#       ./restart.sh --reset      （清空固定工作目录后重新初始化）
#       ./restart.sh --no-reset   （保留数据并补齐缺失的预置资源）
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
RENDERER_DIR="$BACKEND_DIR/render-worker"
WORK_ROOT="$HOME/.dasi/ppt"
BACKEND_PORT="8787"
if [ -f "$BACKEND_DIR/.env" ]; then
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
STEP_2="【2/6】是否重置数据？[y/n]"
STEP_3="【3/6】启动 Chromium 渲染"
STEP_4="【4/6】启动后端 port=$BACKEND_PORT pid=xxxx"
STEP_5="【5/6】启动前端 port=5173 pid=xxxx"
STEP_6="【6/6】打卡浏览器"

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

RESET_MODE="ask"
for arg in "$@"; do
  case "$arg" in
    --reset) RESET_MODE="yes" ;;
    --no-reset) RESET_MODE="no" ;;
    *)
      printf '未知参数：%s（支持 --reset / --no-reset）\n' "$arg" >&2
      exit 1
      ;;
  esac
done

close_running_processes() {
  pkill -f "cmd/server" 2>>"$LOG_DIR/restart.log" || true
  pkill -f "PPT-Agent.*server" 2>>"$LOG_DIR/restart.log" || true
  pkill -f "$FRONTEND_DIR/node_modules/.bin/vite" 2>>"$LOG_DIR/restart.log" || true
  pkill -f "vite" 2>>"$LOG_DIR/restart.log" || true

  if command -v lsof >/dev/null 2>&1; then
    for port in "$BACKEND_PORT" 5173; do
      local pids
      pids="$(lsof -ti tcp:"$port" 2>/dev/null || true)"
      if [ -n "$pids" ] && ! kill -9 $pids 2>>"$LOG_DIR/restart.log"; then
        return 1
      fi
    done
  fi
  sleep 1
}

if close_running_processes; then
  print_result "$STEP_1" "关闭成功 ✅"
else
  print_result "$STEP_1" "关闭失败 ❌：无法终止占用端口的进程（日志：$LOG_DIR/restart.log）"
  exit 1
fi

printf '%s' "$STEP_2"
if [ "$RESET_MODE" = "ask" ]; then
  if ! read -rsn1 ans; then
    print_result_tail "$STEP_2" "初始化失败 ❌：未读取到初始化选项"
    exit 1
  fi
  case "$ans" in
    [yY]*) RESET_MODE="yes" ;;
    *) RESET_MODE="no" ;;
  esac
fi

if [ "$RESET_MODE" = "yes" ] && ! rm -rf "$WORK_ROOT"; then
  print_result_tail "$STEP_2" "初始化失败 ❌：无法清空 $WORK_ROOT"
  exit 1
fi
if ! "$ROOT_DIR/scripts/init-workroot.sh" "$WORK_ROOT"; then
  print_result_tail "$STEP_2" "初始化失败 ❌：无法初始化 $WORK_ROOT"
  exit 1
fi
if [ "$RESET_MODE" = "yes" ]; then
  print_result_tail "$STEP_2" "重置并初始化成功 ✅"
  SEED_DEFAULT_PROMPTS="1"
else
  print_result_tail "$STEP_2" "保留数据并补齐预置资源 ✅"
  SEED_DEFAULT_PROMPTS="0"
fi

if (
  cd "$RENDERER_DIR"
  [ -d node_modules ] || pnpm install --frozen-lockfile --ignore-scripts
  pnpm health
) >"$LOG_DIR/renderer.log" 2>&1; then
  print_result "$STEP_3" "启动成功 ✅"
else
  print_result "$STEP_3" "启动失败 ❌：Chromium 健康检查未通过（日志：$LOG_DIR/renderer.log）"
  exit 1
fi

if (
  cd "$BACKEND_DIR"
  DASI_SEED_DEFAULT_PROMPTS="$SEED_DEFAULT_PROMPTS" \
    nohup go run ./cmd/server >"$LOG_DIR/backend.log" 2>&1 &
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
  cd "$FRONTEND_DIR"
  [ -d node_modules ] || pnpm install
  nohup pnpm dev >"$LOG_DIR/frontend.log" 2>&1 &
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
