# Journal: 在 Linux 上用 jsdom 验证 Wails 前端 (2026-09-09)

**来源任务**: refactor(launcher): 前端重构为 Tailwind CSS + Alpine.js（tag `launcher-tailwind-alpine-20260909`）
**日期**: 2026-09-09
**状态**: 待固化（若下次改前端仍用得上，提议写成 `tools/launcher-frontend-verify.md`）

## 观察

- **J-1 无 Mac 也能跑通前端行为**：`npm i -D jsdom`，把真实的 `dist/index.html` + `dist/app.js` + vendor 的 `dist/alpine.js` 灌进 jsdom，桩掉 `window.go.main.App`（Query/LoadRaw/RenderHTML/Copy/OpenSource/Status/GetTheme/Hide）与 `window.runtime.WindowSetSize`，即可断言查询、高亮、上下键环绕、详情模式、IME 守卫、⌘↵/⌘C/Esc、0 命中、错误隔离等 47 项行为。脚本本次放在 `/tmp/lcheck/`（未入库）。
- **J-2 它真的抓到了 bug**：Tailwind preflight 把 h1–h6 字号与 ul/ol 列表符号重置掉、结果面板恒为 240px（应仅在预览出现时收窄）——两者都是构建全绿、只有渲染后才发现的问题。
- **J-3 窗口自适应链路也可断言**：jsdom 无 ResizeObserver，手写一个记录回调的桩 + 桩 `getBoundingClientRect`，就能验证 ResizeObserver → rAF 合帧 → 钳制 [60,580] → 去重 → `WindowSetSize` 整条链。
- **J-4 测试自身的坑**：Go 的 MatchRange 是 UTF-8 字节偏移，写断言时按字符数算会误判（`#定投` 是 0..7，不是 0..3）。三个 FAIL 全是测试写错，不是产品代码错——先怀疑夹具。

## 备注

- HTML 合法性另用 `golang.org/x/net/html` 做 round-trip 校验（Wails 会重新解析并序列化 index.html，非法结构会被静默改写）。
