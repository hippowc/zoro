# zoro 决策记录（facts/decisions.md）

> 只记「为什么这么选、否决了什么、代价、何时重新考虑」。缺了它，后人会“顺手优化”掉结论。
> 状态未单列时表示仍在生效；废除项见文末。

## AD-1 技术栈：Go + Wails v2

- **选择**：core 用 Go package；桌面 Launcher 用 Wails v2 + 系统 WebView 自绘浮窗；CLI 是裸 Go main。
- **否决**：Rust + Tauri（本项目上一版技术栈）。
- **代价**：
  - macOS 桌面端必须在本机 Xcode 工具链下构建，无法在 Linux 交叉编译 macOS 版。
  - Wails 透明窗口 / 透明 WebView 在不同 macOS 版本有已知行为差异（见 `facts/pitfalls.md`）。
- **何时重新考虑**：若需要三平台原生控件一致性、或必须完全脱离 Xcode 的发布链路时，重新评估 Tauri v2 / egui / 纯 Web 前端。

## AD-2 功能模型延续 v10（统一 Block / manifest v2）

- **选择**：功能场景与设计方案延续并精简 v10（统一 Block 模型 / Workspace 单模型 / manifest v2），仅技术栈与目录落地调整。
- **否决**：按“库类型”分裂多个模型、给 `@index` 特殊入口、存储块 end 位置。
- **理由**：所有标签平级为块，边界到下一个标签 / EOF 即可，现场推导而**不存 end**，降低一致性负担。
- **何时重新考虑**：出现需要跨编辑稳定引用（如块内锚点跳转）时，重新引入“稳定条目 id”，届时与身份三元组并存。

## AD-3 一级对象是 Workspace（单一模型）

- **选择**：一律由 `zoro.toml` 声明库集合；单库 = `libraries` 长度为 1，无特殊入口。
- **否决**：`ZORO_ROOT` / `ZORO_LIBS` 环境变量入路、`./` 单个库快捷模式。
- **理由**：一个模型覆盖所有规模；库名必须出现在展示路径 / 候选行 / URL 中，身份三元组 `(库名, path, start)` 不串库。
- **何时重新考虑**：若单库工作流占绝对多数且“零配置”诉求压倒显式声明时，可加“隐式单库”体验层，但不得改 manifest 模型。

## AD-4 内容契约：Markdown 唯一源 + 目录自由

- **选择**：内容只有 Markdown（+ `@` 指令）；HTML 是渲染产物；内容目录零约束，唯一技术约束是 `@` 标记行格式。
- **否决**：把 HTML / 数据库行 / 专有格式纳入内容源；给内容目录强加固定树状结构。
- **理由**：文本可审计、可 git、可渲染多 target；作者只需维护 Markdown。
- **何时重新考虑**：出现强结构化 / 富媒体独占场景时，经 `Extractor` 插件转换为块，不改 Block 模型。

## AD-5 策展型知识库（不把“全盘文件”当主路径）

- **选择**：默认只搜 `zoro.toml` 已声明库根下的 `*.md` / `*.mdx`；跳过隐藏目录。
- **否决**：“扫描本地所有文件”作为搜索主路径。
- **理由**：意图面由作者显式声明（搜索词），召回质量可控；非 Markdown / 库外文件不进核心搜索面。
- **何时重新考虑**：需要扩展知识源时按 `Collector → Extractor → IndexBackend` 插件抽象接入，不污染 core；不必要时不做。

## AD-6 三面搜索与演进顺序

- **选择**：`index`（主面）→ `title`（召回面）→ `raw`（兜底面）；P0/P1 只落 index terms 主面，title/raw 作为显式扩展点占位。
- **否决**：一开始就上全文检索；让 title/raw 与 index 同权重竞争。
- **理由**：index 是作者主动声明的“何时被找到”，ROI 最高；全文引擎（bleve/zinc）延后到 P4 再定。
- **何时重新考虑**：索引术语覆盖不足、查准/召回明显受损时，再引入 title 面与 P4 全文兜底面。

## AD-7 扩展走 Go interface，不做动态加载

- **选择**：展示扩展 = 新 main package 依赖 core；能力扩展 = core 定义 `Action` / `SyncProvider` / `Cipher` 接口。
- **否决**：插件动态加载、脚本化插件系统。
- **理由**：先单仓库 deliver；动态加载的复杂度（版本、安全、分发）在单平台 MVP 阶段不划算。
- **何时重新考虑**：出现第三方贡献插件、或需要 runtime 安装/卸载插件时再评估。

## AD-8 Action 权限分级与默认动作

- **选择**：默认只保证 `Copy` + `Preview`/`Open`（三平台一致）；`Execute` 须显式确认；`Insert` 回填最后做。
- **否决**：默认执行 `@shell`；默认模拟输入回填。
- **理由**：外部来源内容安全默认拒绝；`Insert` 在 macOS 需 Accessibility、Wayland 基本不可行，强推会牺牲一致性与体验。
- **何时重新考虑**：当“回填”成为高频需求、且目标平台集中在 macOS/Windows 时，再做平台可选回填（Launcher v2 阶段）。

## AD-9 平台分发策略

- **选择**：开发/自用 = 本地构建 + ad-hoc 签名（免费）；CLI 大众分发 = Homebrew formula；GUI 大众分发 = Developer ID 签名 + 公证。
- **否决**：MVP 阶段就上 Developer ID + 公证、或让用户自己装 Wails CLI/Go 工具链。
- **理由**：用户要求“下载即用”；大众化再投入正式签名与公证。
- **何时重新考虑**：进入大众分发 / 需要 GitHub Actions 自动签名公证时，切换到 Developer ID + notarytool 工作流。

## AD-10 Launcher 热键与窗口行为

- **选择**：Carbon `RegisterEventHotKey` 注册 **Cmd+Shift+Z** 全局唤起居/隐藏；无边框、置顶、半透明浮窗；Esc 隐藏，关闭=隐藏到后台。
- **否决**：辅助功能/输入监控权限方案做“无边框输入框”回填。
- **理由**：热键唤起无需特殊权限；回填才需要 Accessibility。
- **何时重新考虑**：若 Cmd+Shift+Z 与用户软件冲突，把热键做成可配置项（配置进 `~/.zoro` 而不是硬编码）。

## AD-11 macOS 透明窗口实现

- **选择**：`WebviewIsTransparent: true` + `WindowIsTranslucent: false` + `BackgroundColour.A=0` + `Appearance: Aqua`；玻璃质感交给前端 CSS（rgba + backdrop-filter）。
- **否决**：`WindowIsTranslucent: true` 的原生 NSVisualEffectView 方案。
- **理由**：后者在较新 macOS 上会在 WebView 后渲染出不透明深色/褐色块，破坏“只有面板有背景、其余全透明”的目标。
- **何时重新考虑**：若后续要求“原生 behind-window 模糊”且目标 macOS 版本表现稳定，可重新验证 `WindowIsTranslucent` + 新 Wails 版本。

## AD-12 Launcher 前端：Tailwind CSS + Alpine.js + 窗口高度自适应

- **选择**：样式用 Tailwind v3（固定 `3.4.19` LTS，唯一源文件 `frontend/src/input.css`，产物 `dist/styles.css` 提交进仓库）；交互用 Alpine.js（vendor `3.17.2` 的 cdn 构建，不打包）；窗口高度由前端 ResizeObserver 单向贴合内容高度（Spotlight 式）。
- **否决**：
  - **继续手写 CSS + vanilla JS**：状态散落在 DOM class（`is-hidden` / `is-visible` / `show-detail` / `show-preview` / `show-results`）里，同一状态多处写入；连续 6 个 `fix(launcher)` 提交都在修同一类问题（内联样式压过类、`innerHTML` 冲掉返回按钮、隐藏不彻底）。用户明确要求「重构到便于后续用低水平大模型开发而不犯错的水平」。
  - **Vue / React + 打包器（Vite/esbuild）**：给一个约 350 行的 UI 引入第二套构建系统、第二份配置、第二组失败面，收益不成比例。
  - **Tailwind v4**：CLI 包名与配置方式（CSS-first）都不同，训练语料里占比低，且 v3 有 LTS 线；本次目标是降低后续维护者的犯错概率，不是追新。
  - **固定窗口高度 + 用 CSS 隐藏空区**：用户多轮反馈的「透明框」真身就是固定 780×580 窗口里的空白透明区（叠加 macOS 窗口阴影），它不是 DOM 元素，`display/visibility/opacity/height/overflow` 五重隐藏都无效。只有让窗口贴合内容才能根治（配合 `MinHeight` 下调，见 P-11）。
- **理由**：让 bug 类别在**结构上不可能发生**，而不是靠每次小心——`x-if` 把无结果区域移出 DOM（透明框无处依附）；单一 Alpine 组件是唯一状态源；汇总文案用 getter 派生（零个写入点）；窗口尺寸只有一个写入点；返回按钮是 `x-html` 容器的兄弟节点（`innerHTML` 冲不掉）；Tailwind 工具类 + `@layer` 顺序消灭特异性战争。
- **代价**：
  - 构建链多一个 Node 依赖（本地 `brew install node`，CI 加 `setup-node@v4`）；`wails build` 会跑 `npm install` + `npm run build`。
  - `dist/styles.css` 是生成物却必须提交（Go `//go:embed` 需要非空目录，且 Linux 侧无 Node 时要能 `go build .`）→ 新增 P-7 这类「手改生成物」的坑，用 banner 注释 + README 规则 + `.gitattributes` + CI grep 四层缓解。
  - 引入两条 Tailwind 特有的静默失败路径（P-8 `@layer` purge、P-9 运行时 HTML 样式），CI 的 `Verify frontend assets` 步骤是唯一自动兜底。
- **何时重新考虑**：UI 规模增长到需要组件复用/路由/双向数据流（例如 Launcher 长出设置面板、多标签页）时，评估 Svelte 或 Vue + Vite；Wails v3 升级（`todos.md` #9.6）时一并重估，因为多窗口会改变「一个窗口一个组件」的前提。

## AD-15 块级写 API：身份仍是三元组 + `expect` 乐观并发 + 就地改写

- **选择**：`core/write.go` 提供 `AppendBlock` / `UpdateBlock` / `DeleteBlock` / `PreviewDelete`，全部按 `(库名, path, start)` 定位，**不引入块 ID**。改与删要求调用方交出 `expect`（它上一次 `LoadRaw` 拿到的原文），写前重新定位 + 逐字比对，不一致返回 `ErrBlockChanged`；写后读回逐字节校验，不符就 `Truncate` 回滚。追加是单次 `O_APPEND`，改写是就地 `Truncate`+`Write`。块边界只有一处定义（`blockEnd` + `isHeadingLine`），扫描器 / 预览 / 写入层共用。
- **否决**：
  - **给块发稳定 ID（写进 Markdown 或存进 store）**：会污染内容契约（作者要看得见并维护它），或在文件被外部编辑后立刻失效。三元组虽然易失，但失效是**可检测**的（`expect` 比对），而 ID 失效往往是静默的。
  - **文件锁 / 全局互斥**：跨进程锁不住编辑器；用户的 md 本来就该能同时被别的工具改。乐观并发 + 明确报错比悲观锁更符合「Markdown 是唯一源」。
  - **tmp+rename 原子替换**：会换 inode。用户的文件常常正在编辑器里开着，编辑器随后保存就会把我们的改动整份覆盖掉（而且没有任何报错）。原子性在这里是**错误**的目标，我们要的是「不打扰别人打开的那个文件」。
  - **让 `UpdateBlock` 一起改块的 `##` 标题**：标题行不在 `Block.Raw` 里（它是块的标题，不是负载），改它意味着改写范围与用户在预览里看到的范围不一致，违反 L2。删除则必须连标题一起删，否则留下孤儿 `##`。
- **理由**：三条写定律（L1 `start` 易失、L2 看到的==被改的、L3 不换 inode）把「精准增删改查」这个核心优势变成可验证的不变式，而不是靠调用方小心。
- **代价**：
  - 前端不能就地更新一条候选：`start` 会漂移，写完必须整体重新查询（L1）。
  - **负载中间夹着 `##` 标题行时硬拒绝**（那种行在 `expect` 里看不见，改它会静默删掉用户内容）→ 用户得去编辑器里手工处理。这是刻意的能力缺口。
  - 并发写冲突时用户要重走一遍「搜索 → 预览 → 改」，没有自动合并。

## AD-16 `zoro.toml` 外科式文本编辑 + 「添加一个库」只有一条链路（决策点 D1 已定）

- **选择**：程序化改声明文件一律走 `core/configedit.go`（按行改、其余字节一个不动，改完前后各 `ParseWorkspaceConfig` 一次自检，任何意外就放弃写入）。「添加一个知识库」整条链路收进 `core.AddLibrary(wsPath, name, rootArg, setDefault)`，CLI `zoro add` 与 Launcher `/lib add` 调同一个函数。
- **否决**：
  - **`MarshalWorkspaceConfig` 读-改-写**：`toml.Marshal` 会**丢掉全部注释**和顶层未知键。用户手写的 `zoro.toml` 里注释是文档，丢了就是数据损坏。
  - **TOML 库的 AST 级编辑**（`toml.Edit` 之类）：能保住结构但保不住格式细节（空行、缩进、行尾注释位置），且引入一个新的依赖面；逐行文本编辑 + 解析自检更直白，也更容易被后来者读懂。
  - **CLI 与 Launcher 各写一份添加逻辑**：两边语义必然漂移（例如「首个库自动设 default」这种规则只有一边记得）。
- **理由**：声明文件是用户手写、版本控制里的东西，**默认不动它**；只动必须动的那几行，并且动之前先证明自己能把它读回来。
- **代价**：文本级编辑对畸形输入更敏感，所以每次改都要跑一遍「解析 → 改 → 再解析 → 比对不变量」的自检；`SetDefaultLibrary` 插入位置贴着第一个表头（空行留在它上面），纯粹不好看，评估后认为不值得为它增加规则。

## AD-17 Launcher 命令面：模式由文本派生 + 所有视图共用一种行形状

- **选择**：同一个输入框既是搜索框也是命令行。模式**不存状态**，由文本派生（`get mode()`：首字符是 `/` 且首个 token 是已注册动词的前缀 → 命令，否则搜索）。命令写在 `COMMANDS` 注册表里（`{args, desc, action, options?}`），加一条命令 = 注册表加一行 + 组件里写一个同名方法。所有视图（搜索命中 / 命令候选 / 知识库列表 / 待确认）产出**同一种行形状** `{id, title, indexHtml, meta, action, payload?}`，模板只遍历 `items` getter；回车统一走 `runActive()` 按 `action` 分派。不可逆动作（写 `zoro.toml`）两步走：先弹**原生目录选择框**，再把结果落成一行「待确认」让用户回车确认。
- **否决**：
  - **快捷键切换输入框模式**（另一个候选方案）：模式变成内存状态，就有「显示的是 A 模式、实际按 B 模式解释」的漂移空间；派生自文本则永远一致，而且用户能从输入框里读出当前模式。
  - **`@` 当命令前缀**：`@` 已经是内容契约里的标签符号（`@index` / `@shell`），复用会让「搜 @shell 块」和「执行命令」冲突。
  - **在 780px 输入框里手打目录路径**：含空格的路径、中文目录名、Tab 补全，原生 `NSOpenPanel` 全都已经解决好了（见 P-14）。
  - **每种视图各写一块 DOM + 各自的显示/隐藏开关**：这正是重构前那套 `is-hidden`/`show-detail`/`show-results` 的翻版，状态多处写入，低水平维护者必错。
  - **shell 式引号/转义规则**：命令面的参数是库名、主题名这类短 token，引号规则会让「难输入」的问题从路径转移到语法上；难输入的东西交给原生控件。
- **理由**：延续 AD-12 的目标——让 bug 类别在结构上不可能发生。模式无状态可漂移；行形状唯一，模板不可能对某种视图漏掉一个分支；候选匹配失败时有兜底行，不会退化成「空面板 = 静默失败」（P-15）。
- **代价**：
  - 搜索以 `/` 开头的词需要在前面加一个空格（逃生舱），是个要记住的小规则。
  - 命令面没有历史、没有多行编辑、没有真正的解析器（不支持引号），命令一复杂就得升级成别的交互。
  - 一次回车只做一件事：`/lib add` 需要两次回车（选目录 → 确认），比「一步到位」多一次按键，换来的是不可逆写入永远经过一次明示确认。

## AD-18 可视化块（脑图这类）：存储面永远是文本，编辑面可以不是（决策点 D5 已定）

- **选择**：把「存储」与「编辑」彻底分开——**存储面永远是纯文本 Markdown**（AD-4 不动摇），**编辑面可以是完全可视化的控件**（拖节点、画框，用户一个字都不敲），两者之间按 kind 注册一个 **codec**（文本 ⇄ 结构化对象）。每种特殊片段 = `{codec, view, actions}` 三元组，`view` / `actions` 留在前端注册表，**core 不因 kind 增加而改动**：它只提供 `LoadRaw`（取块文本）/ `UpdateBlock`（整块替换 + `expect` 乐观并发）/ `RenderMarkdownHTML`（静态兜底渲染）三件通用能力。脑图载荷用 **Markdown 嵌套列表**（`@mindmap <terms>` + `-` 列表），可视化用 **markmap**（MIT，`0.18.12`，已核验 npm：只有 lib/view/toolbar，**没有 editor 包**，即它是纯可视化、不给拖拽改结构；`markmap-view` 依赖 d3）。**分两阶段**：阶段 A「textarea + 实时脑图预览」——codec 是**恒等函数**，于是「序列化幂等/字节稳定」这个最难的风险自动消失、git diff 零噪声；阶段 B「拖拽节点」只有在 A 用不顺时才做（先评估 Tab/Shift-Tab 缩进是否已够用，那是 A 的增量且同样不需要 codec）。完整调研与能力矩阵见 `../journal/2026-09-09-visual-blocks-storage-vs-editing-surface.md`。
- **否决**：
  - **节点级操作进 core**（如 `UpdateMindmapNode(lib, path, start, nodeID, …)`）：① core 会因此认识每种 kind 的内部结构，违反「core 不放 UI/视图概念」；② 节点级 patch 要求 core 理解嵌套列表缩进语义，而那正是 codec 的职责——**两处定义必然漂移**；③ 收益不存在，整块替换 + `expect` 已是最小可用的乐观并发（AD-15），块只有几十行。
  - **JSON blob 存脑图**：不可 diff、合并敌对、用户无法手写、`RenderMarkdownHTML` 给不出任何有意义的降级展示。
  - **4 空格缩进大纲**（不带列表标记）：goldmark 当**代码块** → 渲染成 `<pre>`，树结构全丢；且缩进的 `@xxx` 行会被 `StripDirectives` 吃掉，节点名不能以 `@` 开头。
  - **CRDT（Yjs / Automerge）**：我们是单机工具，并发方是「Launcher + CLI + 用户编辑器」而非多用户实时协同；CRDT 需要旁路元数据，会让「**文件字节 == 用户内容**」失效，直接违反 AD-4。
  - **私有二进制 / SQLite 存内容**、**`.canvas` 式独立格式文件**：造第二内容源，用户的库不再能被 grep / git diff / 静态站点 / 别的编辑器消费（Notion 导出有损、Obsidian Canvas 成飞地，都是实证）。将来要自由画布，照 Excalidraw 的 `.excalidraw.md` 做法——把 JSON 塞进 .md 的 fenced code block，文件仍是合法 Markdown（优先级 P5+，现在不做）。
  - **Mermaid `mindmap` 当主路径**：**单向**渲染，图不能拖回文本；只适合当只读降级展示。
  - **core 里加 `View` / `Codec` 概念**：三个前端（CLI / Web / Launcher）展示能力天差地别，core 一旦有 view 概念就会被最弱的那个拖着走。core 只出 `target`（html/ansi/text），`kind → view` 永远在前端。
- **理由**：用户的疑问「是否一定要文本」其实是三个问题，答案不同——**落盘格式必须是文本**（否则丢掉整个 Markdown 生态），**用户操作不必经过文本**（编辑面与存储契约正交），**core 必须不理解块结构**（否则每加一种可视化块都要动三个前端共享的地基）。分开答这三条，才能既保住 AD-4 又拿到可视化编辑。
- **代价**：
  - 阶段 B 若真要做，`encode(decode(t))` 必须**逐字节相同**，这是个难验收的硬约束（做不到就不要上线，否则每次保存都在用户 git 历史里制造噪声）。
  - vendor markmap 会带进 **d3**，体积不小，落地前要先量（与 Alpine 同样纪律：vendor 进 `frontend/dist/`、离线运行、绝不走 CDN）。
  - 大纲正文当前**不可搜**（`raw` 面是桩）。「搜到脑图节点」属于 P4 全文面，不得在脑图特性里顺手做——会破坏 AD-6 的三面演进顺序。
  - 脑图 view 必须自己声明 `maxHeight`（否则 580px 上限会把它压扁），于是「窗口高度上限」从常量变成了视图属性。
- **何时重新考虑**：需要多人实时协同时（那时 CRDT 的收益才可能盖过它破坏 AD-4 的代价）；或需要自由坐标画布时（走 §5 的 fenced code block 方案，仍不换存储格式）。

## AD-19 稳定版本锚点：语义化版本号的**附注 tag**，不是长期开发分支

- **选择**：main 单条主干直接开发；「稳定版本」= 打在 main 上的**附注 tag** `vMAJOR.MINOR.PATCH`（`v1.0.0` 是第一个）。tag 同时是 CI 构建触发器（`.github/workflows/build-macos-launcher.yml` 匹配 `v*`）与 Release 的名字，于是**一个 tag 同时回答三件事**：这个版本的代码是什么、用户下载到的产物是哪个、什么时候可以退回来。回滚分四档，按破坏性从低到高：下 Release 产物 → `git checkout <tag>`（detached，不碰 main）→ `git revert`（保留历史、可推送）→ `git reset --hard <tag>` + `--force-with-lease`（改写历史，需明示授权）。配方见 `../tools/versioning-and-rollback.md`。
- **否决**：
  - **长期 dev/feature 分支 + 验证后合并主干**（用户最初的想法）：单人开发场景下它不增加任何回滚能力——回滚靠的是「有一个已知良好的锚点」，而 commit 与 tag 已经提供了；反而带来两类真实成本：① `kb/` 文档大量使用**递增编号**（P-1…P-15、AD-1…AD-19、todos 看板 9.x），两条分支各自往下编号，合并时必然冲突且冲突解错了不会有任何报错；② `frontend/dist/styles.css` 是 Tailwind **生成物**，两边各生成一次就是整文件冲突，人工合并等于手写生成物（违反 P-7）。
  - **每次改动开一个短命分支**：同上，且对「回到稳定版本」毫无帮助——稳定性来自 tag，不来自分支数量。
  - **只用 commit 不打 tag**：commit 是无限细的粒度，没有「哪个是稳定版」的语义；出问题时要在几百个提交里靠记忆找那个好的点，而记忆正是最不可靠的部分。tag 的价值就在于它是**显式声明**：我确认过这个版本是好的。
  - **移动 / 删除已推送的 tag 来「修正」一个坏版本**：Release 与用户已下载的 zip 会跟本地历史对不上，坏版本应该发下一个版本修，不是把 tag 挪走。
  - **轻量 tag（`git tag v1.0.0` 不带 `-a`）**：省一个字，丢掉打 tag 的人/时间/说明，`git tag -l -n1` 列不出版本清单，将来无法回答「这个版本当时为什么发」。
- **理由**：用户要的是「有可靠的手段回到一个相对稳定的版本」，这是个**锚点**问题而不是**隔离**问题。锚点用 tag 最直接；分支解决的是隔离（多人并行、怕污染主干），而单人 + 已有 commit 的情况下隔离需求不存在。另外 tag 已经必须存在（CI 触发 + Release 命名），复用它当稳定锚点是零新增机制。
- **代价**：
  - main 上会出现坏提交（没有分支挡着），所以 `scripts/verify.sh` 必须在推之前跑，且**不可逆的写入类改动**（改内容契约、改 `zoro.toml` 格式）要走 MINOR/MAJOR 版本号，不能混在 patch 里发。
  - 历史遗留 17 个 `launcher-<摘要>-<日期>` 轻量 tag 与新约定并存，`git tag -l` 看起来杂乱；保留不动（改写已推送 tag 被否决），只在 `tools/versioning-and-rollback.md` §3 说明。
  - `v*` 触发意味着每个正式版本号都会花一次 macOS runner 构建时间；预发布用 `v1.1.0-rc.1` 可分摊，但也因此产生更多 Release 条目。
- **何时重新考虑**：出现**第二个长期贡献者**、或需要同时维护多个已发布大版本（例如 v1 与 v2 都要收 bug 修复）时，才需要引入 `release/v1` 这类长期分支——那时分支解决的是并行维护，不是回滚。

## 补记：已实现但未记录的决策（流程漂移）

> 这两项**代码已落地**，但当时没写 AD。此处只补记「选了什么、代价是什么」，
> **不补编理由**——原始权衡没有被记录，事后编造比缺失更有害。若将来要推翻其中任一项，先补一次真实评估。

### AD-13（补记）元数据 store：bbolt 单文件

- **现状**：库级元数据从 `.zoro/meta.json` 迁到 bbolt 单文件 `<库根>/.zoro/zoro.db`（或 `<data_dir>/<库名>/zoro.db`），三个 bucket（`meta` / `blocks` / `fingerprints`）；`meta.json` 仅作遗留迁移源。纯 Go、无 cgo、事务自带一致性。
  ⚠️ 容器版本是 `StoreSchema = 1`（`core/store.go`）；`core/meta.go` 的 `Schema = 2` 是**遗留整文件 manifest** 的版本，两者不是一个东西（本文档与 `architecture.md` 都曾写混，2026-09-09 已订正）。
- **未记录的权衡**：为什么不是 sqlite / 继续 json / badger —— 无据可查。
- **已付代价（2026-09-09 已偿清）**：bbolt 是**独占锁**，而当初 `OpenStore` 传 `nil` options（`Timeout=0` → 无限重试永不报错）+ store 常驻不释放 → Launcher 锁死同库 CLI（P-13）。现在 `OpenStore` 传 `Timeout: 300ms` 并把 `bolt.ErrTimeout` 映射成 `ErrStoreLocked`，`Library` 也从不再跨调用持有 store（唯一入口 `withStore`）。
- **何时重新考虑**：需要**多进程并发写**（Launcher + CLI + `zoro serve` 同时在线且都要写索引）时，锁模型必须重新设计（ReadOnly 共享锁 / 或换引擎）。当前的「快速失败 + 尽力刷新」只解决读侧共存。

### AD-14（补记）多面搜索：`Face` 接口 + 库级可配

- **现状**：AD-6 的「三面」已扩成 `Face` 接口（`Name/Weight/Text/Matcher`）+ `faceRegistry`，实现 index(10.0) / title(5.0) / path(3.0) / raw(1.0，**桩，恒不命中**)；库级 `faces` / `face_weights` 可选面与调权，未知面名静默跳过。
- **超出 AD-6 的部分**：AD-6 只说三面且「先 index，再 title，最后 raw」，未预见 `path` 面，也未预见面集可被库级配置改写。
- **已付代价**：`raw` 面注册了但不工作 —— 配置里写 `faces = ["raw"]` 不报错也不生效，是个静默陷阱（已写进 `architecture.md`）。

## 待定项（backlog）

- **Launcher 能力扩展的 6 个决策点**（原始清单见 `../journal/2026-09-09-launcher-command-surface-and-view-extension-plan.md` §9）进度：
  - D1 `zoro.toml` 写入策略 → **已定**，AD-16。
  - D2 删除语义 → **已定**，AD-15（删除连块自己的 `##` 标题一起删；删前 `PreviewDelete`）。
  - D3 命令符号选型 → **已定**，AD-17（`/verb`；`@` 因为已是内容标签符号被否决）。
  - D4 store 生命周期 → **已定**，AD-13「已付代价」+ P-13（fixed）。
  - D6 `capture_file` 归属 → **已定**：库级配置（`LibraryConfig.Extra["capture_file"]`），默认 `inbox.md`，见 `Library.CaptureFile()`；前端还没有暴露它（捕获 UI 未实现）。
  - D5 脑图载荷格式 → **已定**，AD-18（Markdown 嵌套列表 + markmap 可视化；阶段 A 用**恒等 codec**，把「序列化幂等」这个风险直接消掉）。调研与两阶段实施见 `../journal/2026-09-09-visual-blocks-storage-vs-editing-surface.md`。
- ✅ **D1–D6 已全部拍板**，不要再当开放问题重新讨论；要推翻其中任一条，先在对应 AD 的「何时重新考虑」里找到触发条件。
- tag / facet：标签名即 facet；按 facet 过滤的 query（如 `-t video`）待做。
- 同主题块聚合：`index` 与同主题 `shell`/`video` 并排命中是否聚合成「主题行」，留待 P3 交互设计。
- 稳定条目 id：暂用 `(库名, path, start)`；跨编辑引用跳转需再引入内容锚点。
- 库级 `include`/`exclude` glob、`recursive`、`follow_symlinks` 等范围细化待落地。

## 已废除 / 占名不实现

- `ZORO_ROOT` / `ZORO_LIBS` → `zoro.toml`
- `@cmd` → `@shell`
- `@run` → 行为外移（Action 分级）
- `@lang` → fence info 已带语言
- 扩展指令 `@alias` / `@desc` / `@ref` / `@hidden` 占名不实现
