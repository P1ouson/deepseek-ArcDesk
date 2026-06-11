#!/usr/bin/env python3
"""Capture a short ArcDesk window GIF for README hero (Windows)."""
from __future__ import annotations

import ctypes
from ctypes import wintypes
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EXE = ROOT / "desktop" / "build" / "bin" / "arcdesk-desktop.exe"
OUT = ROOT / "docs" / "screenshots" / "desktop-hero.gif"
FRAMES = 8
INTERVAL = 0.75


def main() -> None:
    if not EXE.exists():
        raise SystemExit(f"build first: {EXE}")

    try:
        from PIL import Image, ImageGrab
    except ImportError:
        raise SystemExit("pip install pillow")

    user32 = ctypes.windll.user32
    SW_RESTORE = 9

    subprocess.run(
        ["taskkill", "/IM", "arcdesk-desktop.exe", "/F"],
        capture_output=True,
    )
    time.sleep(1)

    proc = subprocess.Popen([str(EXE)])
    hwnd = None
    for _ in range(40):
        hwnd = user32.FindWindowW(None, "arcdesk")
        if hwnd:
            break
        time.sleep(0.5)
    if not hwnd:
        proc.kill()
        raise SystemExit("ArcDesk window not found")

    user32.ShowWindow(hwnd, SW_RESTORE)
    user32.SetForegroundWindow(hwnd)
    time.sleep(10)

    rect = wintypes.RECT()
    user32.GetWindowRect(hwnd, ctypes.byref(rect))
    box = (rect.left, rect.top, rect.right, rect.bottom)
    if box[2] - box[0] < 200:
        proc.kill()
        raise SystemExit(f"window too small: {box}")

    frames: list[Image.Image] = []
    for _ in range(FRAMES):
        shot = ImageGrab.grab(bbox=box).convert("P", palette=Image.ADAPTIVE, colors=128)
        frames.append(shot)
        time.sleep(INTERVAL)

    OUT.parent.mkdir(parents=True, exist_ok=True)
    frames[0].save(
        OUT,
        save_all=True,
        append_images=frames[1:],
        duration=int(INTERVAL * 1000),
        loop=0,
        optimize=True,
    )
    proc.kill()
    print(f"wrote {OUT} ({len(frames)} frames)")


if __name__ == "__main__":
    main()
