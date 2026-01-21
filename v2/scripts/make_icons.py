from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image


def save_png(image: Image.Image, size: int, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    resized = image.resize((size, size), Image.LANCZOS)
    resized.save(path, format="PNG", optimize=True)


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    default_src = root / "VideoWithYou.png"
    parser = argparse.ArgumentParser(description="Generate extension and client icons.")
    parser.add_argument("--src", type=Path, default=default_src, help="Path to source PNG.")
    args = parser.parse_args()

    src_path = args.src
    if not src_path.exists():
        raise SystemExit(f"source image not found: {src_path}")

    image = Image.open(src_path).convert("RGBA")

    ext_dir = root / "extension" / "public" / "icons"
    for size in (16, 32, 48, 128):
        save_png(image, size, ext_dir / f"icon{size}.png")

    assets_dir = root / "local-client" / "assets"
    assets_dir.mkdir(parents=True, exist_ok=True)
    ico_path = assets_dir / "client.ico"
    image.save(ico_path, format="ICO", sizes=[(16, 16), (32, 32), (48, 48), (256, 256)])

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
