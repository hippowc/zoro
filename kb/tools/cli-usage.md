# CLI 用法（tools/cli-usage.md）

> 原子配方：zoro CLI 的工作区、命令与默认目录。端到端见 `skills/add-library-search-preview.md`。

## 构建入口

```bash
go build -o bin/zoro ./cmd/zoro
```

## 跑示例工作区

```bash
export ZORO_WORKSPACE="$PWD/examples/knowledge-base/zoro.toml"

./bin/zoro index             # 重建元数据 manifest
./bin/zoro search 定投       # 查询并打印候选
./bin/zoro preview 定投      # 按库级 preview 配置渲染（默认 HTML）
./bin/zoro open 定投 --print # 打印第一条命中源文件路径（去掉 --print 调起系统应用打开）
./bin/zoro text 定投         # 终端文本（TTY 彩色 ANSI，否则纯文本）
./bin/zoro html 定投         # Markdown → HTML
```

## 新建工作区

```bash
# 项目级工作区：指定 ZORO_WORKSPACE 时写入该文件
mkdir -p /tmp/notes && cd /tmp/notes
export ZORO_WORKSPACE="$PWD/zoro.toml"
./bin/zoro add 投资笔记 ./kb   # root 会解析成绝对路径写入；第一个库自动设为 default
./bin/zoro index
./bin/zoro search 定投

# 全局默认工作区：不指定时用 ~/.zoro/zoro.toml（首次自动创建）
./bin/zoro add 默认库 ~/notes
```

## 命令表

| 命令 | 说明 |
|------|------|
| `zoro`（无子命令） | 打开 `default` 库并打印全部候选（无 default 则显示帮助） |
| `zoro add <name> <root> [--default]` | 添加知识库并写入/创建 `zoro.toml`（新建 root；首个库自动 default） |
| `zoro search <query>` | 模糊查询并打印候选（省略 query：浏览默认库，否则全部库；支持多词） |
| `zoro preview <query>` | 取第一条命中，按库级 `preview` 渲染（默认 HTML；支持多词） |
| `zoro open <query> [--print]` | 打印/打开第一条命中源文件（含行号；`--print` 仅打印路径） |
| `zoro html <query>` | 第一条命中渲染 Markdown → HTML |
| `zoro text <query>` | 第一条命中渲染终端文本（TTY 彩色 ANSI，否则纯文本） |
| `zoro index` | 强制重建所有库 manifest + `zoro-index.tsv` |
| `zoro serve` | 本地 Web（P2，尚未实现） |

## 工作区查找与默认目录

- `zoro.toml` 查找顺序：`ZORO_WORKSPACE` → `./zoro.toml`（若存在）→ `~/.zoro/zoro.toml`（首次自动创建）。
- `~/.zoro/zoro.toml`：默认配置（库 root 存绝对路径）。
- `~/.zoro/kb/`：默认知识库（自动生成 `默认知识库.md`，用于临时保存/默认搜索）。
- `~/.zoro/index/<库名>/`：manifest 与 `zoro-index.tsv`（索引里的 `path` 是相对知识库目录的路径）。
