# macOS Launcher 构建与发布（tools/launcher-build-release.md）

> 原子配方：本地/CI 构建 `ext/zoro-launcher` 与发布 Release。端到端见 `skills/ship-launcher-release.md`。

## 产物与目录

- 源码：`ext/zoro-launcher`（独立 go.mod，`replace zoro => ../..`）
- 本地产物：`ext/zoro-launcher/build/bin/zoro-launcher.app`（`build-macos.sh` 输出）
- CI 产物：`zoro-launcher-macos-arm64.zip` + `.sha256`

## 前端构建（Tailwind + Alpine）

前端不再是纯手写 CSS：`frontend/dist/styles.css` 是 **Tailwind 编译产物**，唯一源文件是 `frontend/src/input.css`。

- 依赖：**Node.js 18+**（本地 `brew install node`；CI 用 `actions/setup-node@v4` 的 node 22）。Tailwind 固定 `3.4.19`（v3 LTS，不用 v4）。
- Alpine 是 **vendor 文件** `frontend/dist/alpine.js`（`alpinejs@3.17.2/dist/cdn.min.js`），不进 `package.json`、不打包、运行时不联网。
- `wails.json` 两个钩子（**cwd 都是 `frontend/`**，所以不要写 `cd frontend`）：
  - `frontend:install`: `npm install`（`frontend/node_modules` 存在时 Wails 会跳过）
  - `frontend:build`: `npm run build`
- 改样式：只改 `frontend/src/input.css`，然后 `npm run build`（或直接 `wails build`）。**手改 `dist/styles.css` 会在下次构建被整文件覆盖**（`facts/pitfalls.md` P-7）。
- 目录角色、五条防回归规则、跨文件常量耦合表：`ext/zoro-launcher/frontend/README.md`（唯一事实源，此处不复制）。

## 本地构建（macOS Apple Silicon）

```bash
xcode-select --install                                  # 首次需装 Command Line Tools
cd ext/zoro-launcher
./build-macos.sh                                        # 产物：build/bin/zoro-launcher.app
open build/bin/zoro-launcher.app
```

或手动：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
cd ext/zoro-launcher
wails build -clean -platform darwin/arm64
# Intel Mac 把 arm64 换成 amd64
```

首次运行被 Gatekeeper 拦截时：

```bash
xattr -cr build/bin/zoro-launcher.app
open build/bin/zoro-launcher.app
```

> Wails 应用必须用 macOS 本机 Xcode 工具链构建；Linux 侧只能 `go build .` 做开发验证（不含全局热键）。

## CI 发布（当前正式路径）

Workflow：`.github/workflows/build-macos-launcher.yml`

- 触发：push tag 匹配 `launcher-*`，另可 `workflow_dispatch`。
- runner：`macos-14`；Go `1.27.1`；Node `22`（npm 缓存指向 `ext/zoro-launcher/frontend/package-lock.json`）；Wails CLI `wails@v2.15.0`。
- 步骤：checkout → setup-go → setup-node → install Wails CLI → `wails build -clean -platform darwin/arm64 -o zoro-launcher`（内部自动跑 `npm install` + `npm run build`）→ **Verify frontend assets** → codesign（ad-hoc）→ 打包 zip + sha256 → softprops 发布 Release。
- `Verify frontend assets` 步骤只干一件事：确认 `dist/styles.css` 非空且含 `glass-panel`（`@layer components`）与 `wails-draggable`（`@layer utilities`）。它防的是最危险的静默失败——Tailwind 跑成功但 `content` 路径写错，产出近乎空的 CSS，应用无样式却照样构建、签名、发布（`facts/pitfalls.md` P-8）。**删这一步等于放弃唯一的自动兜底。**
- 发布名：`zoro launcher（macOS Apple Silicon）<tag>`。
- 下载地址模板：
  ```
  https://github.com/hippowc/zoro/releases/download/<tag>/zoro-launcher-macos-arm64.zip
  ```

## 检查发布状态与 SHA256

```bash
# Actions 运行状态
curl -sL "https://api.github.com/repos/hippowc/zoro/actions/runs/<run_id>" | jq -r '.status, .conclusion'

# Release 资产
curl -sL "https://api.github.com/repos/hippowc/zoro/releases/tags/<tag>" | jq -r '.assets[] | [.name, .browser_download_url] | @tsv'

# SHA256
curl -sL "https://github.com/hippowc/zoro/releases/download/<tag>/zoro-launcher-macos-arm64.zip.sha256"
```

## 排障（macOS 本机）

```bash
# wails: command not found → GOPATH/bin 不在 PATH
export PATH=$(go env GOPATH)/bin:$PATH

# 抓运行日志：GUI 应用没有终端输出，这是让用户提供日志的唯一办法
log stream --predicate 'process == "zoro-launcher"' --info
```

> **不要**建议用户为全局热键授予「辅助功能 / 输入监控」权限：Carbon `RegisterEventHotKey` 注册的 Cmd+Shift+Z 不需要任何权限（AD-10）。只有「把内容回填到别的应用输入框」才需要 Accessibility，而该功能尚未做（AD-8）。旧文档里的错误授权指引已随文档删除。

## 打包注意

- bin/ zoro-launcher 已是 GUI `.app`；ad-hoc 签名用 `codesign --force --deep --sign -`。
- 改 UI 透明/原生窗口时，四层一起核对：`main.go` 原生窗口配置 → `frontend/src/input.css`（**不是** `dist/styles.css`，后者是生成物）→ `frontend/dist/index.html` → `frontend/dist/app.js` 桥接值判空（见 `facts/pitfalls.md` P-1/P-2/P-6）。
- **窗口高度自适应内容**（Spotlight 式）：前端 `fitWindowToContent()` 是唯一的窗口尺寸写入点，由 ResizeObserver 触发。`main.go` 的 `MinHeight`（60）必须 ≤ 「仅搜索框」时的内容高度（约 76px），否则 NSWindow 的 userMinSize 会把 `WindowSetSize` 的结果夹回去，窗口再也收不下去（P-11 讲的是它的另一半：从 JS 调 `WindowSetMinSize` 会让窗口顶边跳）。

## UI 主题配置（v3.2+）

Launcher v3.2 起支持可配置主题系统，通过 `~/.zoro/launcher.toml` 控制：

```toml
# 可选值: "light-glass" | "dark-glass" | "minimal"
theme = "dark-glass"
```

### 可用主题

| 主题名 | 风格 | 适用场景 |
|--------|------|----------|
| `light-glass` | 浅色毛玻璃，白色半透明背景 | 明亮环境、传统 macOS 风格 |
| `dark-glass` | 深色毛玻璃，深灰半透明背景 + 浅色文字 | 暗色环境、更有质感 |
| `minimal` | 扁平化设计，无模糊效果 | 性能优先、简洁偏好 |

> 三套主题的变量值**唯一源**是 `frontend/src/input.css` §1（`dist/styles.css` 是生成物，改主题只改 `src/input.css`）。前端启动时调 `app.go` 的 `GetTheme()` 拿到主题名，Alpine 用 `:data-theme="theme"` 挂在 `<html>` 上，CSS 按 `[data-theme="…"]` 选择器切换变量。

### 键盘快捷键（2026-09-09 Tailwind+Alpine 重构后据代码整理；Mac 实测待补）

| 快捷键 | 动作 |
|--------|------|
| **↑ / ↓** | 导航候选（首尾环绕）；选中即并排展示预览 |
| **Enter** | 进入**详情模式**（全屏预览，可拖拽整窗）；0 命中时不动作 |
| **⌘/Ctrl + Enter** | 用默认编辑器打开源文件 |
| **⌘/Ctrl + C** | 复制该块原始 Markdown 到剪贴板 |
| **Esc** | 详情模式 → 返回搜索；否则隐藏 launcher |
| **Cmd + Shift + Z** | 全局唤起 / 隐藏（Carbon 热键，见 AD-10） |

> 中文输入法组词期间的 Enter/Esc/方向键原样交给 IME（`app.js` `onKeydown` 第一句守卫），不会误触发详情模式。
> Go 侧的 `PopoutResult`（系统浏览器弹出）仍保留，但前端**未绑定任何按键或按钮**；`app.js` 里对应方法 `popoutActive()` 有注释说明，不要当死代码删掉。

### 搜索结果汇总

汇总（"N 条命中"）是**结果面板的头行**，与候选列表共用同一块玻璃、同一条边框，中间用 `border-b` 分隔；面板整体由 Alpine `x-if` 控制存在性，0 命中时整块从 DOM 移除（不是隐藏），因此不会留下透明空框。文案来自 `app.js` 的 `get summaryText()` getter，没有任何手写更新点。

### 注意事项

- 主题切换需重启 launcher 生效
- 配置文件不存在时使用默认主题 `light-glass`
- Wails v2 不支持多窗口，故 Popout 只能借系统浏览器；真正的原生多窗口等 v3 升级（`../../todos.md` 9.6）
