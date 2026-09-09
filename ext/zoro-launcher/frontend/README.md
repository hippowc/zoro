# launcher 前端（Tailwind CSS + Alpine.js）

> 改 UI 前先读本文件。样式的**唯一源文件**是 `src/input.css`；`dist/styles.css` 是生成物。

## 文件角色

| 文件 | 谁维护 | 说明 |
|---|---|---|
| `src/input.css` | 手写 | 所有样式写在这里；5 节分层，每节的「放哪一层」是正确性问题，见文件内注释 |
| `dist/styles.css` | `npm run build` 生成 | 已提交（Go `//go:embed` 需要），**永不手改**：下次构建整文件覆盖 |
| `dist/index.html` | 手写 | Alpine 模板；结构即状态，全文档无 `id` 属性（用 `x-ref`） |
| `dist/app.js` | 手写 | 唯一的 Alpine 组件 `launcher()`：所有状态、所有桥接调用、窗口尺寸 |
| `dist/alpine.js` | vendor | `alpinejs@3.17.2/dist/cdn.min.js`，**永不编辑**；MIT |

## 命令

```bash
npm install     # 首次
npm run build   # 编译 CSS；wails build 通过 frontend:build 钩子自动跑（cwd 就是本目录）
npm run watch   # 边改边编译。wails dev 没接 watcher，需要时另开一个终端
```

## 七条防回归规则

1. **永不手改 `dist/styles.css`**：改 `src/input.css`，然后重新 build。
2. **运行时生成的 HTML 的样式必须放 input.css 第 5 节（任何 `@layer` 之外）**：`x-html` 注入的 Markdown 与 `highlight()` 的 `<mark>` 是运行时才产生的，Tailwind 扫描器看不到；放进 `@layer` 会被 purge 掉——构建照样成功，样式凭空消失。
3. **`<main x-ref="shell">` 子树里禁止任何视口高度**（`100vh` / `h-screen` / `height:100%`）：会与 ResizeObserver 形成「窗口高度 ↔ 内容高度」回路，窗口自激抖动。shell 高度必须纯内容驱动。
4. **JS 永不调 `window.runtime.WindowSetMinSize()`**：darwin 的 SetMinSize 不重锚顶边，窗口会跳一下；`MinHeight` 只能在 `main.go` 改。窗口尺寸的唯一写入点是 `fitWindowToContent()`，任何 action 之后都不要手动调它——ResizeObserver 已经会触发。
5. **`<head>` 里 `app.js` 必须排在 `alpine.js` 之前，两个都带 `defer`**：Alpine 文件末尾是 `queueMicrotask(() => Alpine.start())`，顺序反了会在 `window.launcher` 还不存在时求值 `x-data`，整页失效。
6. **getter 里只读不写**（`items` / `mode` / `rowKind` / `summaryText` / `hint` / `previewVisible` / `showFooter`）：Alpine 的响应式追踪会在渲染过程中执行它们，写状态 = 无限循环。派生值永远用 getter，不要加「记得同步」的数据字段。
7. **候选行只有一种形状**：`{ id, title, indexHtml, meta, action, payload? }`，`action ∈ "open-result" | "run-command" | "confirm-add" | "reveal-library"`。四个来源（搜索命中 / 命令候选 / 知识库列表 / 待确认添加）都产出它，`index.html` 只遍历 `items`、从不判断「现在是什么模式」。加一种视图 = 在 `app.js` 多一个来源，模板一行都不改。

## 命令面（`/` 前缀）

输入框第一个字符是 `/` 就进入命令模式（`get mode()` 从文本派生，不是开关）；开头留一个空格是逃生舱，`/root/zoro` 这类词仍然当搜索词用。

**加一条命令只有两步，没有别的接线点**：

1. 在 `COMMANDS` 注册表里加一个 `form`：`{ args, desc, action, options? }`。`args` 里的字面量（`"add"`）必须逐字匹配，尖括号（`"<name>"`）是占位符；只有占位符位置上的实参会传给 action。`options` 是占位符的枚举值，用户打完 verb 后会自动展开成一行一个值。
2. 在组件里写一个**与 `action` 同名**的方法，签名 `function (argv)`。

回车统一走 `runActive()` → 按行的 `action` 分派，所以键盘处理不用改。不可逆的动作（写 `zoro.toml`）必须先落到 `pendingAdd` 让用户确认一次；`Esc` 是一架梯子，一次退一层（待确认 → 列表 → 详情 → 隐藏窗口）。

## 跨文件常量耦合

| app.js | 对端 | 值 | 关系 |
|---|---|---|---|
| `MIN_WINDOW_HEIGHT` | main.go `MinHeight` | 60 | **硬耦合**：Go 侧不降，JS 的 `WindowSetSize` 会被 NSWindow 的 userMinSize 夹回去，窗口收不下来 |
| `WINDOW_WIDTH` | main.go `Width` | 780 | **硬耦合**：宽度永不自适应 |
| `MAX_WINDOW_HEIGHT` | — | 580 | 仅前端钳制上限（详情模式 568px 之上留余量）；`Height`(76) 只是初值，首帧由 ResizeObserver 纠正 |
| `EVENT_PROGRESS` | app.go `eventProgress` | `zoro:progress` | **硬耦合**：改名要两边一起改，否则长操作的进度条静默失效 |
| `COMMANDS[].action` | app.go 的桥方法 | — | `listLibraries`→`ListLibraries`、`addLibrary`→`PickDirectory`+`AddLibrary`、`reindex`→`Reindex`、`revealLibrary`→`RevealLibrary`。前端调之前先判 `bridge.X` 是否存在（前后端版本不一致时给可读提示，不静默失败） |

## 安全边界

`x-html` 只用于两处：`previewHtml`（Go `RenderHTML` 渲染用户自己的 Markdown）与行上的 `indexHtml`。`indexHtml` 只允许由 `highlight()` / `escapeHtml()` 产出，且**只在 `get items()` 里生成一次**——模板拿到的已经是安全字符串。查询词、错误串一律走 `x-text`（错误进独立字段 `previewError`，绝不写进 `previewHtml`）。
