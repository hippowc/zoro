# zoro 架构与模型（facts/architecture.md）

> 模块职责、调用链、对外契约；只到模块/包级，不逐文件列。
> 本文是 `agents.md` 之外「反复查阅的设计细节」所在；改动这里等于设计变更。

## 定位与愿景

- zoro = 本地优先、可扩展的**通用知识库框架**。
- 核心 = 一个 Go package（`core`）+ 一个最小 CLI 前端（`cmd/zoro`）。
- 非终端优先：CLI / Web / Launcher 是三个平级前端——Web（`zoro serve`）是大众入口，CLI fzf 是终端效率入口，Launcher（全局快捷键唤起浮窗）是桌面速神入口；终端不是唯一形态。
- 愿景四步走：**标签体系 → Web 体验 → 站点/云/安全**（阶段归属见 `../todos.md`）。
- 内容源**只有一种**：Markdown（+ `@` 指令）；HTML 是渲染产物，不是源码。
- 运行三级流程：**内容（作者写）→ 元数据（analyze 产出）→ 消费（前端只读元数据 + 按需截原文）**。
- **一级对象是 Workspace**：工作区由 `zoro.toml` 声明多个 Library；单库只是列表长度为 1 的特例，不设特殊模式。

## 核心边界（最重要的口径）

```
core（Go package，无 UI / 无 fzf / 无网络）
 ├─ 内容模型：Block / TagKind / Registry / Library / LibraryConfig / Workspace
 ├─ 工作区：Workspace ← zoro.toml 声明（库列表 + 库级配置）
 ├─ 分析（analyze）：扫描 @标签、分块 → 库级元数据（manifest）
 ├─ 元数据：库级 manifest（v2：块视图 + shell 能力）+ mtime 脏检查
 ├─ 查询：Matcher interface（默认纯 Go 子序列模糊匹配，语义对齐 fzf；返回分数 + 命中区间）
 ├─ 渲染：Markdown → HTML（多 target 中的第一落点）
 └─ 扩展点：Render / Action / SyncProvider / Cipher（Go interface）
```

- `core` 不依赖任何 UI；fzf（仅 CLI）、Web、Launcher、SSG 都是消费 `core` 的前端；Launcher 用 Wails + 自绘列表，不引入 fzf。
- 依赖方向永远单向：`frontend → core`，绝不反向。
- **前端只消费元数据**，不直接扫正文；需要正文时按 `(库名, path, start)` 现场截取。
- 能力扩展走 interface；展示扩展 = 新的 main package 依赖 `core`。

## 不变式（原则）

1. **目录自由，内容受限**：内容目录零约束；唯一技术约束在「`@` 标记行」格式上。
2. **Markdown 唯一源**：作者只写 Markdown + `@` 指令（含标签式组件）；HTML/CSS 属于产物与样式层。
3. **统一标签模型**：每个标签都是平级的「块（Block）= 标签名（kind）+ 搜索词（terms）+ 负载（raw）」，边界到下一个标签 / EOF；`@index` 只是普通标签之一，不再特殊。
4. **唯一强制物是库级元数据（manifest）**：版本化、可重建；索引只是它的一个可读视图。
5. **Workspace 单一模型**：一律由 `zoro.toml` 声明库集合；单库 = `libraries` 长度为 1，无特殊入口。
6. **库级身份三元组**：块身份 = `(库名, path, start)`；库名必须出现在展示路径 / 候选行 / URL 中。
7. **策展型知识库**：默认只搜已声明库内的 Markdown 内容；不把“扫描本地所有文件”作为主路径；范围扩展仅作可选插件（Collector / Extractor / IndexBackend），不污染 Block 模型与 core。

## 内容模型：Block 与 @标签（唯一内容契约 v2）

### 统一规则

每个标签产出一个内容块：`@<name> <search-terms>` + 负载。块边界 = 该标签行 → 下一个任意 `@` 标签行 / EOF。

````markdown
## Git 丢弃本地修改（标题可选）

@index git checkout reset 丢弃 还原
`git checkout -- <file>` 会丢弃工作区改动，`reset --hard` 更危险。

@shell git checkout reset 丢弃 还原
```bash
git checkout -- <file>
git reset --hard <commit>
```

@video 操作演示 丢弃 还原
./assets/drop-changes.mp4
````

- 上面是**三个平级块**：`@index`=叙述块、`@shell`=终端执行块、`@video`=视频块，各自可独立命中检索。
- 块边界现场推导，**不存 end**；fence 内部行不是标签。

### v1 默认标签集（最小集）

| 标签 | 语义 | 搜索词 | 负载 | 消费方式 |
|------|------|--------|------|----------|
| `@index` | 叙述 / 定位 | 主体关键词 | 到下一标签之间的正文 | 渲染 Markdown |
| `@shell` | 终端执行 | 命令关键词 | 紧随的 fenced code block | 复制 / 终端执行 |
| `@video` | 视频 | 视频关键词 | 媒体引用（v1 占位） | 媒体预览 |
| `@image` | 图片 | 图片关键词 | 媒体引用（v1 占位） | 媒体预览 |
| `@card` | 卡片（标签式组件） | 组件关键词 | 4 空格缩进正文 | 组件渲染（预留） |

> 表里 v1 占位标签不会被 README 删除；内容契约的唯一事实源以此表与 `core/registry.go` 为准。

### `@shell`

- `@shell 搜索词` 后（可跳过空行）必须紧跟 fenced code block；fence 内每行即执行负载。
- 语言取自 fence info string；v1 语义锁定「在终端执行的 shell 命令」，默认语言 shell。
- 一个 `@shell` 绑定一个 fence；无 fence → **按正文块降级**（保留搜索词）。
- v1：`@shell` 被识别、校验、随正文展示；复制 / 执行见「扩展机制与 Action 分级」（P3 落地）。

### 标题（title）

取「标签向上最近的 `##`」作 title（仅允许中间只隔空行 / 其他 `@` 行）；无 `##` 取搜索词首词，再兜底为 kind。

### 新增标签的唯一判据

先回答：**属于哪个语义域（定位/能力/呈现/偏好），负载怎么取（正文/fence/其他结构）**；两者说不清就不加。登记入口 = `core/registry.go`。

## analyze 与库级元数据（manifest）

### 定位

- `analyze` 把每个库的内容「编译」为库级 manifest；查询/预览/执行以元数据为入口。
- **manifest 是派生物**：可删、可重建、幂等；真相永远在 Markdown + `@` 标签里。`.zoro/` 默认 gitignore。
- 需要正文时按 `(库名, path, start)` 现场截取；元数据**不存 end、不存正文副本、不存配置**。

### manifest 结构（schema v2）

路径：`<库根>/.zoro/meta.json`，**一个库一份**；workspace 组织关系由 `zoro.toml` 承载。

```json
{
  "schema": 2,
  "library": { "name": "sanji", "root": "/data/kb/sanji" },
  "generated_at": "1788166910",
  "blocks": [
    { "title": "Git 丢弃本地修改", "kind": "index",
      "index": "git checkout reset 丢弃 还原", "path": "git/常用操作.md", "start": 3 },
    { "title": "Git 丢弃本地修改", "kind": "shell",
      "index": "git checkout reset 丢弃 还原", "path": "git/常用操作.md", "start": 5,
      "shell": { "lang": "bash", "lines": [7, 8] } }
  ]
}
```

- `library.name` 与 `zoro.toml` 声明一致（加载校验）；`root` 仅作记录。
- `shell` 只记录 fence 语言 + 行号，不存命令正文。

### 声明 vs 决策（关键边界）

元数据只记录「内容声明了什么」；配置决定「工具怎么响应」：

| 来源 | 进哪里 | 示例 |
|------|--------|------|
| 文档内 `@` 标签 | manifest.blocks[].kind + shell | 有 bash 块可执行 |
| 用户/机器偏好 | `zoro.toml` 库级配置 | preview=html、allow_exec=false |

- 库级配置永不进 meta.json（删了 meta 重建不丢配置）；策略交给 `Render`/`Action` 注册表。

### 分块算法与生成失效

- 逐行扫描；任意 `@<name>` 行开新块并查注册表分类；`@shell` 进入 fence 收集态；`##` 记最近标题；其余归当前块正文。
- 主产物 `<库根>/.zoro/meta.json`；同写可读视图 `<库根>/zoro-index.tsv`（四列：title / index / start / path）。
- 查询前 mtime 脏检查（任一 `*.md` 晚于 meta 即 stale，后续升级文件级 fingerprint）；原子写（tmp + rename）。
- manifest 损坏 / schema 不匹配 → 自动重建（派生数据可丢）。
- 扫描范围（默认）= 已声明库根下 `*.md` / `*.mdx`；跳过隐藏目录（含 `.zoro/`、`.git/`）。
- 范围扩展统一走库级配置（`include`/`exclude` glob、`recursive`、`follow_symlinks`），默认不扫描库外、不把非 Markdown 文件当作知识；主路径不扩展到“本地所有文件”。

## Workspace 与库级配置

### `zoro.toml`

```toml
default = "sanji"            # 可选：无参进入的默认库

[[libraries]]
name = "sanji"
root = "/data/kb/sanji"

[libraries.config]
preview = "html"            # 候选：默认预览 target
# allow_exec = false        # 候选：是否允许执行 @shell

[[libraries]]
name = "robin"
root = "robin"              # 支持相对路径，相对 zoro.toml 所在目录解析
```

- **单库就是 `libraries` 只有一项**；已废除 `ZORO_ROOT` / `ZORO_LIBS`。
- 查找顺序：`ZORO_WORKSPACE` 环境变量 → `./zoro.toml`（若存在）→ `~/.zoro/zoro.toml`（首次自动创建）。
- 库 root 落盘为**绝对路径**；索引文件中的 `path` 为**相对知识库目录**的路径（身份三元组仍是 `(库名, path, start)`）。

### Workspace 运行时

- `core.LoadWorkspace(path)`：读声明 → 校验库名唯一 → 逐个库 `OpenCached`（冷启动优先，stale 自动重建）。
- 聚合查询跨库按分数排序；`LoadRaw(library, path, start)` 按三元组分发。

### 库级配置可扩展

```go
type LibraryConfig struct {
    Preview   string
    AllowExec *bool
    Extra     map[string]any   // 未知键保留，前向兼容
}
```

- 只做「该库怎么被消费」的偏好，不做内容语义（内容语义一律走 `@` 标签）。
- 未知键保留、不报错。

## 检索与匹配

**core**：`Matcher` interface（默认纯 Go 子序列模糊匹配，语义对齐 fzf）+ `Candidate{ library, title, index, path, start, score, matches }`；`matches` 为命中区间，供各前端高亮。

### 三面搜索（搜什么）

| 面 | 内容 | 权重 / 定位 | 状态 |
|----|------|-------------|------|
| 标题 title | 标签向上最近的 `##` | 低权重辅助面（记忆 / 召回） | 后续接入 |
| index terms | `@` 标签行的搜索词 | 高权重主搜索面（作者显式声明“何时被找到”） | P0/P1 落地 |
| 正文 raw | 块负载原文 | P4 全文检索兜底面 | 全文引擎 bleve/zinc 再定 |

- 三层分工：`index` = 精确 / 意图面，`title` = 记忆 / 召回面，`raw` = 兜底面；演进顺序固定：先 index，再 title，最后 raw（P4）。
- **P0/P1 只实现 index terms 主面**；title / raw 作为显式扩展点写入本文，避免后续误补错面。

### 主搜索面语义（index terms）

- 每个块的 search-terms 合并为 `index`；所有块平级参与匹配。
- 语义：**按空白拆词、词间 AND 且词序无关、词内字符级子序列匹配、大小写不敏感**。

### 搜索范围（策展型知识库）

- 默认只搜 `zoro.toml` 已声明库根下的 `*.md` / `*.mdx`（跳过隐藏目录）。
- **不把“扫描本地所有文件”作为主路径**；非 Markdown 文件、库目录之外的文件默认不进搜索面。
- 未来如需扩展：作为可选插件，提供 `Collector`（什么算知识源）→ `Extractor`（文件 → Block）→ `IndexBackend`（新索引面）抽象；复用 `(library, path, start)` 身份，普通文件可退化为“一文件一块”，不污染 core 的 Block 模型。
- 范围细化（`include`/`exclude` glob、`recursive`、`follow_symlinks`）统一进库级配置。

### 前端分工

- **CLI 前端**：把全量 candidates 交给 **fzf** 交互（默认控件，随前端捆绑，不进 core）；高亮由 fzf 自行计算或使用 `matches`。
- **Web / Launcher 前端**：调用 `core.Query(q)`，自绘输入框/列表，用 `matches` 高亮；Wails Launcher 是常驻进程 + 全局热键 + 无边框浮窗，不用 fzf。

fzf 三分离：匹配字段（`index`）≠ 显示字段（`title`）≠ 载荷（`(库,path)+start` 现场预览）。

## 渲染管线与标签组件（多 target）

```
Markdown 源
 ├─ 预处理：剥离 @指令（进元数据，不进 AST）
 ├─ goldmark（GFM）解析 → AST
 │   ├─ 普通块 → 标准 HTML
 │   ├─ fence + 已知语言 → chroma 高亮
 │   ├─ fence + mermaid/KaTeX → 前端渲染容器（预留）
 │   ├─ 标签组件(@card 等) → 组件注册表 → 自定义 HTML（预留）
 │   └─ raw HTML → sanitizer（预留）
 └─ 输出 target：HTML（第一落点）/ ANSI / 纯文本（多 target 扩展）
```

- 标签式组件仍走统一 `@` 语法（如 `@card title=概览`，4 空格缩进承载正文）。
- 样式 100% 由 CSS 主题层决定；不可信输入关 raw HTML 透传或严格 sanitize。
- 表达力边界：CSS 无上限；行内排版（粗斜/代码/链接/图片）+ 内嵌 HTML 兜底；块级结构靠 Markdown + `@` 组件；高度交互交给呈现层框架。
- **展示拆两维（正交）**：`target`（终端 / Web / 桌面 / 静态站点，决定产物形态：HTML / ANSI / 纯文本）与 `kind`（`@index`→Markdown、`@shell`→终端代码/动作、`@image`/`@video`→媒体预览、未来 `@mermaid`/`@mindmap`→前端图形视图）正交；core 只输出 target 级产物与足够元数据，脑图等图形视图由 Web/桌面前端按 kind 做成视图插件，不进 core 主路径。

## 扩展机制与 Action 分级

- **展示扩展** = 新的 main package 依赖 `core`。
- **能力扩展** = core 定义接口：`Action` / `SyncProvider` / `Cipher`。
- **动态加载暂缓**；执行安全：外部来源 `@shell` 必须显式确认。

### Action 权限分级（延续 v9/v10 定稿）

| 动作 | 权限 | 跨平台 | 定位 |
|------|------|--------|------|
| `Copy` 复制 | 无 | 一致 | 基础动作 |
| `Preview` 打开自家渲染视图 | 无 | 一致 | 主力 |
| `Open` 按类型调起系统应用 | 无 | 一致 | 主力 |
| `Reveal` 在文件管理器显示 | 无 | 一致 | 辅助 |
| `Execute` 调起终端执行 `@shell` | 无特殊权限 | 可行 | 可选（需安全确认） |
| `Insert` 回填到前台应用 | macOS 需 Accessibility；Wayland 基本不可行 | 最差 | 平台可选，最后做 |

- 默认只保证 `Copy` + `Preview`/`Open`（三平台一致）；「打开」走系统 IPC，不碰模拟输入。
- Launcher v1 只做 `Copy` + `Preview`/`Open`，不碰 `Insert`。

## 目录布局

```
zoro/                         # 本仓库（框架）
├── go.mod                    # module zoro（core 是公开包，供 ext 依赖）
├── core/                     # 核心库（纯 Go，无 UI / 无 fzf / 无网络）
│   ├── block.go registry.go scan.go meta.go config.go
│   ├── index.go query.go render.go library.go workspace.go
│   └── *_test.go
├── cmd/zoro/                 # CLI 前端（main package）
├── ext/
│   └── zoro-launcher/        # Wails 桌面 Launcher（独立 go.mod，replace 引用 core）
│       ├── main.go app.go wails.json
│       └── frontend/dist/    # 自绘浮窗（原生 HTML/JS，不用 fzf）
├── tests/fixtures/  tests/fixtures2/
├── examples/knowledge-base/  # 示例工作区
├── scripts/verify.sh
├── README.md  agents.md  todos.md
└── kb/                       # 本项目知识库（本文件所在目录）
```

内容库侧（示例）：

```
<workspace>/
├── zoro.toml                # 工作区声明（唯一入口，进 git）
├── sanji/
│   ├── .zoro/meta.json      # 库级元数据（派生物，gitignore）
│   ├── zoro-index.tsv       # 可读视图（可选分发）
│   └── ...任意 *.md
└── robin/ ...
```

## 技术选型速览

完整选择与理由见 `facts/decisions.md`；此处只放映射表：

| 能力 | 选择 |
|------|------|
| Markdown 解析 | goldmark + GFM |
| 语法高亮 | chroma（经 goldmark-highlighting） |
| 元数据序列化 | encoding/json（schema 版本化） |
| 工作区配置 | pelletier/go-toml/v2（未知键保留） |
| 核心模糊匹配 | core 内 Matcher；后续可接 nucleo（纯匹配无 UI） |
| CLI 交互 | fzf（前端捆绑，不进 core） |
| 本地 Web | net/http（P2） |
| 桌面 Launcher | Wails v2 + 自绘浮窗 |
| 全文搜索 | bleve / zinc（P4 再定） |
| 站点搜索 | pagefind（P5） |
| 同步 | git 首发，再 object_store/WebDAV（P6） |
| 加密 | age（P7） |
