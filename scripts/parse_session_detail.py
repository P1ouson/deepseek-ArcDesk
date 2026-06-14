import json
import sys
from pathlib import Path


def analyze(path: str) -> None:
    path = Path(path)
    tel = Path(str(path) + ".telemetry.json")
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    msgs = [json.loads(l) for l in lines if l.strip()]

    usage_total = {"prompt": 0, "completion": 0, "cache_hit": 0, "cache_miss": 0}
    if tel.exists():
        raw = json.loads(tel.read_text(encoding="utf-8"))
        rows = raw if isinstance(raw, list) else raw.get("turns") or raw.get("usage") or []
        for u in rows:
            if not isinstance(u, dict):
                continue
            usage_total["prompt"] += u.get("prompt_tokens") or u.get("PromptTokens") or 0
            usage_total["completion"] += u.get("completion_tokens") or u.get("CompletionTokens") or 0
            usage_total["cache_hit"] += u.get("cache_hit_tokens") or u.get("CacheHitTokens") or 0
            usage_total["cache_miss"] += u.get("cache_miss_tokens") or u.get("CacheMissTokens") or 0

    tool_out_bytes: dict[str, int] = {}
    steps: list[dict] = []
    step: dict = {
        "user": "",
        "tools": [],
        "tool_bytes": 0,
        "assistant_preview": "",
        "verify_retry": False,
    }

    for m in msgs:
        role = m.get("role")
        if role == "system":
            sp = m.get("content") or ""
            steps.append({"kind": "system", "chars": len(sp), "preview": sp[:120].replace("\n", " ")})
        elif role == "user":
            if step.get("user") or step.get("tools"):
                steps.append(step)
                step = {
                    "user": "",
                    "tools": [],
                    "tool_bytes": 0,
                    "assistant_preview": "",
                    "verify_retry": False,
                }
            c = m.get("content") or ""
            step["user"] = c[:300].replace("\n", " ")
            low = c.lower()
            if "final-answer readiness" in low or "host final-answer" in low:
                step["verify_retry"] = True
        elif role == "assistant":
            c = m.get("content") or ""
            if c.strip():
                step["assistant_preview"] = c[:220].replace("\n", " ")
            for tc in m.get("tool_calls") or []:
                name = tc.get("name") or (tc.get("function") or {}).get("name")
                args = tc.get("arguments") or (tc.get("function") or {}).get("arguments") or ""
                step["tools"].append({"name": name, "args": str(args)[:140]})
        elif role == "tool":
            c = m.get("content") or ""
            name = m.get("name") or "?"
            b = len(c.encode("utf-8"))
            step["tool_bytes"] += b
            tool_out_bytes[name] = tool_out_bytes.get(name, 0) + b
    if step.get("user") or step.get("tools"):
        steps.append(step)

    user_turns = [s for s in steps if isinstance(s, dict) and s.get("user")]
    print(f"\n{'=' * 72}")
    print(f"FILE: {path.name}")
    print(f"SIZE: {path.stat().st_size // 1024}KB  messages: {len(msgs)}  user_turns: {len(user_turns)}")
    sys_line = next((s for s in steps if s.get("kind") == "system"), None)
    if sys_line:
        print(f"SYSTEM: {sys_line['chars'] // 1024}KB chars  preview={sys_line['preview']!r}")
    if tel.exists():
        print(
            f"TELEMETRY: prompt~{usage_total['prompt']} completion~{usage_total['completion']} "
            f"cache_hit~{usage_total['cache_hit']} cache_miss~{usage_total['cache_miss']}"
        )
    print("TOOL OUTPUT BYTES top:", sorted(tool_out_bytes.items(), key=lambda x: -x[1])[:10])

    for i, s in enumerate(user_turns, 1):
        tag = " [VERIFY RETRY]" if s.get("verify_retry") else ""
        print(f"\n--- User turn {i}{tag} ---")
        print("USER:", s["user"][:280])
        for j, t in enumerate(s.get("tools") or [], 1):
            print(f"  {j}. {t['name']}  args={t['args'][:110]}")
        if s.get("tool_bytes"):
            print(f"  [tool output this turn: {s['tool_bytes'] // 1024}KB]")
        if s.get("assistant_preview"):
            print("  ASSISTANT:", s["assistant_preview"][:220])

    for m in reversed(msgs):
        if m.get("role") == "assistant" and (m.get("content") or "").strip():
            print("\nFINAL ASSISTANT:", (m.get("content") or "")[:500].replace("\n", " "))
            break


if __name__ == "__main__":
    for p in sys.argv[1:]:
        if Path(p).exists():
            analyze(p)
