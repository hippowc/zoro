# 可视化块（脑图这类）：存储面 vs 编辑面，以及 codec 该放哪（设计方案，2026-09-09）

> 回答的问题（用户原话）：「譬如脑图这种扩展，**是否一定要文本，是否各种操作可以不需要文本**，业界有什么更好的实践吗？另外我们这个工具基础能力是对 markdown 文件的增删改查，核心优势是**更精准的增删改查**……另外是对一些**特殊片段的操作能力**：譬如对脑图的可视化操作，譬如执行 shell 脚本等等，请整体考虑方案。」
>
> 本文只沉淀方案，不含代码。落地追踪见 `../../todos.md` 9.10 / 9.11 与 `../facts/decisions.md` **AD-18**。
> 前置阅读：`2026-09-09-launcher-command-surface-and-view-extension-plan.md`（§6 视图注册表、§7 脑图样板、§9 决策点）。**本文 §9 记录了那份文档已被实现推翻的 7 处，照抄前先读。**
> 📖 **引用约定**：下文裸写 `§N` 指**本文**；写 `方案文档 §N` 指上面那份 command-surface 方案。

---

## 0. 结论摘要

**「存储必须是文本」与「操作必须敲文本」是两个问题，答案相反。**

- **存储面永远是纯文本 Markdown**（AD-4 不动摇）。一旦落成二进制/私有数据库，grep、git diff、静态站点、别的编辑器全部失效，用户被锁死。
- **编辑面可以完全不需要文本**。用户在脑图上拖节点、在画布上画框，一个字都不敲——但这些操作最终由 **codec** 序列化回规范文本。

三层分离（本文的核心结论）：

```
存储面  Storage   ← 永远是 .md 纯文本；唯一事实源；git 可 diff
  ↑↓  codec（文本 ⇄ 结构化对象；按 kind 注册）
编辑面  View      ← 可以是可视化控件（脑图/画布/表格），也可以是 textarea
  ↑↓  actions（按 AD-8 分级：Copy/Open/Preview/Execute/Insert）
```

**推论：codec 是可选的。** 如果编辑面本身就是文本（textarea + 实时预览），codec 直接是恒等函数——脑图的第一阶段就该这么做（§4 阶段 A），因为它顺带消灭了「序列化幂等」这个最难的风险。

---

## 1. 把「一定要文本吗」拆成三个问题

用户的疑问混了三件事，分开答才不会做错决定：

| # | 问题 | 答案 | 依据 |
|---|------|------|------|
| Q1 | **落盘格式**必须是文本吗？ | **必须** | AD-4「Markdown 唯一源」。非文本 = 第二个内容源 = 同步/检索/diff/导出全部要重做一遍 |
| Q2 | **用户操作**必须经过文本吗？ | **不必** | 编辑面是 UI 问题，与存储契约正交。Excalidraw/markmap 都是「可视化操作 → 文本落盘」 |
| Q3 | **core 必须理解**每种块的结构吗？ | **必须不** | core 只提供「取块文本 / 整块替换 / 静态渲染」三件通用能力；kind 专属结构留在前端 view（§3）。否则每加一种可视化块都要动三个前端共享的地基 |

Q3 是本文最容易被做错的一条，单独展开在 §6（否决「节点级 API 进 core」）。

---

## 2. 业界实践调研

> 标注 ✅ 的是本次**已核验**的（npm registry / 官方仓库）；标 ⚠️ 的是据既有知识、实现前请再确认一次。

### 2.1 正面样板：把结构化数据塞进合法 Markdown

| 实践 | 做法 | 对我们的启示 |
|------|------|------------|
| **markmap**（`markmap-lib` / `markmap-view` / `markmap-toolbar`）✅ | **MIT**，当前 `0.18.12`。把 Markdown **嵌套列表/标题**直接渲染成交互式思维导图（折叠、缩放、平移）。npm 上**没有 editor 包**——它是**纯可视化**，编辑仍在文本侧。`markmap-view` 依赖 `d3 ^7.8.5` + `@babel/runtime` | ① 「文本本身就是脑图的规范表示」这条路是成熟且零转换成本的；② **它不给拖拽改结构**，要拖拽得自己在 d3 SVG 上写（§4 阶段 B）；③ d3 是个不小的依赖，vendor 前要量体积 |
| **Mermaid `mindmap`** ⚠️ | 文本 DSL → 图，GitHub / Obsidian / VS Code 原生渲染 | **单向**：图不能拖回文本。只适合「只读展示」，不适合「可视化编辑」。我们要的是后者，所以不选它当主路径（但可作为静态站点侧的降级渲染） |
| **obsidian-excalidraw-plugin 的 `.excalidraw.md`** ⚠️ | 文件**是真正的 Markdown**：里面有 `# Text Elements`、`# Drawing` 等小节，绘图 JSON 放在一个 fenced code block 里。于是同一个文件既能被 Excalidraw 当画布打开，又能被任何 Markdown 工具读/搜/diff | **这是「存储是文本、编辑面是画布」最干净的样板**，也是我们将来的自由画布该抄的形状（§5） |
| **Logseq / Roam** ⚠️ | 大纲即块，块级增删改与拖拽重排，存储仍是 markdown / org 纯文本 | 证明「**块级精准增删改查 + 大纲式编辑面**」是成熟范式——正是用户说的「核心优势」那条路 |

### 2.2 反面样板：脱离文本的代价

| 实践 | 代价 |
|------|------|
| **Obsidian Canvas** ⚠️ | 用独立 `.canvas`（JSON）文件，**不是 .md**。结果：Canvas 内容进不了 Markdown 的搜索/嵌入/静态站点管线，成了知识库里的一块飞地 |
| **Notion** ⚠️ | 私有数据库为源，Markdown 导出**有损**（数据库视图、同步块、评论全丢）。用户被锁死，迁移即降级 |

→ 这两条是 AD-4 的实证支持：**只要我们还想让用户的库被别的工具消费，就不能有第二种落盘格式。**

### 2.3 协同编辑：明确不引入

**Yjs / Automerge（CRDT）** ⚠️ 是协同编辑的业界标准，但**否决**，三条理由：

1. 我们是**单机工具**，并发方是「Launcher + CLI + 用户的编辑器」，不是多用户实时协同；
2. CRDT 需要在文件旁存元数据（或把元数据编进内容），会让「**文件字节 == 用户内容**」这个不变量失效——直接违反 AD-4；
3. 我们已经有更便宜的答案：**整块替换 + `expect` 前置条件**（AD-15 乐观并发），冲突时让用户重查一次。块通常几十行，重写成本可忽略。

---

## 3. 架构：`kind → { codec, view, actions }`

每种「特殊片段」= 三元组注册项。这与**方案文档 §6** 的视图注册表（todos 9.10）是同一张表的三列：

```js
// 前端 zoroViews 注册表（示意；真实形状见方案文档 §6.2）
zoroViews.register({
  id:        "mindmap",
  kind:      "mindmap",          // 对应 @mindmap 标签
  title:     "脑图",
  maxHeight: 540,                // 窗口高度上限由视图声明（方案文档 §6.4），否则脑图被压扁

  // codec：文本 ⇄ 结构。**恒等 codec 是合法的**（阶段 A 就是恒等）
  decode: function (rawText) { return tree; },
  encode: function (tree)    { return rawText; },   // ⚠️ 必须幂等且字节稳定

  component: function (ctx) { /* 可视化控件 or textarea */ },

  // actions：按 AD-8 分级，每个动作一个 bridge 方法
  actions: [
    { id: "copy",   label: "复制 Markdown", level: "default" },
    { id: "save",   label: "保存",          level: "write"    },  // → UpdateBlock
    { id: "open",   label: "打开源文件",     level: "default" },
  ],
});
```

**core 侧对应的只有三个通用能力，永远不因 kind 增加而改动：**

| core 能力 | 签名 | 作用 |
|-----------|------|------|
| 取块文本 | `LoadRaw(library, path, start)` | codec 的输入 |
| 整块替换 | `UpdateBlock(path, start, expect, BlockPatch{Terms, Body})` | codec 的输出落盘；`expect` 做乐观并发 |
| 静态兜底渲染 | `RenderMarkdownHTML(raw)` | 嵌套列表**零改动**就能渲染成 `<ul>`——CLI / 静态站点 / 不支持该 view 的前端白拿一个降级展示 |

> 这条边界就是 Q3 的答案：**core 不认识 `MindmapNode`，也不认识画布坐标。**

---

## 4. 脑图的具体方案（分两阶段，先做 A）

内容形态已定（D5，见**方案文档 §7** 样板与 AD-18）：`@mindmap <搜索词>` + **Markdown 嵌套列表**负载。

```markdown
## 定投计划

@mindmap 定投 计划 复盘
- 定投
  - 标的：沪深300
  - 频率：每月
- 止盈
  - 目标收益率：30%
```

两个**已核实的坑**决定了只能用列表（不能用 4 空格缩进大纲，也不能用 JSON blob）：
- 4 空格缩进在 goldmark 里是**代码块** → 渲染成 `<pre>`，树结构全丢；
- 缩进的 `@xxx` 行会被 `StripDirectives` 从渲染输出里**吃掉**（`core/render.go`）→ 节点名不能以 `@` 开头。

### 阶段 A（便宜，先做这个）：**文本即编辑面 + 实时脑图预览**

- 左：textarea（或代码编辑区），右：markmap 渲染的 SVG，输入即刷新。
- **codec 是恒等函数** → 于是：
  - ✅ **不存在序列化器**，「幂等 / 字节稳定」这个最难的验收标准**自动满足**；
  - ✅ **不产生 git diff 噪声**（用户改了什么字节就是什么字节）；
  - ✅ 用户手写、CLI 渲染、静态站点、markmap 可视化**四方共用同一份文本**，没有任何一方是「派生副本」。
- 代价：改结构要靠改缩进，不如拖拽直观。但**缩进列表本身就是一种大纲编辑面**（Logseq/Roam 用户天天这么干），可接受。
- 依赖：vendor `markmap-lib` + `markmap-view` + `d3`（MIT，`0.18.12`）。⚠️ **离线**运行，绝不走 CDN（与 Alpine 同样的纪律：vendor 进 `frontend/dist/`，`package.json` 里不出现「会被打包」的暗示）。落地前先量 d3 的体积。

### 阶段 B（贵，等 A 用顺了再决定要不要做）：**拖拽节点改结构**

- 这时才真正需要 codec：`decode(text) → tree`、`encode(tree) → text`。
- **硬验收标准：`encode(decode(t))` 对同一棵树必须逐字节相同**，且 `decode(encode(tree))` 结构等价。做不到就不要上线——否则每次保存都在用户的 git 历史里制造无意义 diff，这比没有拖拽功能糟糕得多。
- 实现要点：markmap 不提供节点拖拽（§2.1 已核验），要么在它的 d3 SVG 上自己加拖拽 handler，要么换/补一个大纲树控件。**先评估「在 textarea 里做 Tab/Shift-Tab 缩进 + 上下移动行」是否已经够用**——那是阶段 A 的增量，成本远低于拖拽，且同样不需要 codec。

> 建议：**A 上线 → 用一周 → 再决定 B**。很可能 Tab 缩进就够了，那就永远不需要写序列化器。

---

## 5. 自由画布（手绘/框图）：将来抄 Excalidraw 的形状

如果以后要支持「画布」类块（不是脑图那种树，而是自由坐标），**不要发明新文件格式**，照 `.excalidraw.md` 的做法：

````markdown
## 架构草图

@canvas 架构 部署
```zoro-canvas
{"type":"excalidraw","elements":[...],"appState":{}}
```
````

- 文件仍是**合法 Markdown**：fenced code block 是标准语法；
- `RenderMarkdownHTML` 零改动就给出降级展示（一个代码块，内容是 JSON——不好看但**不丢数据**，且能被搜到 `@canvas` 的 terms）；
- codec = `JSON.parse` / `JSON.stringify`，**幂等性由 JSON.stringify 的确定性保证**（键序固定即可），比自造文本格式容易得多；
- ⚠️ 代价：这一块**不可 diff**（JSON 一行长字符串）。缓解：`JSON.stringify(obj, null, 2)` 缩进输出，让 diff 至少能定位到元素级。
- ✅ **已核验的前提**：`StripDirectives`（`core/render.go`）在 fenced code block 内**不做指令剥离**（遇到 ``` 就进 `inFence` 原样输出）。所以 fence 里的 JSON 不会被当指令吃掉——这正是 Excalidraw 式方案能成立的原因，也是「缩进的 `@` 行会被吃掉」那个坑只在 fence 外生效的原因。
- 优先级：**P5+，现在不做**。脑图（树）能覆盖绝大多数「结构化可视化」需求，画布的复杂度（撤销栈、选中态、坐标系、导出 PNG）是另一个量级。

---

## 6. 否决项（每条都写清理由，防止将来重新提案）

| 提案 | 否决理由 |
|------|---------|
| **节点级操作进 core**（如 `core.UpdateMindmapNode(lib, path, start, nodeID, …)`） | ① core 会因此认识每种 kind 的内部结构，违反「core 不放 UI/视图概念」（方案文档 §8 禁止清单 + F-9）；② 节点级 patch 要求 core 理解嵌套列表的缩进语义，而缩进语义正是 codec 的职责——**两处定义必然漂移**；③ 收益不存在：整块替换 + `expect` 已是乐观并发的最小可用版本（AD-15），块只有几十行 |
| **JSON blob 存脑图**（`@mindmap` + 一行 JSON） | 不可 diff、合并敌对、用户无法手写、`RenderMarkdownHTML` 给不出任何有意义的降级展示 |
| **4 空格缩进大纲**（不用列表标记） | goldmark 当代码块 → 树结构全丢（§4 已核实） |
| **CRDT（Yjs / Automerge）** | 单机工具 + git 已是并发控制；CRDT 会让「文件字节 == 用户内容」失效，违反 AD-4（§2.3） |
| **私有二进制/SQLite 存内容** | 造第二内容源；用户的库不再能被别的工具消费（Notion 的教训，§2.2） |
| **`.canvas` 式独立格式文件** | 同上：飞地。要结构化数据就用 fenced code block 塞进 .md（§5） |
| **Mermaid `mindmap` 当主路径** | 单向渲染，图不能拖回文本；只能当只读降级展示 |
| **core 里加 `View` / `Codec` 概念** | 三个前端（CLI / Web / Launcher）的展示能力天差地别，core 一旦有 view 概念就会被最弱的那个拖着走。core 只出 `target`（html/ansi/text），`kind → view` 永远在前端（方案文档 §6.1 架构律） |

---

## 7. 「精准增删改查 + 特殊片段操作」能力矩阵（当前真实状态）

用户说的核心优势，逐条对齐到代码现状——**这张表就是接下来的排期依据**：

| 能力 | core API | 前端入口 | 状态 |
|------|----------|---------|------|
| 精准查某片段 | `Query` + 三面（index/title/path） | 搜索框 | ✅ 已上线 |
| 定位到块 | 三元组 `(库, path, start)` + `LoadRaw` | 预览 / 详情模式 | ✅ 已上线 |
| **随意添加一个片段** | `AppendBlock(BlockDraft)`（落点 `CaptureFile()`） | `/new [kind]` 捕获面板 | core ✅ / **UI ⬜**（todos 9.9） |
| 改片段 | `UpdateBlock(path, start, expect, patch)` | 详情模式内编辑 | core ✅ / **UI ⬜**（todos 9.10） |
| 删片段 | `PreviewDelete` + `DeleteBlock` | 二次确认行 | core ✅ / **UI ⬜**（todos 9.10） |
| 脑图可视化 | **不需要 core 改动**（§3） | view 注册表 + markmap | ⬜（todos 9.11，先做阶段 A） |
| 执行 shell 脚本 | `allow_exec` helper 已就位 | Action 分级（AD-8：`Execute` 必须确认） | ⬜（todos #8，P3） |
| 加库 | `core.AddLibrary`（CLI 与 Launcher 同一条链） | `zoro add` / `/lib add` | ✅ 已上线 |

**关键观察：core 侧的写能力已经全部就绪，剩下的全是前端。** 所以 §4 阶段 A 之前，应该先把 9.9（捕获 UI）做完——它让 Launcher 从「只读搜索器」变成「能往里写东西的入口」，而且是脑图 `/new mindmap` 的前置。

### 特殊片段的操作能力 = actions 分级（对齐 AD-8）

「对特殊片段的操作能力」不是每个 kind 各写一套按钮，而是**统一分级 + 按 kind 注册可用集**：

| level | 含义 | 例子 | 约束 |
|-------|------|------|------|
| `default` | 只读，随手可做 | Copy / Open / Preview | 无需确认 |
| `write` | 改用户内容 | Save（`UpdateBlock`）/ Delete（`DeleteBlock`） | **必须过确认行**（AD-17 不变量 ③）；删除前先 `PreviewDelete` 给用户看会删掉哪些行 |
| `execute` | 跑用户代码 | `@shell` 执行 | **必须** `allow_exec` 为真 **且** 二次确认（AD-8）；库级开关，默认关 |
| `insert` | 回填到别的应用 | 把片段插进当前光标 | 平台可选，需要 Accessibility 权限，**尚未做**（AD-8） |

> `write` 级动作之后**必须整体重查**（`resetResults()` + `doQuery()`），禁止局部 patch 手里的候选列表——`start` 是行号，写操作会让它漂移（写定律 L1）。

---

## 8. 实施顺序（每步可独立验证）

1. **9.9 捕获 UI**（`/new`）：core 已就绪，纯前端。验收：`/new` → 面板 → 提交 → **立刻能搜到**。
2. **9.10 view 注册表**：先把现有 markdown 预览改造成 `zoroViews.markdown`，**行为完全不变**（jsdom 冒烟测试守住，见 `2026-09-09-launcher-frontend-jsdom-verify.md`）。
3. **9.10 CRUD 三件套**：把 `UpdateBlock` / `PreviewDelete` + `DeleteBlock` 接到详情模式；删除走确认行。
4. **9.11 脑图阶段 A**：登记 `KindMindmap`（**三处**：`core/block.go` 的常量组 + `core/registry.go` 的 `ClassifyTag` switch + `core/block.go` 的 `TagKindFromString` 白名单）→ vendor markmap → `zoroViews.mindmap`（**恒等 codec** + textarea + 实时 SVG）。⚠️ 不登记也能用（未登记 tag 降级成 `unknown:mindmap`，照常检索），但 title 降级、faces、校验会不一致，所以推荐登记。
5. **用一周，再决定阶段 B**（拖拽 / Tab 缩进增强）。很可能不需要。

> 第 1、2、3、4 步的 core/JS 逻辑都能在 Linux 侧用 jsdom 验证；markmap 的 SVG 渲染效果与窗口高度需 Mac 实测（走 `../skills/ship-launcher-release.md`）。

---

## 9. 与 2026-09-09 设计文档的实现分歧（⚠️ 照抄前必读）

那份文档写于实现之前，以下 **7 处已被落地代码推翻**。以代码为准：

| # | 文档原文 | 实际落地 |
|---|---------|---------|
| 1 | §4.4 第 6 步：`a.ws.Close()` 再 `LoadWorkspace`，「不 Close = 自锁死（F-5）」 | **已过时**。P-13 修复后 `Library` 从不跨调用持有 store（唯一入口 `withStore(fn)`），旧 workspace 没有需要 Close 的东西，直接 `a.ws, a.wsErr = ws, nil` 即可（`ext/zoro-launcher/app.go` 的 `AddLibrary` 注释里写了原因） |
| 2 | §4.4 前端 bridge 名 `PickLibraryDir()` | 实际是 **`PickDirectory()`** |
| 3 | §4.4 让 Launcher「逐步对齐 F-3」自己实现加库 | 实际收拢成 **`core.AddLibrary`** 一条链，CLI `zoro add` 与 `/lib add` 走同一个函数（AD-16） |
| 4 | §6.3 写 API 返回 `WriteResult` + **`InvalidatedPaths []string`** | **没有 `InvalidatedPaths`**。约定改为「写后前端整体重查」（写定律 L1），失效列表是多余状态 |
| 5 | §6.3 写前用 **`SliceBlock`** 取块校验 | **不存在 `SliceBlock`**。实际是 `mutateBlock` + `resolveContentPath` + `expect` 落盘前后各自校验（`core/write.go`） |
| 6 | 未提及 | **新增 `PreviewDelete`**：删除前返回「将会删掉哪些行」给用户看。删除是 `write` 级动作，必须先可见再确认 |
| 7 | §7 / D5 只说「删/改的范围」 | 落地时明确了 **L2**：`UpdateBlock` **不碰**块上方的 `## 标题`（块身份从 `@` 行开始），而 `DeleteBlock` **连块自己的 `## 标题` 一起摘掉**（只留标题会变孤儿），但**绝不**碰下一个块的标题 |

另外文档 §9 的 D1–D6 现状：**D1/D2/D3/D4/D6 已拍板并落地**（AD-16 / AD-17 / P-13 修法），**D5 由本文拍板**（AD-18）。

---

## 10. 落地追踪

- `../../todos.md`：**9.9**（捕获 UI，core 已就绪）、**9.10**（view 注册表 + CRUD）、**9.11**（脑图，本文 §4 阶段 A）
- `../facts/decisions.md`：**AD-18**（存储面/编辑面分离 + `kind → {codec, view, actions}` + 否决节点级 core API；D5 已定）
- `../facts/architecture.md`：写入层（`core/write.go`，三条写定律 L1–L3）、前端分工（core 出 `target`，`kind → view` 在前端）
- `../facts/pitfalls.md`：P-13（store 锁，已修）、P-15（命令面空面板）；脑图落地后预计新增「序列化不幂等 → git diff 噪声」与「vendor d3 体积」两条
