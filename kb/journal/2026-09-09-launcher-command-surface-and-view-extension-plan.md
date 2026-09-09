# Launcher 能力唤起面 + 面板可扩展架构（设计方案，2026-09-09）

**来源任务**: 用户提出的三个设计问题（添加库不手写 toml / 自由新增知识片段 / 结果与预览面板可扩展到脑图这类 CRUD 视图）
**日期**: 2026-09-09
**状态**: **已部分落地，且部分被实现推翻**（2026-09-09 同日）。D1–D6 全部拍板：AD-15 / AD-16 / AD-17 / **AD-18** + P-13 修法（清单见 `../facts/decisions.md` 待定项节）。
> ⚠️ **§4.4 / §6.3 / §7 有 7 处与代码不符，照抄会写错**（例如第 6 步的 `a.ws.Close()` 在 P-13 修复后已无必要、`InvalidatedPaths` 与 `SliceBlock` 根本不存在）。动手前先读对照表：`2026-09-09-visual-blocks-storage-vs-editing-surface.md` **§9**。
> ✅ 仍然权威、可以直接照抄的是：**§2 已核实的源码事实（F-1…F-18）** 与 **§8 的纪律与禁止清单**。
**读者**: 后续实现者（可能是能力较弱的模型）。

---

## 0. 结论摘要

| 问题 | 推荐方案 | 一句话理由 |
|---|---|---|
| ① 添加库不手写 toml | **前缀命令 `/verb`**（用户的思路 a），快捷键降级为「往输入框写文本」 | 模式状态存在于**文本**里而不是内存里 → 延续本次重构的「单一状态源」律，弱模型改不坏 |
| ② 新增知识片段 | 同一个命令面：`/new` 进**捕获面板**（复用详情模式外壳） | 唤起方式统一，UI 零新组件；但**前提**是 core 先开出内容写入 API（现在完全没有） |
| ③ 结果/预览面板扩展 | **两维正交**：`target`（产物形态）留在 core，`kind`（内容语义→视图）在前端做 `viewRegistry` | architecture.md:239 已定此律，本文把它落成可执行契约 + 给出脑图样板 |
| **前置阻断项** | **必须先修 bbolt 独占锁**（§3） | 不修则「添加库→重载工作区」会让 Launcher **自己把自己挂死**，且今天 CLI 与 Launcher 已不能同时用同一个库 |

---

## 1. 为什么 ① 和 ② 必须一起设计

两者是同一个问题的两个实例：**只有一个 780px 输入框的极简 UI，如何承载不断增长的能力集，且不引入隐藏状态。**
一旦为「添加库」发明一套唤起方式、为「新增片段」再发明一套，第三、第四个能力就会继续叠加，最终回到「快捷键打架 + 模式状态散落」的老路。所以先定**一个**唤起契约，再让所有能力挂上去。

---

## 2. 已核实的源码事实（实现前必须知道，全部带出处）

### 2.1 关于「写」

- **F-1 core 完全没有内容写入 API。** 无 `AppendBlock` / `WriteBlock` / `UpdateBlock` / `DeleteBlock`，无任何 `.md` 写函数。core 里仅有的写入是：`core/config.go:201`（写 zoro.toml）、`core/bootstrap.go:150`（一次性生成 `默认知识库.md`）、`core/store.go`（只写派生索引，不写内容）。`Block.Raw` 明确是只读运行时字段（`core/block.go:65`）。
  → **结论：问题 ② 和 ③ 的 CRUD 都必须先在 core 新增写 API，不是「加个 UI」的事。**
- **F-2 `WriteWorkspaceConfig` 会丢注释、丢顶层未知键、且非原子写。** `MarshalWorkspaceConfig`（`core/config.go:169-187`）用 `toml.Marshal` 序列化 struct `workspaceConfigOut`（`config.go:159-163`，只有 `default` / `data_dir` / `libraries` 三个字段）：
  - `zoro.toml` 里所有 `#` 注释、键顺序、空行**全部丢失**（go-toml/v2 的 struct marshal 无 AST round-trip）；
  - **顶层**未知键**静默丢弃**；只有**库级** config 的未知键靠 `LibraryConfig.Extra` 保住（`config.go:104-148` 读、`config.go:204-235` 写回）；
  - `os.WriteFile` 直接覆盖（`config.go:201`），**非原子**：中途失败会留下半个 zoro.toml = 工作区打不开。
  - core 里已有 `atomicWrite`（`core/library.go:425-431`，tmp+rename），但**唯一调用者 `writeIndexView` 本身零调用者**（死代码）。
- **F-3 CLI `add` 的真实流程**（`cmd/zoro/main.go:199-251`，Launcher 应逐步对齐）：
  1. `workspacePath()`（`ZORO_WORKSPACE` → `./zoro.toml` → `~/.zoro/zoro.toml`）；若等于默认路径先 `core.EnsureDefaultWorkspace()`
  2. `os.ReadFile` + **`core.ParseWorkspaceConfig(string)`** —— 故意**不用** `WorkspaceConfigFromPath`，否则相对 root 会被提前解析掉（见 `config.go:165-168` 注释）
  3. 库名查重 → 4. root 相对路径以 zoro.toml 所在目录为基准转绝对 + `filepath.Clean`
  5. `append(cfg.Libraries, core.LibrarySpec{Name, Root})`，**Config 留零值**；`--default` 或「首个库且 default 为空」时设 `cfg.Default`
  6. root 不存在则 `os.MkdirAll(root, 0o755)` → 7. `core.WriteWorkspaceConfig(wsPath, cfg)`
  8. 注意：CLI **不**在 add 后重建索引（下一次 `search` 由 `ensureFresh` 自动增量刷新）。Launcher 是常驻进程、内存里握着旧 `ws`，**必须显式重载**（见 F-5）。
- **F-4 没有 `AddLibrary` / `RemoveLibrary` / `SaveWorkspace`。** 可用积木只有：`ParseWorkspaceConfig(s string)`、`MarshalWorkspaceConfig(cfg)`、`WriteWorkspaceConfig(path, cfg)`、`(*WorkspaceConfig).ResolvePaths(base)`。且 **`Workspace` 不持有自己的 config 路径**（`core/workspace.go:11-13` 只有 `Libraries []*Library`）——前端必须自己记住 `wsPath`（Launcher 已有 `a.wsPath`，`app.go:27`）。

### 2.2 关于索引生命周期（**最危险的一组事实**）

- **F-5 bbolt 独占锁 + 常驻进程 = 挂死。**
  - `OpenStore` → `bolt.Open(path, 0o600, nil)`（`core/store.go:28-42`）；`nil` 选项 → `Options.Timeout = 0`（bbolt v1.4.0 `db.go:1348`）。
  - bbolt 的 `flock`（`bolt_unix.go:17-45`）在 `timeout == 0` 时是**无限重试循环**（每 50ms 一次，永不返回错误）。RW 模式取 `LOCK_EX`（独占），ReadOnly 取 `LOCK_SH`（共享）。
  - `Library.Query` → `ensureFresh()` → 首次即 `OpenStore`（`core/library.go:316-341`），且 store **常驻不释放**（只有显式 `Library.Close()` / `Workspace.Close()` 才关，`library.go:376-381`、`workspace.go:72-77`）。
  - Launcher 是常驻进程（`HideWindowOnClose`）→ **它一旦查询过某库，就永久持有 `<库根>/.zoro/zoro.db` 的独占 flock。**
  - 后果：① Launcher 开着时终端跑 `zoro search/preview/index` 同一库 → **CLI 永久卡死**（无输出无报错）；② 第二个 Launcher 实例卡死；③ 未来「添加库后重载」若不先 `Close()` → **同进程不同 fd 照样互斥 → Launcher 挂死自己**。
  - 详见 `facts/pitfalls.md` **P-13**。
- **F-6 `ensureFresh()` 吞掉所有错误**（`core/library.go:317-320`，空 if 体，注释说「非致命，用旧块继续」）。→ 一旦按 §3 加了锁超时，争用会表现为「静默返回旧索引」= 用户搜不到刚写入的内容，比卡死更难查。**必须让这个错误可观测。**
- **F-7 刷新是文件级指纹增量**：`FileFingerprint{Size, MTime(ns)}`（`core/meta.go:37-40`）+ `diffFingerprints`（`library.go:393-408`）；脏文件才重扫（`rebuildIncrementalStore`，`library.go:187-231`）。无内容哈希、无 watcher → **同 size 同 mtime 的改动不可见**。写后不要依赖隐式刷新，显式调 `Library.Analyze(false)`。
- **F-8 索引存储已从 meta.json 迁到 bbolt**：`<库根>/.zoro/zoro.db`（`library.go:93-99`；有 `data_dir` 时用 `<dataDir>/<库名>/zoro.db`），`.zoro/meta.json` 只是**遗留迁移源**（`store.go:201-236`，迁完 `os.Remove`）。⚠️ **`facts/architecture.md` 仍写着 meta.json，是文档漂移，实现前以代码为准。**

### 2.3 关于渲染与扩展点

- **F-9 渲染不知道 kind。** `RenderMarkdownHTML(md string)` / `RenderMarkdown(md string, target RenderTarget)`（`core/render.go:28,97`）只收字符串，**没有 kind/library/path 参数**；target 选择是个 `switch`（`render.go:97-108`），**无注册表、无接口**。→ 「按 kind 分派视图」在当前签名下不可能，只能由前端分派（这也正是 architecture.md:239 定的归属）。
- **F-10 `TagKind` 是开放字符串。** `type TagKind string`（`core/block.go:12`）；未登记的名字变成 `unknown:<name>`（`block.go:23`），照样被扫描、持久化、检索。→ **`@mindmap` 今天就能存能搜，零 core 改动。** 要「一等公民」化只需三处：`block.go:15-20` 加常量、`registry.go:5-18` 加 case、`block.go:29-39` 白名单加一行。
- **F-11 `PayloadStyleOf`（`core/registry.go:31-36`）是零调用者的死钩子**；`@shell` 的 fence 收集是硬编码（`scan.go:130-142`）。→ 「新 kind 的负载怎么取」目前无可扩展机制，若脑图需要结构化负载，要么前端自己解析 `Raw`（推荐），要么把这个钩子真正接上（改动大，不推荐）。
- **F-12 扩展点接口大多只存在于文档**：`Action` / `SyncProvider` / `Cipher` / `Extractor` / `Collector` / `IndexBackend` 在 `.go` 里**均未定义**。代码里真实的只有 `Matcher`（`core/query.go:44-49`，含可替换的 `DefaultMatcher`）与 `Face`（`core/face.go:4-13`，已实现 Index/Title/Path 三面，`RawFace` 是**永不命中的占位**，`face.go:47-59`）。`faceRegistry`（`face.go:62-67`）是私有 map，**无导出的 Register** → 加面必须改 core。
- **F-13 大纲正文当前不可搜**：只有 `@` 行的 terms 进 index 面（`DefaultFaces()` = index/title/path，`face.go:70`），`RawFace` 是占位。→ 「搜到脑图节点」属于 P4 全文面（AD-6 的演进顺序），**不要在脑图特性里顺手做**。

### 2.4 关于 Wails v2.15.0 能力（已核实 module cache 源码）

- **F-14 原生目录选择框可用**：`runtime.OpenDirectoryDialog(ctx, OpenDialogOptions{Title, DefaultDirectory, CanCreateDirectories, ShowHiddenFiles, ...}) (string, error)`（`pkg/runtime/dialog.go:33`；选项结构 `internal/frontend/frontend.go:17-26`）。**返回 `""` 表示用户取消**（不是 error）。另有 `OpenFileDialog` / `SaveFileDialog` / `MessageDialog`。
- **F-15 ⚠️ darwin 上对话框是 sheet，挂在主窗口下**：`[dialog beginSheetModalForWindow:self.mainWindow ...]`（`internal/frontend/desktop/darwin/WailsContext.m:658`、`:741`）。我们的主窗口是无边框、置顶、76px 高的小浮窗 → sheet 会从一个小窗口垂下来，观感待验证；sheet 期间窗口不可移动。**必须 Mac 实测**；兜底方案见 §5.4。
- **F-16 ❌ macOS 文件拖拽不可用**（负面事实，别去试）：v2.15.0 的 `options.Options` **没有** `EnableDragAndDrop`（只有 `DisableResize`，`pkg/options/options.go:38`）；runtime JS 的 drop 分支依赖 `window.chrome?.webview?.postMessageWithAdditionalObjects`（`internal/frontend/runtime/runtime_prod_desktop.js`，`CanResolveFilePaths`），那是 **Windows WebView2 专有** → WKWebView 上恒为 false，drop 无任何效果。所以「把文件夹拖进搜索框来添加库」在 v2 做不到（等 v3，见 todos 9.6）。
- **F-17 进度推送可用**：`runtime.EventsEmit(ctx, name, data...)` + JS `window.runtime.EventsOn(name, cb)`（`pkg/runtime/events.go:8,46`）。大库重建索引时用它推进度，避免 UI 假死。
- **F-18 `mac.Options` 无多窗口/无文件 drop**：字段只有 `TitleBar / Appearance / ContentProtection / WebviewIsTransparent / WindowIsTranslucent / Preferences / DisableZoom / About / OnFileOpen / OnUrlOpen / DisableEscapeExitsFullscreen`（`pkg/options/mac/mac.go:18-36`）。

---

## 3. 前置阻断项 P0：先修 bbolt 锁（否则 ①②③ 全部会挂死）

**必修（两条，都很小）：**

1. `core.OpenStore` 传超时，让争用**快速失败**而不是无限挂起：
   ```go
   db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 300 * time.Millisecond})
   ```
   错误信息要点名原因（「另一个 zoro 进程正持有 <库> 的索引锁」），不要裸抛 `timeout`。
2. Launcher 改成**不长期持有 store**：二选一
   - **推荐**：`Hide()` 时 `a.ws.Close()`，唤起（`toggle`/首次 Query）时重开 → 与 CLI 可共存，代价是每次唤起多几毫秒 mmap；
   - 或：每次 bridge 调用内 Open→用→Close（更彻底，但要给 `Library` 加「无 store 也能用内存 blocks 查询」的路径）。

**必须同时做**：把 `ensureFresh()` 吞掉的错误送出来（F-6）——否则加了超时后，症状从「卡死」变成「静默搜不到新内容」，更难查。最小改法：`Library` 上存一个 `LastRefreshErr error`，`Status()` 里带出来。

**记录但不实施**：长期正解是换成可并发读的存储（SQLite WAL / 只读 JSON 快照 + 内存索引），属 AD 级决策。`ReadOnly: true`（LOCK_SH）能让多个读者共存，但仍排斥写者（CLI 的 Analyze 需要 RW），只能当优化，不能当唯一手段。

---

## 4. 方案①：统一能力唤起面 = 前缀命令 + 派生模式

### 4.1 三个候选的评估

| 维度 | A 前缀命令 `/verb` | B 快捷键切模式 | C 首 token 匹配动词（无 sigil） |
|---|---|---|---|
| 状态可见性 | ✅ 状态就在文本里（可见/可复制/可撤销/可日志） | ❌ 状态在内存 = 隐藏状态，正是本次重构刚消灭的 bug 源 | ✅ 同 A |
| 歧义 | ⚠️ 需一条精确判定规则（§4.2） | ✅ 无 | ❌ 搜「add 定投」会被劫持 → **直接否决** |
| 可发现性 | ✅ 输入 `/` 即在**现有结果面板**里列命令 | ⚠️ 需要额外提示条 | ⚠️ 同 B |
| 扩展成本 | ✅ 加一条 = 注册表加一项 | ❌ 加一个模式 = 新 UI + 新快捷键 + 冲突管理（键盘有限，不可持续） | ✅ 同 A |
| core 是否需感知 UI | ✅ 不需要（verb 表全在前端） | ✅ 不需要 | ✅ 不需要 |

**推荐 A 为主干，B 降级为「输入方式」**：快捷键不再是模式开关，而是**往输入框写文本**的动作（空查询时按 Tab 或 ⌘K → 写入 `/`）。用户拿到 B 的手感，架构上没有 B 的隐藏状态。

### 4.2 判定规则（必须精确、可单测）

```
mode === "command"  当且仅当：
  query[0] === "/"（无前导空格）
  且 第一个 token 去掉 "/" 后 ∈ 已注册 verb 集合，或为空（→ 列出全部命令）
否则 mode === "search"
```

- 于是搜 `/root/zoro` 仍是搜索（`root/zoro` 不是 verb）。
- **逃生舱**：前导空格强制搜索模式。
- `mode` 必须是 **getter**（`get mode() { ... }`），不是数据字段 —— 与 `get summaryText()` 同构，**零写入点，不可能忘记同步**。
- **不做 shell 式引号解析**。这是刻意的简化：凡是需要引号的复杂参数（含空格的路径、多行正文）都应走**原生控件或专用面板**，而不是塞进一行输入框。参数只按空白切分。

### 4.3 命令面 v1（最小集；每条写操作都要有确认步骤）

| 命令 | 行为 | 「难输入」怎么解决 |
|---|---|---|
| `/lib` | 列出已声明库（name · root · 块数） | — |
| `/lib add` | 添加知识库 | **弹原生目录框**（F-14），选完在面板显示「将添加 X 为 `<name>`，Enter 确认」 |
| `/lib rm <name>` | 移除声明（**只改 zoro.toml，绝不删文件**） | 名字靠补全选，不手打 |
| `/lib default <name>` | 设默认库 | 同上 |
| `/new [kind]` | 新增知识片段 | 进**捕获面板**（§5），多行输入 |
| `/theme <name>` | 切主题 | 补全三个值 |
| `/reindex` | 强制重建索引 | 无参；异步 + `EventsEmit` 推进度（F-17） |
| `/help` | 命令列表 | — |

**UI 统一性的关键收益**：输入 `/` 的瞬间，**结果面板改为展示命令候选**——复用同一个 `<ul>`、同一个 `.result-item`、同一套高亮、同一套上下键/Enter、同一套窗口高度自适应。搜索与命令是**同一个组件的两条数据源**，视觉与代码都统一，不需要任何新 UI。

### 4.4 `/lib add` 的完整链路（照抄，别漏步骤）

前端：`/lib add` → Enter → bridge `PickLibraryDir()`（Go 弹 F-14 的目录框）→ 返回路径 → 面板显示确认行（含将要写入的 name，取自目录 basename，可编辑）→ Enter → bridge `AddLibrary(name, root)`。

Go 侧 `AddLibrary` 必须**逐步对齐 F-3**，然后多做两步：

```
1. 读 a.wsPath → os.ReadFile + core.ParseWorkspaceConfig(string)   // 不要用 WorkspaceConfigFromPath
2. 库名查重（重复 → 返回明确错误，不覆盖）
3. root 转绝对 + filepath.Clean；os.MkdirAll(root, 0o755)（不存在则建）
4. append LibrarySpec{Name, Root}；首个库且 Default 为空 → 设 Default
5. 写回 zoro.toml（写入方式见 §4.5 的决策）
6. ★ a.ws.Close() 再 a.ws, a.wsErr = core.LoadWorkspace(a.wsPath)   // 不 Close = 自锁死（F-5）
7. ★ 对新库 Analyze(true)（或 AnalyzeAll(false)），大库用 EventsEmit 推进度
8. statusText 报「已添加 <name>（<N> 个块）」
```

第 6、7 步是 CLI 不需要、Launcher 必须的（常驻进程 + 内存态）。**这两步漏掉的症状分别是「Launcher 卡死」和「加了库但搜不到」。**

### 4.5 zoro.toml 写入方式（**待拍板**，见 §9-D1）

F-2 意味着：在 Launcher 里加一个库，用户手写的 zoro.toml **注释会全部消失**，顶层未知键**静默丢失**，且写入非原子。三个选项：

- **(a) 接受 + UI 明示**「将重写 zoro.toml（注释会丢失）」。成本最低，但对「知识库框架」来说丢用户注释很难看。
- **(b) 外科式文本追加（推荐）**：`add` 这类**只增不改**的操作，只在文件末尾追加一个 `[[libraries]]` 段，不重写既有字节 → 注释、顺序、未知键全部保住。代价：要处理文件末尾无换行、追加段的缩进/空行风格；且 `rm` / `default` 这类**必须重写**的操作仍退回 (a)。
- **(c) TOML AST 编辑**：go-toml/v2 不提供；要换库或自己写解析器，成本过高 → 否决。

**无论选哪个，都要顺手修的两件事**（都很小）：
1. `WriteWorkspaceConfig` 改用已有的 `atomicWrite`（F-2）；
2. `workspaceConfigOut` 加顶层未知键兜底（否则用户写的任何顶层自定义键都会在第一次 `add` 时被吃掉）。

---

## 5. 方案②：新增知识片段（capture）

### 5.1 core 必须先开出写 API（F-1：现在完全没有）

```go
// core/write.go（新增）
type WriteRequest struct {
    Library string   // 必填：身份三元组的库名
    Path    string   // 相对库根的 md 路径；空 → 库配置 capture_file，兜底 "inbox.md"
    Kind    TagKind  // 空 → KindIndex
    Terms   []string // @标签行的搜索词
    Title   string   // 可选：写成 "## <title>"
    Body    string   // 负载正文（可多行）
}
type WriteResult struct {
    Library, Path string
    Start         int      // 新块的 1-based 起始行 → 完整身份三元组
    Analyzed      bool     // 是否已刷新索引
}
func (w *Workspace) AppendBlock(req WriteRequest) (WriteResult, error)
```

**写 API 的五条约束（照抄，这是安全边界）：**

1. **只追加，永不改写既有字节**（append-only）。改/删是独立 API，且必须显式确认（延续 AD-8「Execute 须显式确认」的精神）。
2. 追加文本形如 `\n## <title>\n\n@<kind> <terms…>\n<body>\n`；文件不存在则创建（含父目录 `MkdirAll`）。
3. **写前 `Stat` 记录旧 size，失败时 `Truncate` 回原长度** → capture 出错不留半个块（这是用户可感知的可靠性）。
4. **写后显式 `Library.Analyze(false)`**，不要依赖 F-7 的隐式指纹刷新（同 size+mtime 不可见；`ensureFresh` 还吞错误）。
5. 用 `O_APPEND|O_CREATE|O_WRONLY` 单次 write；**不要** tmp+rename（会换 inode、丢权限、破坏用户的 git 假设）。

### 5.2 捕获面板：复用详情模式外壳，零新组件

`/new` → 主体区（现在放结果面板+预览的那个 `flex` 行）换成**捕获面板**，它就是详情模式那块 `glass-panel`：

- 一个 `<textarea>`：Shift+Enter 换行、⌘↵ 保存、Esc 退回搜索；
- 底部一行**必须显示确切落盘路径**：`保存到 <library>/<path> 的 @<kind> 块`；
- 一个保存按钮（`wails-no-drag`，与详情模式返回按钮同构：**放在 x-html/动态容器的兄弟位置**，不要被 innerHTML 冲掉）。

两个白拿的红利（本次重构已铺好，方案里点明以免实现者另起炉灶）：
- **窗口高度自适应**已就位：textarea 变高 → ResizeObserver 量到 shell 变高 → 窗口自动长高。不需要任何新代码。
- **`x-if` 物理移除**已就位：捕获面板用 `x-if="mode==='capture'"`，不占用「无内容时的透明区」。

⚠️ 唯一要注意的：捕获面板的高度上限（§7.3 的 `maxHeight` 声明机制同样适用）。

### 5.3 落盘目标从哪来

`zoro.toml` 的 `default` 库 + 库级配置新增 `capture_file`。
`LibraryConfig` 已有 `Extra map[string]any` 兜底未知键（`config.go:31-42`、读写见 F-2）→ **可以先用 `Extra["capture_file"]` 试水，不动 struct**；确认要长期保留再提升为一等字段。这符合「配置与内容分离」禁区（偏好进 zoro.toml，内容语义进 `@` 标签）。

---

## 6. 方案③：结果/预览面板的可扩展架构

### 6.1 架构律（architecture.md:239 已定，本文落成契约）

- **`target` 维（产物形态：html / ansi / text）留在 core。** core 只输出静态产物，**永不感知 UI 组件**（禁区：core 不进 UI）。
- **`kind` 维（内容语义 → 视图）在前端。** Launcher / Web 各自持有 `viewRegistry: kind → view`。
- core 的职责止于「把块的 kind + 结构化负载（`Raw`）交给前端」。F-9 决定了**分派只能在前端做**，这不是妥协，是本来就该如此。

### 6.2 预览面板扩展 = 加一个 view（便宜）

一个 view 就是一个 Alpine 组件对象 + 一段模板，**一个文件一个 view，互不引用**：

```js
// frontend/dist/views/mindmap.js
window.zoroViews = window.zoroViews || {};
zoroViews.mindmap = {
  id: "mindmap",
  kind: "mindmap",                    // 匹配 Block.Kind（开放字符串，F-10）
  title: "脑图",
  maxHeight: 540,                     // 声明想要的窗口高度上限（§6.4）
  actions: ["copy", "open", "save"],  // 能力表 → footer 快捷键提示由它派生
  component: function (ctx) { return { /* Alpine 状态 + 方法 */ }; },
};
```

分派仍然是 **getter，不是 if/else 链**：

```js
get activeView() { return zoroViews[this.activeKind] || zoroViews.markdown; }
```

模板里 `<template x-if="activeView.id === 'mindmap'">` 一个视图一段。
**给弱模型的顺序建议**：先用 `x-if` 笨分派（不会错）；视图超过 3 个再考虑 Alpine mixin 把 view 状态并进主组件（混入后仍在同一个响应式根上，不违反单一状态源）。

**新的 view 必须遵守的既有规则**（`frontend/README.md` 五条防回归规则全部适用），尤其：
- 运行时生成的 HTML 的样式放 **任何 `@layer` 之外**（否则被 Tailwind purge，P-9）；
- 不用 `id` 属性，用 `x-ref`；
- 桥接返回值一律判空（Wails 把 Go nil slice 序列化成 JS null，P-2）。

### 6.3 结果面板 CRUD = 难的那一半

难点不在 UI，在**四层一致性**：UI 操作 → 内容变更 → 索引刷新 → 身份三元组仍然有效。

**规则（照抄）：**

1. **一个动作一个 bridge 方法**，绝不在 JS 里拼文件路径或拼内容：
   `AppendBlock` / `UpdateBlock(library, path, start, terms, body)` / `DeleteBlock(library, path, start)` / `AddLibrary` / `RemoveLibrary` / `SetDefaultLibrary`。
2. **`start` 是行号 → 任何写操作都会让它漂移。** 删/插一个块后，同文件后续块的 `start` 全部失效，而前端手里还握着旧 candidate 列表。
   → **任何写成功后，前端一律 `resetResults()` + 重新 `doQuery()`，禁止局部 patch。** 慢一点，但**结构上不可能出现三元组错乱**（延续「让 bug 不可能发生」的律）。
   → 写 API 统一返回 `WriteResult`（含新三元组）+ `InvalidatedPaths []string`，供 UI 提示。
3. **core 侧写前必须重新定位校验**：读文件 → `SliceBlock` 取该块 → 校验该行确实是 `@<kind>` 且 terms 匹配 → 不匹配就返回「内容已变化，请重新搜索」。**绝不能盲写行号区间**（这是乐观并发控制的最小可用版本）。
4. **删除策略（待拍板，§9-D2）**：推荐 `DeleteBlock` 真删该块行区间，但 UI 二次确认 + 显示将删原文前 3 行，撤销依赖用户的 git（知识库通常在 git 里）。
   **否决**「移到 `.zoro/trash/`」：那会造出**第二份内容源**，违反「Markdown 唯一源」禁区。

### 6.4 窗口高度上限要变成视图声明（否则脑图被压扁）

`MAX_WINDOW_HEIGHT = 580` 现在是全局钳制。脑图/编辑器需要更大画布 → 改成由 view 声明：

```js
var cap = (this.activeView && this.activeView.maxHeight) || MAX_WINDOW_HEIGHT;
var target = Math.max(MIN_WINDOW_HEIGHT, Math.min(cap, h));
```

仍然是 `fitWindowToContent()` **唯一写入点**，只是上限来源变成派生值。下限不动（`main.go` 的 `MinHeight: 60` 与 `MIN_WINDOW_HEIGHT` 的耦合不受影响，P-11 的规则继续成立）。

---

## 7. 样板实例：脑图（把 §6 钉到一个具体特性上）

1. **内容形态**：`@mindmap <搜索词>` + **Markdown 嵌套列表**负载（不是 4 空格纯缩进，也不是 JSON blob）：
   ```markdown
   @mindmap 定投 计划 复盘
   - 定投
     - 标的：沪深300
     - 频率：每月
   ```
   - 选嵌套列表的理由（**两个已核实的坑**）：① **4 空格缩进在 goldmark 里是代码块**（→ `<pre>`，树结构全丢）；② **缩进的 `@xxx` 行会被 `StripDirectives` 从渲染输出里吃掉**（`core/render.go:56`）→ 节点名不能以 `@` 开头。
   - 否决 JSON blob：不可 diff、合并敌对、用户无法手写。
   - **最大红利**：嵌套列表本身就是合法 Markdown → **`RenderMarkdownHTML` 零改动就能给出可用的静态渲染**（CLI / 静态站点白拿一个还不错的降级展示）。
2. **登记 kind**（可选但推荐，F-10 三处改动）：`KindMindmap` 常量 + `registry.go` case + `TagKindFromString` 白名单。不登记也能用（`unknown:mindmap`），但 title 降级、faces、校验会不一致。
3. **交互编辑**（Launcher，`views/mindmap.js`）：`LoadRaw` 拿文本 → 解析成树 → 折叠/增删/拖动节点 → 保存时序列化回嵌套列表 → `UpdateBlock`。
   **验收标准：序列化必须幂等且保真**（同一棵树 → 逐字节相同的文本），否则每次保存都产生无意义 diff，污染用户 git 历史。
4. **新建脑图**：走 §5 的 `/new mindmap`（捕获面板里 body 就是初始大纲）→ 唤起方式与「新增片段」完全统一，这正是 §1 的目的。
5. **检索边界**：大纲正文当前**不可搜**（F-13）。「搜到脑图节点」是 P4 全文面的事，**不要在本特性里顺手做**（会破坏 AD-6 的三面演进顺序）。
6. **窗口**：脑图 view 声明 `maxHeight`（§6.4），例如 540 或更大。

---

## 8. 给实现者的总纪律（一句话 + 禁止清单）

> **新能力 = 注册表加一项（verb 或 view）+ bridge 加一个方法 + core 加一个 API。**
> 永远不要新增「模式状态」、不要在 JS 里拼内容或路径、不要绕过三元组定位就写文件。

**禁止清单：**
- ❌ 用 `@` 当命令 sigil（与内容模型的 `@tag` 语义冲突）
- ❌ 把 `mode` 做成数据字段（必须是 getter）
- ❌ 做 shell 式引号解析（复杂输入走原生控件/专用面板）
- ❌ 重载 workspace 前不 `Close()`（F-5 → 挂死）
- ❌ 依赖隐式指纹刷新（F-7 → 写后显式 `Analyze`）
- ❌ 写后局部 patch 结果列表（规则 §6.3-2 → 一律重查）
- ❌ 盲写行号区间（规则 §6.3-3 → 先定位校验）
- ❌ 在 core 里放任何 UI/视图概念（禁区 + F-9）
- ❌ 试图用文件拖拽添加库（F-16：v2 macOS 不支持）
- ❌ 把删除的内容另存到 `.zoro/trash/`（违反 Markdown 唯一源）
- ❌ 手改 `frontend/dist/styles.css`（P-7）；运行时 HTML 样式必须放 `@layer` 外（P-9）
- ❌ 从 JS 调 `WindowSetMinSize`（P-11）

---

## 9. 待拍板决策点

- **D1 zoro.toml 写入方式**：(a) 整文件重写 + UI 明示丢注释 / **(b) 推荐：只增操作走外科式文本追加** / (c) 换 TOML AST 库（否决）。附带必修：`WriteWorkspaceConfig` 改原子写、顶层未知键兜底。
- **D2 删除语义**：**(推荐) 真删 + 二次确认 + 靠 git 撤销** / 移入 `.zoro/trash/`（否决，会造第二内容源）/ 暂不实现删除。
- **D3 命令 sigil**：**(推荐) `/`** / `>` / `:`。（`@` 已否决）
- **D4 Launcher 的 store 生命周期**（§3-2）：**(推荐) `Hide()` 时 Close、唤起时重开** / 每次调用内 Open-Close / 保持常驻并接受「CLI 与 Launcher 不能同时用同一库」。
- **D5 脑图负载格式**：**(推荐) Markdown 嵌套列表** / 4 空格缩进大纲（否决，goldmark 当代码块）/ JSON blob（否决，不可 diff）。
- **D6 捕获目标**：库级 `capture_file` 先用 `Extra` 兜底试水，还是直接提升为 `LibraryConfig` 一等字段？

---

## 10. 建议实施顺序（每步可独立验证）

1. **P0 修锁**（§3）：`OpenStore` 加 Timeout + Launcher 的 Close 策略 + `ensureFresh` 错误可观测。验收：Launcher 开着时，终端 `zoro search` 同一库能在 ~1s 内返回结果或明确报错，**不挂死**。
2. **core 写 API**（§5.1）：`AppendBlock` + 单测（追加、建文件、失败回滚、写后 Analyze、返回三元组）。纯 core，可用 `go test ./...` 独立验证，**不需要 Mac**。
3. **命令面骨架**（§4）：`mode` getter + verb 注册表 + 命令候选复用结果面板 + `/help` `/theme` `/reindex`（这三条不写内容、不改配置，风险最低，先把唤起契约跑通）。
4. **`/lib`**（§4.4 + §4.5）：先做 `list` / `default`（只读 + 小改），再做 `add`（含原生目录框，**Mac 实测 sheet 观感**），最后 `rm`。
5. **`/new` 捕获面板**（§5.2）：依赖第 2 步。
6. **view 注册表 + markdown view 迁移**（§6.2）：先把现有预览改造成 `zoroViews.markdown`，行为完全不变（可用 jsdom 冒烟测试守住，见 `journal/2026-09-09-launcher-frontend-jsdom-verify.md`）。
7. **CRUD 三件套**（§6.3）：`UpdateBlock` / `DeleteBlock` + 定位校验 + 写后重查。
8. **脑图**（§7）：最后做，此时它只是「注册表加一项 + bridge 加一个方法」。

> 第 1、2、6 步都能在 Linux 侧完整验证；第 4、5、7、8 步需要 Mac 实测（走 `skills/ship-launcher-release.md`）。

---

## 附：本文引出的两处文档漂移

1. ✅ **已修（2026-09-09 同日）**：`facts/architecture.md` 曾落后于代码——manifest 写的是 `.zoro/meta.json`（实际已是 bbolt `.zoro/zoro.db`，meta.json 只是遗留迁移源，F-8）；三面搜索写「P0/P1 只落 index 面」（实际 `core/face.go` 已实现 index/title/path 三面 + `RawFace` 占位）；`LibraryConfig` 字段表缺 `Faces` / `FaceWeights`；`zoro.toml` 示例缺 `data_dir`；技术选型表无 bbolt 行。校正后：analyze/元数据节按 bbolt 现状重写、多面搜索表改为「代码现状 + 默认权重 + raw 是桩」、`LibraryConfig` 补全五字段并加「写回会丢注释/顶层未知键」警告、技术选型补 bbolt 行。`facts/decisions.md` 同步补记 **AD-13 / AD-14**（已实现但当初没写 AD 的两项，只记现状与代价，不补编理由）。
2. ⬜ **仍未处理**：`zoro-index.tsv` 从未被写出——`writeIndexView`（`core/library.go:254-256`）零调用者，但 CLI usage 文本（`cmd/zoro/main.go:24`）仍在承诺它 → 要么接上，要么删掉承诺。（已在 `architecture.md` 标注为已知漂移。）

> **落地追踪**：`todos.md` 看板 **5.5**（P0 修锁）与 **9.8–9.11**（命令面 / 捕获 / 视图注册表 / 脑图）；D1–D6 拍板后各自补一条 AD（见 `facts/decisions.md` 待定项）。
