#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 0 ]; then
  printf '用法：./scripts/tokenize.sh\n' >&2
  exit 1
fi
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TOKENIZE_VENV="$ROOT_DIR/tmp/token-count/venv"
if [ ! -x "$TOKENIZE_VENV/bin/python" ]; then
  printf '准备本地 tokenizer 环境…\n' >&2
  python3 -m venv "$TOKENIZE_VENV" >&2
fi
exec "$TOKENIZE_VENV/bin/python" - "$ROOT_DIR" <<'TOKENIZE_PY'
"""Count the current execute prompt and initial tools with local tokenizers.

OpenAI uses o200k_base; DeepSeek uses the official V4 vocabulary. Counts cover
rendered system text and each provider's tool JSON, excluding request framing,
runtime context and conversation history. total is the sum of the four rows.
The first run downloads dependencies/vocabularies into tmp/token-count; later
runs use those cached files without any model API or prompt upload.
"""

import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.request
import zipfile


ROOT = Path(sys.argv[1]).resolve()
CACHE = ROOT / "tmp" / "token-count"
VENV = CACHE / "venv"
REQUIREMENTS = ("tiktoken==0.14.0", "tokenizers==0.23.3")
DEEPSEEK_URL = "https://cdn.deepseek.com/api-docs/deepseek_v4_tokenizer.zip"
DEEPSEEK_SHA256 = "89085f12ef79460ac5f66d1119325ddfc694b4ab209d80bbd81d35f081dc9614"

# Go overlays expose existing assembly functions only to this one command. The
# real server entry point is replaced in memory; no backend file is modified.
WORKFLOW_EXPORT = r'''
type tokenCountCatalog struct{}
type tokenCountSkills struct{}

func (tokenCountCatalog) Get(string) (model.Component, error) {
    panic("token counting does not execute tools")
}

func (tokenCountSkills) Get(string) (model.RepositorySkill, error) {
    panic("token counting does not execute tools")
}

func ExportTokenCountInputs() (string, []ToolSchema, error) {
    pack := contextengine.ContextPack{}
    pack.Command.Mode = model.ModeExecute
    provider := DefaultDomainToolProvider{
        Pack: pack, Components: tokenCountCatalog{}, Skills: tokenCountSkills{},
        GitCommit: func(context.Context, string, map[string]any) (map[string]any, error) {
            panic("token counting does not execute tools")
        },
    }
    registry, err := buildDomainToolRegistry(RuntimeInput{DomainTools: provider}, pack)
    if err != nil { return "", nil, err }
    state := &RunState{
        mode: model.ModeExecute, phase: PhaseExecuting,
        scope: pack.Command.Scope, tools: registry,
    }
    return runtimeSystemPromptForRequest(AgentRequest{Mode: model.ModeExecute, Context: pack}), windowTools(state), nil
}
'''

LLM_EXPORT = r'''
func ExportTokenCountWireTools(tools []ToolSchema) (any, any) {
    anthropic := make([]anthropicTool, 0, len(tools))
    for _, tool := range tools {
        anthropic = append(anthropic, anthropicTool{
            Name: tool.Name, Description: tool.Description,
            InputSchema: toolParameters(tool.Parameters),
        })
    }
    return responsesTools(tools), anthropic
}
'''

GO_MAIN = r'''package main

import (
    "encoding/json"
    "fmt"
    "os"
    "github.com/dasi0227/PPT-Agent/backend/internal/llm"
    "github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func run() error {
    system, tools, err := workflow.ExportTokenCountInputs()
    if err != nil { return err }
    openai, deepseek := llm.ExportTokenCountWireTools(tools)
    serialized := map[string]string{}
    for name, value := range map[string]any{"openai": openai, "deepseek": deepseek} {
        raw, err := json.Marshal(value)
        if err != nil { return err }
        serialized[name] = string(raw)
    }
    return json.NewEncoder(os.Stdout).Encode(map[string]any{"system": system, "tools": serialized})
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
'''


def bootstrap():
    CACHE.mkdir(parents=True, exist_ok=True)
    with (CACHE / "setup.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        marker = VENV / "tokenizer-requirements.txt"
        expected = "\n".join(REQUIREMENTS)
        if not marker.is_file() or marker.read_text() != expected:
            subprocess.run(
                [sys.executable, "-m", "pip", "install", "--disable-pip-version-check",
                 "--no-cache-dir", "--only-binary=:all:", *REQUIREMENTS],
                check=True, stdout=sys.stderr,
            )
            marker.write_text(expected)


def snapshot():
    if shutil.which("go") is None:
        raise RuntimeError("缺少 Go；请先安装 backend/go.mod 要求的 Go 版本")
    with tempfile.TemporaryDirectory(prefix="request-", dir=CACHE) as directory:
        work = Path(directory)
        overlays = {}
        for relative, extra in (
            ("internal/workflow/context_window.go", WORKFLOW_EXPORT),
            ("internal/llm/anthropic.go", LLM_EXPORT),
        ):
            original = ROOT / "backend" / relative
            replacement = work / original.name
            replacement.write_text(original.read_text() + "\n" + extra)
            overlays[str(original)] = str(replacement)
        main = work / "main.go"
        main.write_text(GO_MAIN)
        overlays[str(ROOT / "backend/cmd/server/main.go")] = str(main)
        overlay = work / "overlay.json"
        overlay.write_text(json.dumps({"Replace": overlays}))
        result = subprocess.run(
            ["go", "run", "-mod=readonly", "-overlay", str(overlay), "./cmd/server/main.go"],
            cwd=ROOT / "backend", check=True, stdout=subprocess.PIPE, text=True,
        )
        return json.loads(result.stdout)


def load_tokenizers():
    import tiktoken
    from tokenizers import Tokenizer

    # Both vocabularies are cached under the ignored project tmp/ directory.
    os.environ["TIKTOKEN_CACHE_DIR"] = str(CACHE / "openai")
    with (CACHE / "vocabularies.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        openai = tiktoken.get_encoding("o200k_base")
        path = CACHE / "deepseek-v4-tokenizer.json"
        if not path.is_file():
            print("下载 DeepSeek 官方离线词表…", file=sys.stderr)
            with tempfile.TemporaryFile() as archive:
                with urllib.request.urlopen(DEEPSEEK_URL, timeout=60) as response:
                    shutil.copyfileobj(response, archive)
                archive.seek(0)
                with zipfile.ZipFile(archive) as files:
                    raw = files.read("deepseek_v4_tokenizer/tokenizer.json")
            if hashlib.sha256(raw).hexdigest() != DEEPSEEK_SHA256:
                raise RuntimeError("DeepSeek 官方词表版本已变化；请更新脚本中的词表校验值")
            temporary = path.with_suffix(".part")
            temporary.write_bytes(raw)
            temporary.replace(path)
        raw = path.read_bytes()
        if hashlib.sha256(raw).hexdigest() != DEEPSEEK_SHA256:
            raise RuntimeError("DeepSeek 缓存词表校验失败；请删除 tmp/token-count 中的词表后重试")
        deepseek = Tokenizer.from_str(raw.decode())
    return (
        lambda text: len(openai.encode(text, disallowed_special=())),
        lambda text: len(deepseek.encode(text, add_special_tokens=False).ids),
    )


def group_prompts(system):
    groups = {name: [] for name in ("core", "mode", "playbook")}
    pattern = re.compile(r'<system_prompt id="([^"]+)"[^>]*>\n.*?\n</system_prompt>', re.S)
    modules = list(pattern.finditer(system))
    if not modules or "\n".join(match.group(0) for match in modules) != system:
        raise RuntimeError("后端系统提示词格式已变化，无法完整分组")
    for match in modules:
        group = match.group(1).split(".", 1)[0]
        if group not in groups:
            raise RuntimeError(f"出现未归类的提示词模块：{match.group(1)}")
        groups[group].append(match.group(0))
    if any(not values for values in groups.values()):
        raise RuntimeError("execute 提示词缺少 core、mode 或 playbook 分组")
    return {name: "\n".join(values) for name, values in groups.items()}


def main():
    if len(sys.argv) != 2:
        raise RuntimeError("用法：./scripts/tokenize.sh")
    bootstrap()
    request = snapshot()
    groups = group_prompts(request["system"])
    counters = load_tokenizers()
    rows = []
    for name in ("core", "mode", "playbook"):
        rows.append((name, *(count(groups[name]) for count in counters)))
    rows.append(("tool", counters[0](request["tools"]["openai"]),
                 counters[1](request["tools"]["deepseek"])))
    rows.append(("total", sum(row[1] for row in rows), sum(row[2] for row in rows)))
    width = max(8, *(len(str(value)) for row in rows for value in row[1:]))
    print(f"{'':8} {'openai':>{width}} {'deepseek':>{width}}")
    for name, openai, deepseek in rows:
        print(f"{name:8} {openai:>{width}} {deepseek:>{width}}")


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, ValueError, zipfile.BadZipFile, subprocess.CalledProcessError) as error:
        print(f"Token 统计失败：{error}", file=sys.stderr)
        sys.exit(1)

TOKENIZE_PY
