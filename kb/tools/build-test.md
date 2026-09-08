# 构建与测试（tools/build-test.md）

> 原子配方：怎样在本地把 cli / launcher 编译出来、跑测试、做静态检查。

## 前置

- Go 1.27（`/usr/local/go/bin` 一般在 PATH；如不在先 `export PATH=$PATH:/usr/local/go/bin`）。
- 网络受限时先导出模块缓存环境：
  ```bash
  export GOCACHE=/tmp/gocache
  export GOPATH=/tmp/gopath
  export GOMODCACHE=/tmp/gomodcache
  export GOPROXY=https://goproxy.cn,direct
  ```

## CLI（根模块）

```bash
cd /root/zoro
go build -o /tmp/zoro ./cmd/zoro   # 或 bin/zoro
go test ./...
go vet ./...
bash scripts/verify.sh             # 需要时
```

## Launcher（独立 go module，Linux 只做开发验证）

```bash
cd /root/zoro/ext/zoro-launcher
export PATH=$PATH:/usr/local/go/bin
export GOCACHE=/tmp/gocache GOPATH=/tmp/gopath GOMODCACHE=/tmp/gomodcache
export GOPROXY=https://goproxy.cn,direct
go build -o /tmp/zoro-launcher-linux .   # 不含全局热键
go vet ./...
```

前端 JS 语法检查：

```bash
node --check /root/zoro/ext/zoro-launcher/frontend/dist/app.js
```

## 发布级构建（macOS）

- 必须在 macOS + Xcode 工具链；Linux 无法交叉编译 macOS 版（见 `facts/pitfalls.md` P-5）。
- 本地 Apple Silicon：`cd ext/zoro-launcher && ./build-macos.sh`。
- CI：推送 `launcher-*` tag 触发 GitHub Actions，见 `tools/launcher-build-release.md`。

## 测试原则

- `go test ./...` + 文本夹具；core 不带 UI / fzf / 网络依赖（`go list` 可验证）。
- 验收清单见 `tools/verify.md`。
