#!/usr/bin/env python3
"""Render desktop/build/appicon.png and build/windows/icon.ico from frontend logo.svg.

Wails embeds build/windows/icon.ico into the Windows exe and systray. The repo
fork still shipped the upstream Reasonix appicon until this script is run before
`wails build`.
"""

from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LOGO_SVG = ROOT / "frontend" / "src" / "assets" / "logo.svg"
APPICON_PNG = ROOT / "build" / "appicon.png"
ICON_ICO = ROOT / "build" / "windows" / "icon.ico"
SIZE = 1024
BLUE = (1, 83, 229, 255)
WHITE = (255, 255, 255, 255)


def cubic(p0, p1, p2, p3, t: float) -> tuple[float, float]:
    u = 1 - t
    x = u**3 * p0[0] + 3 * u**2 * t * p1[0] + 3 * u * t**2 * p2[0] + t**3 * p3[0]
    y = u**3 * p0[1] + 3 * u**2 * t * p1[1] + 3 * u * t**2 * p2[1] + t**3 * p3[1]
    return x, y


def render_appicon_png(path: Path) -> None:
    try:
        from PIL import Image, ImageDraw
    except ImportError as exc:  # pragma: no cover - dev helper
        raise SystemExit("pip install pillow") from exc

    scale = SIZE / 32.0
    img = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    draw.rounded_rectangle([0, 0, SIZE - 1, SIZE - 1], radius=int(7 * scale), fill=BLUE)

    pts: list[tuple[float, float]] = []
    for i in range(101):
        t = i / 100
        x, y = cubic((8, 21.25), (10.2, 11.8), (13.1, 9.25), (16, 9.25), t)
        pts.append((x * scale, y * scale))
    for i in range(1, 101):
        t = i / 100
        x, y = cubic((16, 9.25), (18.9, 9.25), (21.8, 11.8), (24, 21.25), t)
        pts.append((x * scale, y * scale))

    stroke = max(3, int(2.75 * scale))
    draw.line(pts, fill=WHITE, width=stroke * 2, joint="curve")

    cx, cy = 16 * scale, 21.75 * scale
    cr = 1.85 * scale
    draw.ellipse([cx - cr, cy - cr, cx + cr, cy + cr], fill=WHITE)

    path.parent.mkdir(parents=True, exist_ok=True)
    img.save(path, "PNG")


def render_icon_ico(png_path: Path, ico_path: Path) -> None:
    try:
        from PIL import Image
    except ImportError as exc:  # pragma: no cover - dev helper
        raise SystemExit("pip install pillow") from exc

    img = Image.open(png_path).convert("RGBA")
    sizes = [16, 24, 32, 48, 64, 128, 256]
    icons = [img.resize((s, s), Image.Resampling.LANCZOS) for s in sizes]
    ico_path.parent.mkdir(parents=True, exist_ok=True)
    icons[-1].save(ico_path, format="ICO", sizes=[(i.width, i.height) for i in icons])


def main() -> None:
    if not LOGO_SVG.exists():
        raise SystemExit(f"missing logo source: {LOGO_SVG}")
    render_appicon_png(APPICON_PNG)
    render_icon_ico(APPICON_PNG, ICON_ICO)
    print(f"wrote {APPICON_PNG}")
    print(f"wrote {ICON_ICO}")


if __name__ == "__main__":
    main()
