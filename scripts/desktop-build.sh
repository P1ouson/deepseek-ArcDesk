#!/usr/bin/env bash
# Build and package the Wails desktop app for one platform (native runner only).
#
# Outputs in <repo>/dist/ (installer-style names for GitHub Releases):
#   arcdesk-desktop-windows-amd64-installer.exe
#   arcdesk-desktop-darwin-universal.dmg
#   arcdesk-desktop-darwin-{arm64,amd64}.zip   (updater channel)
#   arcdesk-desktop-linux-amd64-installer.tar.gz  (binary + install.sh)
#
# Usage: scripts/desktop-build.sh <os/arch> <version>
set -euo pipefail

PLATFORM="${1:?usage: desktop-build.sh <os/arch> <version>}"
VERSION="${2:?usage: desktop-build.sh <os/arch> <version>}"

os="${PLATFORM%/*}"
arch="${PLATFORM#*/}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APPNAME="ArcDesk"
WAILS_OUT="arcdesk-desktop"
RELEASE_PREFIX="arcdesk-desktop"

cd "$ROOT/desktop"

# desktop-v0.1.2 / v0.1.2 -> 0.1.2 (NSIS needs X.X.X.X via wails productVersion)
numver="${VERSION#desktop-}"
numver="${numver#v}"
numver="${numver%%-*}"
node -e 'const fs=require("fs"),f="wails.json",j=JSON.parse(fs.readFileSync(f,"utf8"));j.info.productVersion=process.argv[1];fs.writeFileSync(f,JSON.stringify(j,null,2)+"\n")' "$numver"

build_args=(-clean -platform "$PLATFORM" -ldflags "-X main.version=$VERSION")
[ "$os" = windows ] && build_args+=(-nsis)
[ "$os" = linux ] && build_args+=(-tags webkit2_41)

echo "==> wails build ${build_args[*]}"
wails build "${build_args[@]}"

mkdir -p "$ROOT/dist"

case "$os" in
darwin)
	staging=$(mktemp -d)
	app="$staging/${APPNAME}.app"
	cp -R "build/bin/${WAILS_OUT}.app" "$app"
	codesign --force --deep -s - "$app" 2>/dev/null || true
	if [ "$arch" = universal ]; then
		ditto -c -k --keepParent "$app" "$ROOT/dist/${RELEASE_PREFIX}-darwin-arm64.zip"
		ditto -c -k --keepParent "$app" "$ROOT/dist/${RELEASE_PREFIX}-darwin-amd64.zip"
	else
		ditto -c -k --keepParent "$app" "$ROOT/dist/${RELEASE_PREFIX}-darwin-${arch}.zip"
	fi
	dmgsrc=$(mktemp -d)
	cp -R "$app" "$dmgsrc/${APPNAME}.app"
	dmg="$ROOT/dist/${RELEASE_PREFIX}-darwin-universal.dmg"
	if command -v create-dmg >/dev/null 2>&1; then
		create-dmg \
			--volname "$APPNAME" \
			--window-size 540 380 \
			--icon-size 110 \
			--icon "${APPNAME}.app" 150 190 \
			--app-drop-link 390 190 \
			--no-internet-enable \
			"$dmg" "$dmgsrc" || true
	fi
	if [ ! -f "$dmg" ]; then
		hdiutil create -volname "$APPNAME" -srcfolder "$dmgsrc" -ov -format UDZO "$dmg"
	fi
	rm -rf "$staging" "$dmgsrc"
	;;
windows)
	installer=$(ls build/bin/*installer*.exe 2>/dev/null | head -n1 || true)
	[ -n "$installer" ] || { echo "no NSIS installer in build/bin" >&2; exit 1; }
	cp "$installer" "$ROOT/dist/${RELEASE_PREFIX}-windows-${arch}-installer.exe"
	;;
linux)
	pack=$(mktemp -d)
	cp "build/bin/$WAILS_OUT" "$pack/$WAILS_OUT"
	cp "$ROOT/desktop/scripts/linux-install.sh" "$pack/install.sh"
	[ -f "$ROOT/desktop/build/appicon.png" ] && cp "$ROOT/desktop/build/appicon.png" "$pack/appicon.png"
	chmod +x "$pack/install.sh" "$pack/$WAILS_OUT"
	tar -czf "$ROOT/dist/${RELEASE_PREFIX}-linux-${arch}-installer.tar.gz" -C "$pack" .
	rm -rf "$pack"
	;;
*)
	echo "unsupported os: $os" >&2
	exit 1
	;;
esac

echo "==> packaged:"
ls -la "$ROOT/dist"
