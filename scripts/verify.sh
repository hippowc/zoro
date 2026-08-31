#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/target/release/zoro"
KB="$ROOT/examples/knowledge-base"
export ZORO_WORKSPACE="$KB/zoro.toml"

echo "== zoro index =="
"$BIN" index
echo
echo "== zoro git =="
"$BIN" git
echo
echo "== zoro 定投 =="
"$BIN" 定投
echo
echo "== zoro html 定投（HTML 渲染）=="
"$BIN" html 定投 | head -15
