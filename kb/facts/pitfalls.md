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

- **状态**：active
- **触发**：只改 `frontend/dist/*.css` 的透明声明，不动 `main.go` 的原生窗口配置。
- **症状**：CSS 已 `background: transparent`，但窗口仍出背景块或深色块。
- **根因**：透明由“原生窗口层 + WebView 透明 + CSS 透明”三层共同决定，缺一层都不生效。
- **修法**：改动 UI 透明效果时三层一起检查：`main.go`（P-1）→ `frontend/dist/index.html` / `styles.css` → `app.js` 桥接结果判空（P-2）。
