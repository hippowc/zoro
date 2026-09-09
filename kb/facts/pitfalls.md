# zoro 已知坑（facts/pitfalls.md）

> 固定格式：触发 → 症状 → 根因 → 修法；状态字段 `active`（仍会踩）/ `fixed-in-X`（已修复）。
> 不贴原文长代码；要完整步骤去 `tools/` 或 `skills/`。

---

## P-1 macOS 透明窗口出现深色/褐色背景块

- **状态**：fixed-in-launcher-macos-v3.1（2026-09-08）
- **触发**：Wails macOS 窗口配置 `WindowIsTranslucent: true` + `WebviewIsTransparent: true` 的透明浮窗。
- **症状**：搜索框背后仍有一块正方形不透明深色/褐色背景；CSS 设了 `background: transparent` 也去不掉。
- **根因**：`WindowIsTranslucent: true` 会让 Wails 在 WebView 后插入 `NSVisualEffectView`（behind-window 模糊层），较新 macOS 上该层渲染成不透明色块，反而盖住真正透明的窗口背景。
- **修法**：设 `WindowIsTranslucent: false`，保留 `WebviewIsTransparent: true` + `BackgroundColour.A=0`，`Appearance` 用 `NSAppearanceNameAqua`；玻璃感放前端 CSS 的 `rgba` + `backdrop-filter`。参考 `ext/zoro-launcher/main.go`。

## P-2 Launcher 空查询结果导致前端 TypeError

- **状态**：fixed-in-launcher-macos-v3.1（2026-09-08）
- **触发**：`Query` 无命中时读取 `candidates.length`。
- **症状**：状态栏显示 `查询失败：TypeError: null is not an object (evaluating 'candidates.length')`。
- **根因**：Wails bridge 把 Go 的空 slice 序列化成 JS `null`，`.then(candidates => ...)` 直接访问 `.length`。
- **修法**：消费桥接返回值前先兜底 `candidates = candidates || []`；其他桥接结果（LoadRaw / RenderHTML）同样先判空再用。

## P-3 macOS 下载的 zoro 二进制被 Gatekeeper 拦截

- **状态**：active
- **触发**：浏览器下载 `zoro-darwin-arm64` / `.app` 后直接运行。
- **症状**：无法打开，或提示已损坏 / 未验证开发者。
- **根因**：二进制带 `com.apple.quarantine`；当前发布采用 ad-hoc 签名，未做 Developer ID + 公证（见 `facts/decisions.md` AD-9）。
- **修法**：`xattr -d com.apple.quarantine <file>`（或对 .app：`xattr -cr build/bin/zoro-launcher.app`）后运行；CI 发布的 zip 也可解压后用同样方式处理。

## P-4 工作区配置写不进去 / 读不到 zoro.toml

- **状态**：active（已在外壳修复，仍有版本差异）
- **触发**：运行裸二进制时当前目录没有 `zoro.toml`。
- **症状**：`读取工作区声明失败 zoro.toml: open zoro.toml: no such file or directory`；或目录不可写时报 `operation not permitted`。
- **根因**：查找顺序为 `ZORO_WORKSPACE → ./zoro.toml → ~/.zoro/zoro.toml`；老版本只认当前目录，未自动创建 `~/.zoro`。
- **修法**：优先设 `ZORO_WORKSPACE`；或用新版本让 CLI/桌面端首次自动创建 `~/.zoro/zoro.toml` 与 `~/.zoro/kb/默认知识库.md`。zip 解压后如遇 quarantine 先按 P-3 处理。

## P-5 无法在 Linux 交叉编译 macOS Launcher

- **状态**：active（平台限制，无修复）
- **触发**：在 Linux/Windows 上执行 `wails build -platform darwin/arm64` 或 `./build-macos.sh`。
- **症状**：CGO/Objective-C 编译失败或缺 Xcode 工具链。
- **根因**：Wails v2 macOS 前端含 Objective-C/C 源码，依赖 macOS 本机 Xcode 工具链；GitHub Actions 用 `macos-14` runner 承担。
- **修法**：macOS 本地构建用 `tools/launcher-build-release.md`；跨平台产物经 GitHub Actions `launcher-*` tag 触发。

## P-6 Launcher 前端 UI 与原生窗口背景不同步

- **状态**：active（2026-09-08 前端重构后由三层变四层）
- **触发**：只改前端 CSS 的透明声明，不动 `main.go` 的原生窗口配置；或反过来。
- **症状**：CSS 已 `background: transparent`，但窗口仍出背景块或深色块。
- **根因**：透明由「原生窗口层 + WebView 透明 + CSS 透明 + 桥接判空」四层共同决定，缺一层都不生效。
- **修法**：四层一起检查：`main.go`（P-1/P-11）→ `frontend/src/input.css`（**不是 `dist/styles.css`，后者是生成物，见 P-7**）→ `frontend/dist/index.html` → `frontend/dist/app.js` 桥接结果判空（P-2）。

## P-7 手改 `dist/styles.css` 的改动凭空消失

- **状态**：active
- **触发**：直接编辑 `ext/zoro-launcher/frontend/dist/styles.css`。
- **症状**：改完当场生效，下一次 `wails build` / `npm run build` 后改动全无。
- **根因**：该文件是 Tailwind 的编译产物，`frontend:build` 钩子每次整文件覆盖。
- **修法**：改 `frontend/src/input.css`，再 `npm run build`。生成物已用 `.gitattributes linguist-generated=true` 标记，PR 里默认折叠。

## P-8 Tailwind `@layer` 里的自定义类被 purge，构建却是绿的

- **状态**：active
- **触发**：`tailwind.config.js` 的 `content` 路径写错 / 漏了 `dist/app.js`；或类名只在运行时字符串里拼接。
- **症状**：`npm run build` 成功、`wails build` 成功、应用照常签名发布，但界面完全没样式（`styles.css` 只剩几 KB 的 preflight）。
- **根因**：Tailwind 会 purge `@layer components` / `@layer utilities` 里「没在 content 文件中以字面量出现」的类；`content` 错了就等于所有自定义类都没被用到。
- **修法**：类名必须以字面量出现在 `content` 覆盖的文件里（`dist/index.html` + `dist/app.js`，**绝不含 `dist/alpine.js`**——压缩过的第三方代码会提取出成百上千假类名）；改完 `grep <类名> dist/styles.css` 自查；CI 的 `Verify frontend assets were compiled` 步骤是唯一的自动兜底，不要删。

## P-9 预览区 Markdown / 命中高亮样式消失

- **状态**：active
- **触发**：把 `.md-body`、`mark` 相关规则放进 `@layer components` 或 `@layer utilities`。
- **症状**：搜索结果的高亮底色没了、预览区 Markdown 变成一坨无样式文本，但构建全绿。
- **根因**：这两处 HTML 是运行时才生成的（`x-html` 注入 Go `RenderHTML` 的产物、`highlight()` 产出的 `<mark>`），Tailwind 的扫描器永远看不到它们，放进 `@layer` 必被 purge。
- **修法**：放 `src/input.css` **第 5 节**，即任何 `@layer` 之外的普通 CSS。代价是它排在 utilities 之后，所以不要再在同一元素上叠 Tailwind 工具类（会反过来被压掉）。

## P-10 窗口高度抖动 / ResizeObserver 自激

- **状态**：active（预防性）
- **触发**：在 `<main x-ref="shell">` 子树里使用 `100vh` / `h-screen` / `height:100%`。
- **症状**：窗口高度反复跳动，或稳定在一个错误的值。
- **根因**：窗口高度决定 webview 高度，视口相关高度又让内容高度跟随 webview → 「窗口高度 ↔ 内容高度」互为因果形成回路。
- **修法**：shell 高度必须 100% 由内容决定；窗口尺寸只由 `app.js` 的 `fitWindowToContent()` 单向下发（rAF 合帧 + `lastWindowHeight` 去重）。

## P-11 窗口收不下去 / 从 JS 改 MinSize 后窗口顶边跳一下

- **状态**：fixed-in-launcher-tailwind-alpine-20260909
- **触发**：① `main.go` 的 `MinHeight` 大于前端「仅搜索框」的内容高度；② 在 JS 里调 `window.runtime.WindowSetMinSize()`。
- **症状**：① `WindowSetSize(780, 76)` 被静默夹到 380，窗口下方留一大块透明空白（用户看到的「透明框」真身）；② 调用瞬间窗口整体明显跳一下。
- **根因**：① darwin `SetSize` 走 `setFrame:`，受 NSWindow `userMinSize` 约束，而 `userMinSize` 由 `options.MinHeight` 在创建时初始化；② darwin `SetMinSize` 走 `adjustWindowSize()`，它做的 `setFrame:` **不重新锚定顶边**（对比 `SetSize` 会做 `origin.y += size.height - height`）。
- **修法**：`MinHeight` 降到 60（≤ 仅搜索框高度 76），与 `app.js` 的 `MIN_WINDOW_HEIGHT` 保持一致；`MinHeight` **只能**在 `main.go` 改，前端永不调 `WindowSetMinSize`。

## P-12 改 `:root` 字号导致整个 UI 缩放 6.25%

- **状态**：active（预防性）
- **触发**：为了「恢复旧版默认字号」把 `:root { font-size: 15px }` 加回去。
- **症状**：所有 padding / gap / width 一起变小（`p-3.5` 从 14px 变 13.125px），布局看着哪儿哪儿都不对，但没有任何报错。
- **根因**：Tailwind 的整套间距刻度基于 `rem`，改根字号等于全局缩放。
- **修法**：根字号保持浏览器默认 16px；默认字号用 shell 上的 `text-[15px]` 表达（只影响文字，不影响间距刻度）。

## P-13 Launcher 常驻持有 bbolt 独占锁 → CLI 卡死 / 重载工作区自锁死

- **状态**：fixed-in-2026-09-09（`core/store.go` + `core/library.go`）
- **触发**：① Launcher 运行时（常驻进程，`HideWindowOnClose`）在终端跑 `zoro search / preview / index` 操作**同一个库**；② 启动第二个 Launcher 实例；③ 「添加库后重载 workspace」。
- **症状**：① CLI **永久卡死**——无输出、无报错、只能 Ctrl-C；② 第二个实例同样卡死；③ Launcher 挂死自己（同进程不同 fd 也互斥）。全程没有任何错误信息，看起来像「程序坏了」。
- **根因**：`core.OpenStore` 曾调 `bolt.Open(path, 0o600, nil)`，`nil` 选项 → `Options.Timeout = 0`；bbolt 的 `flock`（`bolt_unix.go:17-45`）在 timeout 为 0 时是**无限重试循环**（每 50ms 一次，永不返回错误）。而 `Library.Query → ensureFresh()` 首次就会 `OpenStore`，store 之后**常驻不释放**。RW 模式取的是 `LOCK_EX` 独占锁。
- **修法（已落地）**：
  1. `bolt.Open` 传 `&bolt.Options{Timeout: 300 * time.Millisecond}`，把 `bolt.ErrTimeout` 映射成 `ErrStoreLocked`（「知识库索引正被另一个 zoro 进程占用（请先退出它）」）——争用**快速失败且有名字**。
  2. `Library` 从不跨调用持有 store：唯一入口是 `withStore(fn)`，Open → 用 → Close 在一次调用内完成。常驻进程因此不再有「忘记关」的状态。
  3. ⚠️ **配套必做**：`Library.Query` 是「尽力刷新」——拿不到锁就用内存里的块继续搜、**不报错**。加了超时之后症状从「卡死」变成「静默返回旧索引 = 搜不到刚写的内容」，更难查。所以 Launcher 的 `Status()` 显式 `RefreshAll()` 一次并把错误拼进返回串（`⚠️ 索引未刷新（结果可能是旧的）`）；那是用户唯一能知道这件事的地方。
  4. 内存指纹镜像的「已加载」判据是 `l.fps != nil` 而**不是** `len(l.fps) > 0`（空库的镜像 legitimately 是空 map，用 len 判会让空库每次都全量重扫）。

## P-14 想在 macOS 上「拖文件夹进搜索框添加库」——Wails v2 做不到

- **状态**：active（框架限制，非本项目 bug；已按修法落地）
- **触发**：在 Launcher 里实现文件/目录拖拽（`OnFileDrop`），期望 macOS 上可用。
- **症状**：`options.Options` 里找不到 `EnableDragAndDrop`；即使注册了 `runtime.OnFileDrop`，拖拽进来也**毫无反应**。
- **根因**：v2.15.0 的 drop 分支依赖 `window.chrome?.webview?.postMessageWithAdditionalObjects`（`internal/frontend/runtime/runtime_prod_desktop.js` 的 `CanResolveFilePaths`），那是 **Windows WebView2 专有 API**，WKWebView 上恒为 false；且 `pkg/options` 只有 `DisableResize`，没有开关可打开。
- **修法**：改用原生目录选择框 `runtime.OpenDirectoryDialog(ctx, OpenDialogOptions{...})`（`pkg/runtime/dialog.go:33`，**返回 `""` 表示取消，不是 error**）。已落地为桥接方法 `App.PickDirectory`（`ext/zoro-launcher/app.go`），由 `/lib add` 调用；前端把 `""` 当**取消**处理（`statusText = "已取消（没有选择目录）"`），绝不当错误显示成红字。⚠️ darwin 上它以 **sheet 形式挂在主窗口下**（`WailsContext.m:658`），对无边框 76px 浮窗的观感需 Mac 实测；兜底是让用户 ⌘V 粘贴路径。原生多窗口/更好的 drop 等 Wails v3（`../../todos.md` 9.6）。

## P-15 命令面出现空面板 = 用户眼里「工具坏了」

- **状态**：active（预防性；2026-09-09 由 jsdom 冒烟测试当场抓到）
- **触发**：给 `/verb` 命令面加新形态时，让「匹配不上」直接返回空数组；或写参数匹配规则时把「多打的实参」判成合法。
- **症状**：输入 `/lib add` 回车，跑的是**列库**而不是加库；或输入 `/lib xyz` 后面板整个消失、什么都不发生、没有任何提示。两种都没有报错。
- **根因**：① 候选列表是 `x-if="items.length > 0"` 驱动的，空数组 = 整块 DOM 被移出，静默失败；② 零参形态（`args: []`）如果用「实参数 ≥ 形态参数数」判 exact，会把任意多余实参吞掉，于是 `/lib add` 同时匹配上 `/lib`（exact）与 `/lib add <name>`（completion），而 exact 排在前面。
- **修法**：
  1. `buildCommandItems` 有**第二遍兜底**：verb 对得上但实参对不上时，列出该 verb 的全部形态，绝不返回空数组。
  2. `matchArgs` 里多打的实参**只有末尾是占位符的形态能吸收**，否则判 null。
  3. 排序 = exact 优先，同档内**参数多的形态优先**（`/lib add notes` 必须压过 `/lib`）。
  4. 改这块必须跑 jsdom 冒烟测试（`/tmp/lcheck/test.js` 的 [11]–[15] 节，不属于仓库）：候选排序是纯派生逻辑，只有断言能守住它。
