#!/usr/bin/env python3
"""Explicit maintenance command; stop the server before replacing presets."""
import argparse
from pathlib import Path
import subprocess

parser = argparse.ArgumentParser(description="显式替换预置主题和组件的正文、名称、描述与标签；请先停止后端。")
parser.add_argument("--work-root", type=Path, default=Path.home() / ".dasi/ppt")
parser.add_argument("--components-only", action="store_true", help="仅替换预置组件")
args = parser.parse_args()
project = Path(__file__).resolve().parents[1]
command = [
    "go", "run", "./cmd/init-resources", "--work-root", str(args.work_root.expanduser().resolve()),
    "--seed-root", str(project / "seed"), "--replace-presets",
]
if args.components_only:
    command.append("--components-only")
subprocess.run(command, cwd=project / "backend", check=True)
