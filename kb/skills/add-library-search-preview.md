# 新增知识库并完成搜索 / 预览 / 打开（skills/add-library-search-preview.md）

> 端到端 SOP：从零建工作区到搜索消费一条命中。依赖 `tools/cli-usage.md`、`facts/architecture.md`（内容模型）。

## 目标

新建一个知识库，写入带 `@` 标签的 Markdown，建索引后能 search / preview / open。

## 步骤

### 1. 准备内容

```bash
mkdir -p /tmp/notes/git
cat > /tmp/notes/git/http.md <<'MD'
## 回滚最近一次提交

@index git reset 回滚 反向操作
git reset --soft HEAD~1 撤销提交并保留改动。

@shell git reset 回滚 反向操作
```bash
git reset --soft HEAD~1
```
MD
```

> 注意 `@shell` 后要跟 fenced code block；`@index` 后的正文到下一个 `@` 标签为止。

### 2. 建工作区并添加库

```bash
cd /tmp/notes
export ZORO_WORKSPACE="$PWD/zoro.toml"
/path/to/zoro add git ./git   # root 解析成绝对路径写入 zoro.toml；首个库自动 default
```

### 3. 建索引

```bash
/path/to/zoro index
# 检查产物：zoro.toml、git/.zoro/meta.json、git/zoro-index.tsv
```

### 4. 搜索 / 预览 / 打开

```bash
/path/to/zoro search reset      # 打印候选（带库名前缀与得分）
/path/to/zoro preview reset     # 第一条命中按 preview 配置渲染
/path/to/zoro open reset --print # 打印源文件路径（含行号）
/path/to/zoro html reset        # 渲染 HTML 到终端
```

### 5. 全局默认工作区（可选）

不设 `ZORO_WORKSPACE` 时，用 `~/.zoro/zoro.toml`；首次由 CLI/桌面端自动创建 `~/.zoro` 与默认知识库。

## 常见坑

- `zoro.toml` 不落当前目录 → 报 `no such file or directory`：先 `export ZORO_WORKSPACE=...`。
- 改了 `.md` 后查询没变化：重新 `zoro index`（当前脏检查按库内 mtime）。
- `@shell` 没绑定 fence 会降级为正文块，搜索词保留但不可执行复制。
