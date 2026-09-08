#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export PATH="$PATH:/usr/local/go/bin"
# 容器/只读主目录环境下，把构建缓存放到临时目录；沿用已预热的 /tmp 缓存。
export GOCACHE="${GOCACHE:-/tmp/gocache}"
export GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}"
export GOPATH="${GOPATH:-/tmp/gopath}"
export ZORO_WORKSPACE="$ROOT/examples/knowledge-base/zoro.toml"

echo "== go test ./... =="
go test ./...

echo
echo "== build CLI =="
go build -o bin/zoro ./cmd/zoro

echo
echo "== zoro index =="
./bin/zoro index

echo
echo "== zoro search git =="
./bin/zoro search git

echo
echo "== zoro search 定投 =="
./bin/zoro search 定投

echo
echo "== zoro preview 定投（按 preview 配置渲染）=="
./bin/zoro preview 定投 | head -15

echo
echo "== zoro text 定投（非 TTY 纯文本，前 15 行）=="
./bin/zoro text 定投 | head -15

echo
echo "== zoro html 定投（HTML 渲染，前 15 行）=="
./bin/zoro html 定投 | head -15

echo
echo "== zoro open --print 定投（打印源文件路径，不实际打开）=="
./bin/zoro open --print 定投

echo
echo "== zoro add 空工作区 MVP 验证（临时目录）=="
tmpadd="$(mktemp -d)"
(
  unset ZORO_WORKSPACE
  cd "$tmpadd"
  "$ROOT/bin/zoro" add demo ./demo
  grep -q "name = 'demo'" zoro.toml
  grep -q "root = './demo'" zoro.toml
  if "$ROOT/bin/zoro" add demo ./demo >/dev/null 2>&1; then
    echo "FAIL: duplicate add succeeded"
    exit 1
  fi
)
rm -rf "$tmpadd"
echo "add ok"
