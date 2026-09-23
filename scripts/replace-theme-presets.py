#!/usr/bin/env python3
"""Explicit development-time replacement; normal startup never overwrites assets."""
import argparse
from pathlib import Path
import shutil

parser = argparse.ArgumentParser(description="替换三个预置主题和五个预置组件，并移除五个退役主题；不修改项目或附件。")
parser.add_argument("--work-root", type=Path, default=Path.home() / ".dasi/ppt")
args = parser.parse_args()
source = Path(__file__).resolve().parents[1] / "seed/assets"
root = args.work_root.expanduser().resolve()
active = ("editorial-serif", "blueprint", "bold-signal")
retired = ("swiss-modern", "corporate-clean", "warm-pastel", "tokyo-night", "xiaohongshu-white")
components = ("feature-card", "quote-block", "svg-bar", "kv-list", "stat-badge")
# Preflight all destinations before mutating anything. Never follow repository symlinks.
for folder, names in (("themes", active + retired), ("components", components)):
    for name in names:
        target = root / "assets" / folder / name
        for entry in (root / "assets", target.parent, target):
            if entry.is_symlink():
                parser.error(f"拒绝覆盖符号链接：{entry}")
        if name not in retired:
            filename = "theme.css" if folder == "themes" else "index.html"
            if not (source / folder / name / filename).is_file():
                parser.error(f"预置资源不存在：{name}")
            if (target / filename).is_symlink():
                parser.error(f"拒绝覆盖符号链接：{target / filename}")
for folder, names, filename in (("themes", active, "theme.css"), ("components", components, "index.html")):
    for name in names:
        target = root / "assets" / folder / name
        target.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source / folder / name / filename, target / filename)
for name in retired:
    target = root / "assets/themes" / name
    if target.exists():
        shutil.rmtree(target)
print(f"预置资源已替换：{root}")
print("保留全部项目、附件及其他资源；引用退役主题的项目需手动重新选择主题。")
