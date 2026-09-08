# zoro

本地优先、可扩展的**通用知识库框架**。

- 核心 = `core`（Go package：解析 / 索引 / 查询 / 渲染管线）+ `cmd/zoro`（最小 CLI 前端）
- 内容源：**Markdown 唯一**；统一标签模型：每个 `@` 标签是一个块（标签名 + 搜索词 + 负载），如 `@index` 叙述、`@shell` 终端执行
- 多 target 展示：终端 / 本地 Web / 桌面 / 静态站点（扩展独立）
- 愿景路线：标签体系 → Web 体验 → 站点/云/安全
- 设计与开发规范见 [agents.md](agents.md)（第一版）

## 技术栈

Go + Wails v2。桌面 Launcher 位于 `ext/zoro-launcher`（独立 Go module，`replace` 引用本仓库 `core`）。

## 快速开始

```bash
# 构建 CLI
go build -o bin/zoro ./cmd/zoro

# 跑示例工作区
export ZORO_WORKSPACE="$PWD/examples/knowledge-base/zoro.toml"

./bin/zoro index             # 重建元数据 manifest
./bin/zoro search 定投       # 查询并打印候选
./bin/zoro preview 定投      # 按库级 preview 配置渲染（默认 HTML）
./bin/zoro open 定投 --print # 打印第一条命中源文件路径（去掉 --print 则调起系统应用打开）
./bin/zoro text 定投         # 终端文本（TTY 彩色 ANSI，否则纯文本）
./bin/zoro html 定投         # Markdown → HTML
```

新建工作区（不污染示例）：

```bash
mkdir -p /tmp/notes && cd /tmp/notes
./bin/zoro add 投资笔记 ./kb   # 在空目录创建 zoro.toml 并添加第一个库（自动设为 default）
./bin/zoro index
./bin/zoro search 定投
```

## 命令

| 命令 | 说明 |
|------|------|
| `zoro`（无子命令） | 打开 `zoro.toml` 声明的 `default` 库并打印全部候选（无 default 则显示帮助） |
| `zoro add <name> <root> [--default]` | 添加知识库并写入/创建 `zoro.toml`（新建 root 目录；首个库自动设为 default） |
| `zoro search <query>` | 模糊查询并打印候选（省略 query：浏览默认库，否则全部库；支持多词） |
| `zoro preview <query>` | 取第一条命中，按库级 `preview` 配置渲染（默认 HTML；支持多词） |
| `zoro open <query> [--print]` | 打印/打开第一条命中所在源文件（含行号；`--print` 仅打印路径） |
| `zoro html <query>` | 取第一条命中，渲染 Markdown → HTML |
| `zoro text <query>` | 取第一条命中，渲染终端文本（TTY 彩色 ANSI，否则纯文本） |
| `zoro index` | 强制重建所有库的 manifest + `zoro-index.tsv` |
| `zoro serve` | 本地 Web（P2，尚未实现） |

`zoro.toml` 查找顺序：`ZORO_WORKSPACE` → `./zoro.toml`。

## 目录结构

```
zoro/
├── core/            # 核心库（纯 Go，无 UI / 无 fzf / 无网络）
├── cmd/zoro/        # CLI 前端
├── ext/zoro-launcher/  # Wails 桌面 Launcher（独立 go.mod）
├── tests/fixtures*  # 文本夹具
├── examples/        # 示例工作区
└── agents.md        # 开发规范（单一事实源）
```

## 桌面 Launcher

```bash
cd ext/zoro-launcher
go build -o zoro-launcher .
# 或使用 Wails CLI：wails build
```

Launcher v1 MVP：全局热键唤起 + 无边框浮窗 + 查询 + 预览 + 复制（回填/执行受平台权限限制，见 `agents.md` §9/§14）。

## 测试

```bash
go test ./...
```

## 下载（公网）

- 公网 IP：`8.146.233.2`（80 端口）
- 目录列表：http://8.146.233.2/
- macOS（Apple Silicon / arm64）：http://8.146.233.2/zoro-darwin-arm64
- Linux（amd64）：http://8.146.233.2/zoro-linux-amd64
- 校验和：http://8.146.233.2/SHA256SUMS.txt
- 下载目录：`/tmp/zoro-downloads`，放入新文件后通过 `http://8.146.233.2/<文件名>` 下载
- 网关：nginx（systemd 管理），站点配置 `/etc/nginx/sites-available/zoro-downloads`

macOS 首次运行（浏览器下载会带 quarantine）：

```bash
xattr -d com.apple.quarantine zoro-darwin-arm64
chmod +x zoro-darwin-arm64
./zoro-darwin-arm64
```

用 curl 下载一般只需：

```bash
curl -L -o zoro http://8.146.233.2/zoro-darwin-arm64
chmod +x zoro
```
