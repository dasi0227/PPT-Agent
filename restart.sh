#!/usr/bin/env bash
# restart.sh — 清理旧进程、重置数据、启动前后端并打开浏览器。
# 用法：./restart.sh              （交互式：会询问是否清库）
#       ./restart.sh --reset      （强制清空 WORK_ROOT，不询问）
#       ./restart.sh --no-reset   （保留数据，不清库）
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
RENDERER_DIR="$BACKEND_DIR/render-worker"
WORK_ROOT="$HOME/.dasi/ppt"
BACKEND_ADDR="127.0.0.1:8787"
FRONTEND_URL="http://localhost:5173"
LOG_DIR="$ROOT_DIR/.run"
mkdir -p "$LOG_DIR"

RESET_MODE="ask"
for arg in "$@"; do
  case "$arg" in
    --reset)    RESET_MODE="yes" ;;
    --no-reset) RESET_MODE="no" ;;
    *) echo "未知参数: ${arg} （支持 --reset / --no-reset）"; exit 1 ;;
  esac
done

echo "==> 1/6 关闭可能在运行的前后端进程"
# 后端：cmd/server 进程
pkill -f "cmd/server" 2>/dev/null || true
pkill -f "PPT-Agent.*server" 2>/dev/null || true
# 前端：该项目下的 vite dev
pkill -f "$FRONTEND_DIR/node_modules/.bin/vite" 2>/dev/null || true
pkill -f "vite" 2>/dev/null || true
# 兜底：按端口杀（lsof 可能未安装，忽略错误）
if command -v lsof >/dev/null 2>&1; then
  for port in 8787 5173; do
    pids="$(lsof -ti tcp:"$port" 2>/dev/null || true)"
    [ -n "$pids" ] && kill -9 $pids 2>/dev/null || true
  done
fi
sleep 1

echo "==> 2/6 初始化数据目录与数据库 (WORK_ROOT=$WORK_ROOT)"
if [ "$RESET_MODE" = "ask" ]; then
  read -r -p "清空 ${WORK_ROOT} （所有项目/线程/DB 将被删除）？[y/N] " ans
  case "$ans" in [yY]*) RESET_MODE="yes" ;; *) RESET_MODE="no" ;; esac
fi
if [ "$RESET_MODE" = "yes" ]; then
  rm -rf "$WORK_ROOT"
  echo "    已清空并重建 $WORK_ROOT"
else
  echo "    保留现有数据（未清库）"
fi
mkdir -p "$WORK_ROOT/db" "$WORK_ROOT/projects" "$WORK_ROOT/_assets"

echo "==> 3/6 准备隔离 Chromium 渲染 worker"
(
  cd "$RENDERER_DIR"
  [ -d node_modules ] || pnpm install --frozen-lockfile --ignore-scripts
  pnpm health >/dev/null
)
echo "    render worker 健康检查通过"

echo "==> 4/6 启动后端 (go run, $BACKEND_ADDR)"
(
  cd "$BACKEND_DIR"
  nohup go run ./cmd/server >"$LOG_DIR/backend.log" 2>&1 &
  echo $! >"$LOG_DIR/backend.pid"
)
echo "    backend pid=$(cat "$LOG_DIR/backend.pid")，日志：$LOG_DIR/backend.log"

echo "    等待后端就绪 (healthz)…"
for i in $(seq 1 60); do
  if curl -sf "http://$BACKEND_ADDR/api/v1/healthz" >/dev/null 2>&1; then
    echo "    后端已就绪"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "    ✗ 后端 60s 内未就绪，查看 $LOG_DIR/backend.log"; tail -20 "$LOG_DIR/backend.log" || true; exit 1
  fi
  sleep 1
done

echo "==> 5/6 启动前端 (pnpm dev, $FRONTEND_URL)"
(
  cd "$FRONTEND_DIR"
  [ -d node_modules ] || pnpm install
  nohup pnpm dev >"$LOG_DIR/frontend.log" 2>&1 &
  echo $! >"$LOG_DIR/frontend.pid"
)
echo "    frontend pid=$(cat "$LOG_DIR/frontend.pid")，日志：$LOG_DIR/frontend.log"

echo "    等待前端就绪…"
for i in $(seq 1 60); do
  if curl -sf "$FRONTEND_URL" >/dev/null 2>&1; then
    echo "    前端已就绪"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "    ✗ 前端 60s 内未就绪，查看 $LOG_DIR/frontend.log"; tail -20 "$LOG_DIR/frontend.log" || true; exit 1
  fi
  sleep 1
done

echo "==> 6/6 打开浏览器 $FRONTEND_URL"
if command -v open >/dev/null 2>&1; then
  open "$FRONTEND_URL"
fi

echo ""
echo "全部就绪："
echo "  前端: $FRONTEND_URL"
echo "  后端: http://$BACKEND_ADDR"
echo "  日志: $LOG_DIR/backend.log  $LOG_DIR/frontend.log"
echo "  停止: pkill -f cmd/server ; pkill -f vite"
