#!/usr/bin/env bash
# Run both services against a fresh, disposable database. Ctrl-C stops the pair.
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
api_port=${JAVA_PORT:-18787}
ui_port=${FRONTEND_PORT:-15173}
java_pid=
ui_pid=
cleanup() {
  [ -z "$ui_pid" ] || kill "$ui_pid" 2>/dev/null || true
  [ -z "$java_pid" ] || kill "$java_pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT TERM
(cd "$root/backend_java" && mvn -q -DskipTests package)
cd "$tmp"
sqlite3 -bail data.db < "$root/backend_java/sql/schema.sql"
sqlite3 -bail data.db <<'SQL'
INSERT INTO settings_fileopen DEFAULT VALUES;
INSERT INTO settings_shortcut DEFAULT VALUES;
INSERT INTO settings_behavior DEFAULT VALUES;
INSERT INTO settings_models (id, name, protocol, provider, base_url, model, key)
VALUES ('fixture', 'Fixture', 'responses', 'gpt', 'http://127.0.0.1:1', 'fixture', 'integration-test-only');
INSERT INTO settings_route (main_default, side_default) VALUES ('fixture', 'fixture');
SQL
export PPT_AGENT_ROOT="$tmp" PPT_AGENT_DB="$tmp/data.db"
export PPT_AGENT_JEV_MODEL=test PPT_AGENT_JEV_KEY=integration-test-only
export PPT_AGENT_SANDBOX_ALLOWED_FILEPATHS="$tmp" PPT_AGENT_SANDBOX_ALLOWED_DOMAINS=localhost
if curl -s --max-time 1 "http://127.0.0.1:$api_port/api/settings/shortcuts" >/dev/null; then
  echo "Java test port $api_port already in use" >&2
  exit 1
fi
java -jar "$root/backend_java/ppt-agent-bootstrap/target/ppt-agent.jar" --server.port="$api_port" >java.log 2>&1 &
java_pid=$!
for i in {1..60}; do
  kill -0 "$java_pid" || { tail -40 java.log; exit 1; }
  if curl -fsS "http://127.0.0.1:$api_port/api/settings/shortcuts" >/dev/null 2>&1; then break; fi
  sleep 1
done
curl -fsS "http://127.0.0.1:$api_port/api/settings/shortcuts" >/dev/null
(cd "$root/frontend" && export PPT_JAVA_ORIGIN="http://127.0.0.1:$api_port" && exec node_modules/.bin/vite --host 127.0.0.1 --port "$ui_port" --strictPort) &
ui_pid=$!
if [[ ${1:-} == --verify ]]; then
  for i in {1..45}; do
    kill -0 "$ui_pid" || exit 1
    if curl -fsS "http://127.0.0.1:$ui_port" >/dev/null 2>&1; then break; fi
    sleep 1
  done
  cd "$root"
  export FRONTEND_PORT="$ui_port"
  npm --prefix frontend run build
  python3 docs/test/settings-assets-api.py
  if node -e "require.resolve('playwright')" 2>/dev/null; then
    node docs/test/settings-assets-browser.cjs
  else
    npm exec --yes --package=playwright -- node docs/test/settings-assets-browser.cjs
  fi
else
  wait "$ui_pid"
fi
