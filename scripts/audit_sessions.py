#!/usr/bin/env python3
"""Quick session jsonl audit for tool patterns."""
import json
import glob
import os
import collections
from pathlib import Path

DIRS = [
    Path(os.environ.get("LOCALAPPDATA", "")).parent / "Roaming" / "arcdesk" / "sessions",
    Path(os.environ.get("LOCALAPPDATA", "")).parent / "Roaming" / "reasonix" / "sessions",
]

for d in DIRS:
    if not d.is_dir():
        continue
    print(f"\n=== {d} ===")
    files = sorted(d.glob("*.jsonl"), key=lambda p: p.stat().st_size, reverse=True)
    for fp in files[:10]:
        tools = collections.Counter()
        users = assistants = tool_msgs = 0
        turns = []
        cur_user = None
        cur_tools = []
        try:
            with fp.open(encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    m = json.loads(line)
                    role = m.get("role")
                    if role == "user":
                        users += 1
                        if cur_user is not None:
                            turns.append((cur_user, list(cur_tools)))
                        cur_user = (m.get("content") or "")[:100]
                        cur_tools = []
                    elif role == "assistant":
                        assistants += 1
                        for tc in m.get("tool_calls") or []:
                            name = tc.get("name") or (tc.get("function") or {}).get("name")
                            if name:
                                tools[name] += 1
                                cur_tools.append(name)
                    elif role == "tool":
                        tool_msgs += 1
            if cur_user is not None:
                turns.append((cur_user, list(cur_tools)))
        except Exception as e:
            print(f"  ERR {fp.name}: {e}")
            continue
        sz = fp.stat().st_size
        dup_reads = sum(1 for _, ts in turns if ts.count("read_file") > 2)
        print(
            f"  {fp.name}: {sz//1024}KB users={users} asst={assistants} "
            f"tool_msgs={tool_msgs} dup_read_turns={dup_reads} "
            f"top={dict(tools.most_common(6))}"
        )
