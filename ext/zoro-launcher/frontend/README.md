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

## 五条防回归规则

1. **永不手改 `dist/styles.css`**：改 `src/input.css`，然后重新 build。
2. **运行时生成的 HTML 的样式必须放 input.css 第 5 节（任何 `@layer` 之外）**：`x-html` 注入的 Markdown 与 `highlight()` 的 `<mark>` 是运行时才产生的，Tailwind 扫描器看不到；放进 `@layer` 会被 purge 掉——构建照样成功，样式凭空消失。
3. **`<main x-ref="shell">` 子树里禁止任何视口高度**（`100vh` / `h-screen` / `height:100%`）：会与 ResizeObserver 形成「窗口高度 ↔ 内容高度」回路，窗口自激抖动。shell 高度必须纯内容驱动。
4. **JS 永不调 `window.runtime.WindowSetMinSize()`**：darwin 的 SetMinSize 不重锚顶边，窗口会跳一下；`MinHeight` 只能在 `main.go` 改。窗口尺寸的唯一写入点是 `fitWindowToContent()`，任何 action 之后都不要手动调它——ResizeObserver 已经会触发。
5. **`<head>` 里 `app.js` 必须排在 `alpine.js` 之前，两个都带 `defer`**：Alpine 文件末尾是 `queueMicrotask(() => Alpine.start())`，顺序反了会在 `window.launcher` 还不存在时求值 `x-data`，整页失效。

## 跨文件常量耦合

| app.js | main.go | 值 | 关系 |
|---|---|---|---|
| `MIN_WINDOW_HEIGHT` | `MinHeight` | 60 | **硬耦合**：Go 侧不降，JS 的 `WindowSetSize` 会被 NSWindow 的 userMinSize 夹回去，窗口收不下来 |
| `WINDOW_WIDTH` | `Width` | 780 | **硬耦合**：宽度永不自适应 |
| `MAX_WINDOW_HEIGHT` | — | 580 | 仅前端钳制上限（详情模式 568px 之上留余量）；`Height`(76) 只是初值，首帧由 ResizeObserver 纠正 |

## 安全边界

`x-html` 只用于两处：`previewHtml`（Go `RenderHTML` 渲染用户自己的 Markdown）与 `highlightHtml`（先 `escapeHtml` 再只插 `<mark>`）。查询词、错误串一律走 `x-text`（错误进独立字段 `previewError`，绝不写进 `previewHtml`）。
