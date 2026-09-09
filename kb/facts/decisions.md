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

## 补记：已实现但未记录的决策（流程漂移）

> 这两项**代码已落地**，但当时没写 AD。此处只补记「选了什么、代价是什么」，
> **不补编理由**——原始权衡没有被记录，事后编造比缺失更有害。若将来要推翻其中任一项，先补一次真实评估。

### AD-13（补记）元数据 store：bbolt 单文件

- **现状**：库级元数据从 `.zoro/meta.json` 迁到 bbolt 单文件 `<库根>/.zoro/zoro.db`（或 `<data_dir>/<库名>/zoro.db`），schema 2，三个 bucket（`meta` / `blocks` / `fingerprints`）；`meta.json` 仅作遗留迁移源。纯 Go、无 cgo、事务自带一致性。
- **未记录的权衡**：为什么不是 sqlite / 继续 json / badger —— 无据可查。
- **已付代价**：bbolt 是**独占锁**且 `OpenStore` 传 `nil` options（`Timeout=0` → 无限重试永不报错），store 打开后常驻不释放 → 常驻型前端会锁死同库 CLI，见 `pitfalls.md` **P-13**。这个代价当初没被识别。
- **何时重新考虑**：需要多进程并发读写（Launcher + CLI + `zoro serve` 同时在线）时，锁模型必须重新设计（`bolt.Options{Timeout}` / ReadOnly 共享锁 / 或换引擎）。

### AD-14（补记）多面搜索：`Face` 接口 + 库级可配

- **现状**：AD-6 的「三面」已扩成 `Face` 接口（`Name/Weight/Text/Matcher`）+ `faceRegistry`，实现 index(10.0) / title(5.0) / path(3.0) / raw(1.0，**桩，恒不命中**)；库级 `faces` / `face_weights` 可选面与调权，未知面名静默跳过。
- **超出 AD-6 的部分**：AD-6 只说三面且「先 index，再 title，最后 raw」，未预见 `path` 面，也未预见面集可被库级配置改写。
- **已付代价**：`raw` 面注册了但不工作 —— 配置里写 `faces = ["raw"]` 不报错也不生效，是个静默陷阱（已写进 `architecture.md`）。

## 待定项（backlog）

- **Launcher 能力扩展的 6 个待拍板决策点 D1–D6**（`zoro.toml` 写入策略 / 删除语义 / 命令符号选型 / store 生命周期 / 脑图载荷格式 / `capture_file` 归属）：方案与推荐见 `../journal/2026-09-09-launcher-command-surface-and-view-extension-plan.md` §9，**定了之后各自补一条 AD**。
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
