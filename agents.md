# agents.md — zoro 通用知识库框架 · 开发指南（v5 定稿）

> 本文件是给「后续负责迭代开发的 agent / 人」读的**单一事实源**。
> 任何 agent 开始本仓库工作前，先读本文件；改动行为前先改本文件。
>
> 区分两个概念：
> - **运行时**：本地优先；一切网络能力均为可选插件，关掉即纯本地。
> - **开发期**：欢迎 agent 参与，但必须遵守下文"不改的设计"。

---

## 1. 定位与愿景

**zoro**：本地优先、可扩展的通用知识库框架。

- 核心 = 一个 Rust library（`zoro-core`）+ 一个最小 CLI 前端（`zoro`）。
- **非终端优先**：CLI 与 Web 是平级前端——Web（`zoro serve`）是大众入口，CLI fzf 是效率入口，终端不是唯一形态。
- 愿景四步走：**标签体系 → Web 体验 → 站点/云/安全**（见 §11 产品路线图）。
- 内容源**只有一种**：Markdown（+ `@` 指令）。HTML 是渲染产物，不是源码。

## 2. 核心边界（最重要的口径）

```
zoro-core（library，无 UI / 无 fzf / 无网络）
 ├─ 内容模型：Entry / Directive / Library
 ├─ 解析：@index/@cmd 标记与分块
 ├─ 索引：四列 TSV + 全文索引（tantivy）
 ├─ 查询：Matcher trait（默认 nucleo）+ 候选模型
 ├─ 渲染：Markdown → AST/HTML/ANSI（多 target）
 └─ trait 扩展点：Render / Action / SyncProvider / Cipher
```

- **core 不依赖任何 UI**；fzf、Web、桌面、SSG 都是消费 core 的前端。
- 依赖方向永远单向：`frontend → core`，绝不反向。
- 能力扩展走 trait；展示扩展 = 新 binary 依赖 `zoro-core`。

## 3. 原则（不可违背）

1. **目录自由，内容受限**：内容目录零约束；唯一技术约束在「`@` 标记行」格式上（§4）。
2. **Markdown 唯一源**：作者只写 Markdown + `@` 指令（含标签式组件）；HTML/CSS 属于产物与样式层。
3. **两个技术要素，单行标记（v1）**：`@index`（值型，必须）、`@cmd`（结构型，可选）。
4. **唯一强制物是索引**：扁平 TSV，四列 `title index start path`（tag 待定后扩列）。

## 4. 条目格式（唯一内容契约 v1）

```markdown
## Git 丢弃本地修改（可读性标题，可选）

@index git checkout reset 丢弃 还原

@cmd
```bash
git checkout -- <file>
git reset --hard <commit>
```

正文说明。
```

### 4.1 指令分三形态（统一标签体系）

| 形态 | 内容从哪来 | 示例 |
|------|-----------|------|
| 值型 | 值本身 | `@index git checkout 丢弃` |
| 结构型 | 紧随的 Markdown 原生结构（fenced code block） | `@cmd` |
| 块组件型 | 紧随的 4 空格缩进块 | `@card title=概览` |

### 4.2 `@index`（值型，必须）

- 每条条目恰好一条，同时是**条目锚点**和**检索命中面**。
- 行首顶格 `^@index `，后跟至少一个词。

### 4.3 `@cmd`（结构型，可选）

- `@cmd` 后（跳过空行）必须紧跟 fenced code block；块内每行是该 `@cmd` 的负载。
- 语言取自 fence info string；一个 `@cmd` 绑一个块；无块则**警告并忽略**。
- v1：`@cmd` 被识别、校验、随正文展示，不提供复制/执行（§11 P3）。

### 4.4 边界规则（现场推导，不存 end）

- **条目边界 = `@index` 行 → 下一个 `@index` 行 / 文件末尾**；`##` 不参与边界。

### 4.5 标题（title）规则

- `##` 紧贴 `@index` 上方；索引取"`@index` 向上最近的 `##`"作 `title`；无 `##` 取 `@index` 首词。

## 5. 索引（唯一强制物）

**四列 TSV：`title index start path`**（字段约束同 v4：单行、不含 `\t`）。

### 5.1 分块算法

逐行扫描；分块单位 = 条目。`@index` 开新条目、`@cmd` 记待绑定、fence 开==绑定、`##` 记最近标题、其余为正文。

### 5.2 生成与失效（预构建 + 脏检查）

- 内容根下 `zoro-index.tsv`（随内容库分发）；查询前 mtime 脏检查，变更才重建。
- 原子写：临时文件 + `mv`。

### 5.3 扫描范围

- 单内容根 `ZORO_ROOT`；只扫 `*.md` / `*.mdx`；跳过隐藏目录；多根为 future。

## 6. 检索与匹配（fzf 属于前端，不在 core）

- **core**：`Matcher trait`（默认 `nucleo`）+ 候选模型 `Candidate{ title, index, path, start, match_ranges }`。
- **CLI 前端**：把全量 candidates 交给 **fzf** 交互（fzf 是该前端的默认控件，随前端捆绑）。
- **Web/桌面前端**：调用 `core.query(q)`，自绘输入框/列表/高亮，不用 fzf。

fzf 三分离语义必须沉淀进 core 模型：**匹配字段（index）≠ 显示字段（title）≠ 载荷（path+start 即时预览）**。CLI 用 fzf 实现它们，桌面端同样按此模型实现。

## 7. 渲染管线与标签组件（多 target）

### 7.1 管线

```
Markdown 源
 ├─ 预处理：剥离 @指令（进索引，不进 AST）
 ├─ pulldown-cmark 解析 → 事件流
 │   ├─ 普通块 → 标准 HTML
 │   ├─ fence + 已知语言 → syntect 高亮
 │   ├─ fence + mermaid/KaTeX → 前端渲染容器
 │   ├─ 标签组件(@card 等) → 组件注册表 → 自定义 HTML
 │   └─ raw HTML → sanitizer → 放行
 └─ 输出 target：
     ├─ Web/桌面/SSG → HTML 文档
     ├─ CLI/TUI → ANSI
     └─ 非 TTY → 纯文本
```

### 7.2 标签式组件（统一 `@` 语法，不换源）

```markdown
@card title=概览 size=large
    组件正文（可继续是 Markdown），
    用 4 空格缩进承载。
```

- 行首 `@name key=value...` 为组件声明：`@name` 是组件类型，`key=value` 为 props；**组件正文 = 紧随的 4 空格缩进块**。
- 渲染器维护**组件注册表**；未知组件名**安全降级**：内容原样保留、不渲染成组件（前向兼容）。
- 声明层（Markdown）与呈现层解耦：同一个 `@card`，CLI 渲染成框线，Web 渲染成 Tailwind 卡片，桌面渲染成 React 组件。组件可用任意前端框架（React/Vue/Svelte）+ CSS 框架（Tailwind 等）实现，属于**呈现层自由**。
- fenced code block 三反引号回归唯一职责：**表示真代码**（`@cmd` 绑的就是它）。

### 7.3 样式与内嵌 HTML

- 样式 100% 由 CSS 主题层决定（HTML 产物 + CSS 无上限），与源码无关。
- 一次性样式可用 Markdown 内嵌 HTML（`<details>` 等），但必须过 **sanitizer**（白名单标签/属性）。
- 内容为不可信输入（云同步/他人分享）时，关 raw HTML 透传或严格 sanitize，防 XSS。

### 7.4 表达力边界与逃生门

标签的表现力是**有限且故意有限**的，不追求覆盖全量 HTML。分层如下：

| 层 | 能力 | 边界 |
|----|------|------|
| L0 样式（CSS） | 外观/颜色/动画 | **无上限**——产物是 HTML，CSS 随便做 |
| L1 行内排版 | 粗体/斜体/代码/链接/图片 + `==高亮==`/KaTeX/上下标 | 有限；缺口用**内嵌 HTML span** 兜底 |
| L2 块级结构 | Markdown 块 + `@` 标签组件 | 主力，覆盖知识文档常见结构 |
| L3 精确/极端结构 | 一次性样式 | 内嵌 HTML（过 sanitizer） |
| L4 高度交互/应用 | 复杂交互界面 | **交给呈现层框架**，不在标签职责内 |

设计立场：

> 标签目标是「知识文档的表达力」，不是「任意网页的表达力」。
> 样式无上限靠 CSS；结构够用靠标签 + Markdown；行内缺口靠内嵌 HTML；复杂应用交给呈现层。

- `@` 标签是**块级**语法，不承担行内排版；行内需求由「少量预定义内联扩展 + 内嵌 HTML span」解决。
- 绝不为了让标签吃掉全部 HTML 而退化成模板语言（那会毁掉低门槛）。

## 8. 扩展机制

- **展示扩展** = 新 binary 依赖 `zoro-core`（无插件系统）。
- **能力扩展** = core 定义 trait：`Action` / `SyncProvider` / `Cipher`；内置最小实现，第三方按 crate 加。
- **动态加载（libloading/wasmtime）暂缓**：无第三方不开源场景前不做。
- 执行安全：来自外部库的 `@cmd` 执行必须显式确认；打开/编辑/执行均为 `ActionRegistry` 分发。

## 9. 目录布局（workspace）

```
zoro/
├── Cargo.toml              # workspace
├── crates/
│   ├── zoro-core/          # library：模型/解析/索引/查询/渲染管线/trait
│   └── zoro/               # thin CLI frontend（捆绑 fzf）
├── ext/                    # 扩展（可独立 repo 或本仓库 workspace）
│   ├── zoro-server/        # P2 本地 Web
│   ├── zoro-desktop/       # Tauri 桌面壳
│   └── zoro-publish/       # P5 静态站点导出
├── assets/fzf-<platform>   # 仅 CLI 前端捆绑
├── tests/fixtures/
├── README.md
└── agents.md
```

## 10. 技术选型（Rust 生态）

| 能力 | 选择 |
|------|------|
| Markdown 解析 | `pulldown-cmark` + GFM 扩展 |
| 语法高亮 | `syntect` |
| 核心模糊匹配 | `nucleo`（纯 matcher） |
| CLI 交互 | `fzf`（前端捆绑，不进 core） |
| 全文搜索 | `tantivy`（P4） |
| 本地 Web | `axum`/`warp`（P2，ext） |
| 桌面壳 | `tauri`（ext） |
| 站点搜索 | `pagefind`（P5，ext） |
| 同步 | git 首发，再 `object_store`/WebDAV（P6，ext） |
| 加密 | `age`/`rage`（P7，ext） |

## 11. 产品路线图（归属标注）

| 阶段 | 交付 | 归属 |
|------|------|------|
| P0 | **标签体系**：`@index/@cmd/@card` 解析 + 索引（无 UI） | core |
| P1 | **渲染管线**：Markdown→HTML/ANSI（多 target） | core |
| P2 | **本地 Web**：`zoro serve` 浏览器体验（大众入口） | ext/zoro-server |
| P3 | **CLI 前端**：fzf 浏览 + `@cmd` 复制/执行（效率入口） | zoro |
| P4 | 全文搜索 tantivy | core |
| P5 | 静态站点发布 | ext/zoro-publish |
| P6 | 同步插件（git 首发） | core trait + ext |
| P7 | age 整库加密 + 密钥授权 | core trait + ext |

- core 到 **P1** 后基本冻结（标签 + 渲染是共同地基）；P2 起 CLI 与 Web 是**平级前端**，非终端用户走 Web。
- P2 与 P3 可并行；Web 是大众入口，CLI 是效率入口。

## 12. 待定项（backlog）

### tag（重要，方向未定）

暂不实现 `@tag`。初步判断 tag 是"范围"概念，不属于正文，候选：目录名=tag / 构建期配置 / 目录级继承。确定后索引四列可扩五列，不影响既有数据。

### 已推翻 / 可省

- `@run`：废除（行为外移为运行时扩展点）。
- `@lang`：可省（fence info string 已带语言）。

### 扩展指令占名（不实现）

`@alias`（别名）、`@desc`（摘要）、`@ref`（跳转）、`@hidden`（不进索引）。

## 13. 开发约定

- **不改设计**：§2–§8 的改动都是设计变更，先改本文件再动代码。
- **本地优先**：关掉网络/云/发布能力 = 纯本地工具；任何网络能力都是可选插件。
- **性能目标**：未变更进入交互延迟近零；变更重建百毫秒级（千级条目）。
- **索引是派生物**：随时可重建，但作为分发产物入库。
- **前向兼容**：未知顶格 `@xxx` 不报错、不影响分块；未知组件标签安全降级（内容原样）；未知 fence 语言当普通代码块。
- **安全默认拒绝**：外部来源内容默认不执行、sanitize、显式确认。
- **不把标记变成编程语言**：指令只做分类/声明，逻辑交给运行时扩展点。
- **测试轻量**：Rust `tests/` + 文本夹具，不引入额外框架。

## 14. 验收标准（P0–P3 按阶段）

- [ ] 【P3】`zoro` 无参进入 fzf 全量浏览；候选区显示标题。
- [ ] 【P3】`zoro 初始词` 预填 query，可改写/清空。
- [ ] 连续两条 `@index` 条目展示正确、无粘连；条目到 EOF 不越界。
- [ ] `title`：有 `##` 取 `##`，无则取 index 首词。
- [ ] `@cmd` 正确绑定紧随 fence；无 fence 警告忽略、不崩溃。
- [ ] 未变更直接读索引；某 `.md` 更新后自动重建。
- [ ] 【P3】CLI 缺 fzf 报错或走非交互输出；无 TTY 时 `zoro | less` 可用。
- [ ] 【P3】`--preview` 随选随显；回车全屏展示。
- [ ] 【P3】单二进制（捆绑 fzf）在新机器零安装跑通全链路。
- [ ] `zoro-core` 无 fzf / 无 UI 依赖（`cargo tree` 可验证）。
- [ ] 【P2】`zoro serve` 在浏览器完成浏览/搜索/渲染，全程无需终端。
