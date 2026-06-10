#!/usr/bin/env python3
"""Blur sensitive regions in README sidebar screenshots."""

from __future__ import annotations

from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

# (left, top, right, bottom) — tuned for 1024×543/544 sidebar captures
REDACTIONS: dict[str, list[tuple[int, int, int, int]]] = {
    "sidebar-code.png": [
        (478, 510, 1022, 543),  # 底部用量 / 余额
    ],
    "sidebar-write.png": [
        (478, 509, 1022, 542),
    ],
    "sidebar-extensions.png": [
        (478, 510, 1022, 543),
    ],
    "sidebar-schedule.png": [
        (478, 510, 1022, 543),
        (512, 288, 1018, 362),  # 工作区根目录整行（含 E:\ 路径）
    ],
    "sidebar-connect.png": [
        (328, 218, 668, 332),   # 连接状态 + 局域网地址
        (328, 358, 755, 500),   # 跟随桌面配置
        (658, 138, 1015, 360),  # 已配对设备
    ],
    "sidebar-settings.png": [
        (118, 32, 700, 110),  # 配置文件路径整行
    ],
    "runtime-agent-approval.png": [
        (478, 508, 1022, 544),  # 底部用量 / 余额
    ],
    "runtime-web-preview.png": [
        (478, 508, 1022, 544),
    ],
    "runtime-writing.png": [
        (478, 509, 1022, 544),
    ],
}


def sample_fill_color(im: Image.Image, box: tuple[int, int, int, int]) -> tuple[int, int, int]:
    left, top, right, bottom = box
    w, h = im.size
    left = max(0, left)
    top = max(0, top)
    right = min(w, right)
    bottom = min(h, bottom)
    sample_top = max(0, top - 6)
    sample = im.crop((left, sample_top, right, top))
    if sample.size[0] == 0 or sample.size[1] == 0:
        sample = im.crop((left, top, right, min(h, top + 4)))
    pixels = list(sample.getdata())
    if not pixels:
        return (236, 238, 242)
    r = sum(p[0] for p in pixels) // len(pixels)
    g = sum(p[1] for p in pixels) // len(pixels)
    b = sum(p[2] for p in pixels) // len(pixels)
    return (r, g, b)


def blur_box(im: Image.Image, box: tuple[int, int, int, int], radius: int = 12) -> None:
    left, top, right, bottom = box
    w, h = im.size
    left = max(0, left)
    top = max(0, top)
    right = min(w, right)
    bottom = min(h, bottom)
    if right <= left or bottom <= top:
        return
    color = sample_fill_color(im, (left, top, right, bottom))
    draw = ImageDraw.Draw(im)
    draw.rectangle((left, top, right, bottom), fill=color)
    region = im.crop((left, top, right, bottom))
    blurred = region.filter(ImageFilter.GaussianBlur(radius=max(2, radius // 4)))
    im.paste(blurred, (left, top))


def process_dir(directory: Path) -> None:
    for name, boxes in REDACTIONS.items():
        path = directory / name
        if not path.exists():
            print(f"skip missing: {path}")
            continue
        im = Image.open(path).convert("RGB")
        for box in boxes:
            blur_box(im, box)
        im.save(path, optimize=True)
        print(f"redacted: {path}")


if __name__ == "__main__":
    repo = Path(__file__).resolve().parents[1] / "docs" / "screenshots"
    desktop = Path.home() / "Desktop" / "docs" / "screenshots"
    process_dir(repo)
    if desktop.exists():
        process_dir(desktop)
