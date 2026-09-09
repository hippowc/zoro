#!/usr/bin/env bash
# 在 macOS（Apple Silicon）上构建 zoro 桌面 Launcher。
# 首次使用：
#   1) 安装 Xcode Command Line Tools： xcode-select --install
#   2) 安装 Go 1.27+（https://go.dev/dl/）
#   3) 安装 Node.js 18+（https://nodejs.org/ 或 brew install node）
#      —— frontend/dist/styles.css 由 Tailwind 编译，wails build 会自动跑
#   4) 安装 Wails CLI： go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
# 之后： ./build-macos.sh
set -euo pipefail
cd "$(dirname "$0")"

ARCH="${ZORO_LAUNCHER_ARCH:-arm64}"   # Apple Silicon；Intel 用 amd64
APP_NAME="${ZORO_LAUNCHER_APP_NAME:-zoro-launcher}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export PATH="$(go env GOPATH)/bin:$PATH"

if ! command -v wails >/dev/null 2>&1; then
  echo "缺少 wails CLI，正在安装 wails@v2.15.0 …" >&2
  go install "github.com/wailsapp/wails/v2/cmd/wails@v2.15.0"
fi

if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
  echo "缺少 Node.js/npm：Tailwind 需要它把 frontend/src/input.css" >&2
  echo "编译成 frontend/dist/styles.css（wails build 会自动触发）。" >&2
  echo "安装： brew install node   或   https://nodejs.org/" >&2
  exit 1
fi

echo "== 构建 darwin/${ARCH} =="
wails build -clean -platform "darwin/${ARCH}" -o "$APP_NAME"

APP="build/bin/${APP_NAME}.app"
if command -v codesign >/dev/null 2>&1; then
  echo "== ad-hoc 签名（本地运行无需 Apple Developer 证书）=="
  codesign --force --deep --sign - "$APP"
fi

echo
echo "完成：$(pwd)/${APP}"
echo "运行： open \"${APP}\""
echo "首次运行若被 Gatekeeper 拦截： xattr -cr \"${APP}\" && open \"${APP}\""
