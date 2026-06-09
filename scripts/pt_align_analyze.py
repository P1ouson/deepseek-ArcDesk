#!/usr/bin/env python3
"""Aggregate tool-usage stats from session jsonl files (before/after PT-align fix)."""
from __future__ import annotations

import json
import sys
from collections import Counter, defaultdict
from pathlib import Path

TRACK = {
    "bash", "read_file", "grep", "glob", "web_fetch",
    "todo_write", "complete_step", "wait", "kill_shell", "bash_output",
}
HALLUC = {"todo_write", "complete_step", "wait", "kill_shell", "bash_output"}


def analyze_file(path: Path) -> dict:
    tools = Counter()
    failed = Counter()
    halluc_attempts = Counter()
    bg_bash = 0
    users = 0
    has_scope = False
    dup_read_turns = 0
    cur_tools: list[str] = []

    with path.open(encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            m = json.loads(line)
            role = m.get("role")
            content = m.get("content") or ""
            if role == "system" and "# Session tool scope" in content:
                has_scope = True
            if role == "user":
                users += 1
                if cur_tools.count("read_file") > 2:
                    dup_read_turns += 1
                cur_tools = []
            elif role == "assistant":
                for tc in m.get("tool_calls") or []:
                    name = tc.get("name") or (tc.get("function") or {}).get("name") or ""
                    if not name:
                        continue
                    tools[name] += 1
                    cur_tools.append(name)
                    if name in HALLUC:
                        halluc_attempts[name] += 1
                    if name == "bash":
                        args = tc.get("arguments") or (tc.get("function") or {}).get("arguments") or ""
                        if "run_in_background" in str(args) and "true" in str(args).lower():
                            bg_bash += 1
            elif role == "tool":
                if "unknown tool" in content.lower():
                    failed["unknown_tool"] += 1

    if cur_tools.count("read_file") > 2:
        dup_read_turns += 1

    total_calls = sum(tools.values())
    read_like = tools["read_file"] + tools["grep"] + tools["glob"] + tools.get("ls", 0)
    bash_n = tools["bash"]
    return {
        "path": str(path),
        "name": path.name,
        "users": users,
        "has_scope": has_scope,
        "tools": dict(tools),
        "total_calls": total_calls,
        "bash": bash_n,
        "read_file": tools["read_file"],
        "grep": tools["grep"],
        "glob": tools["glob"],
        "web_fetch": tools["web_fetch"],
        "read_like": read_like,
        "bash_ratio": (bash_n / total_calls) if total_calls else 0.0,
        "read_ratio": (read_like / total_calls) if total_calls else 0.0,
        "halluc_attempts": dict(halluc_attempts),
        "halluc_total": sum(halluc_attempts.values()),
        "failed_unknown": failed["unknown_tool"],
        "dup_read_turns": dup_read_turns,
        "bg_bash": bg_bash,
    }


def aggregate(rows: list[dict]) -> dict:
    if not rows:
        return {}
    n = len(rows)
    sums = Counter()
    halluc = Counter()
    scope = sum(1 for r in rows if r["has_scope"])
    users = sum(r["users"] for r in rows)
    dup = sum(r["dup_read_turns"] for r in rows)
    for r in rows:
        for k in ("bash", "read_file", "grep", "glob", "web_fetch", "total_calls", "read_like", "halluc_total", "failed_unknown", "bg_bash"):
            sums[k] += r.get(k, 0)
        for t, c in r.get("halluc_attempts", {}).items():
            halluc[t] += c
    tc = sums["total_calls"] or 1
    return {
        "sessions": n,
        "user_turns": users,
        "with_scope": scope,
        "total_tool_calls": sums["total_calls"],
        "bash": sums["bash"],
        "read_file": sums["read_file"],
        "grep": sums["grep"],
        "glob": sums["glob"],
        "web_fetch": sums["web_fetch"],
        "read_like": sums["read_like"],
        "bash_per_session": sums["bash"] / n,
        "read_file_per_session": sums["read_file"] / n,
        "bash_ratio": sums["bash"] / tc,
        "read_ratio": sums["read_like"] / tc,
        "halluc_total": sums["halluc_total"],
        "halluc_by_tool": dict(halluc),
        "failed_unknown": sums["failed_unknown"],
        "dup_read_turns": dup,
        "bg_bash": sums["bg_bash"],
    }


def load_dir(d: Path, after_only: bool | None) -> list[dict]:
    rows = []
    for fp in sorted(d.glob("*.jsonl")):
        row = analyze_file(fp)
        if after_only is True and not row["has_scope"]:
            continue
        if after_only is False and row["has_scope"]:
            continue
        rows.append(row)
    return rows


def main() -> int:
    roots = []
    if len(sys.argv) > 1:
        roots = [Path(p) for p in sys.argv[1:]]
    else:
        app = Path.home() / "AppData" / "Roaming"
        roots = [app / "reasonix" / "sessions", app / "arcdesk" / "sessions"]
        bench = Path(__file__).resolve().parent.parent / "benchmarks" / "pt-align" / "sessions"
        if bench.is_dir():
            roots.append(bench)

    before: list[dict] = []
    after: list[dict] = []
    for root in roots:
        if not root.is_dir():
            continue
        for row in load_dir(root, None):
            if row["has_scope"]:
                after.append(row)
            else:
                before.append(row)

    print(json.dumps({"before": aggregate(before), "after": aggregate(after)}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
