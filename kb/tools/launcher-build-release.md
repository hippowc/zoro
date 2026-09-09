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

## 命令面（`/` 前缀，2026-09-09）

输入框是**双模**的，但模式**不是状态**——它由 `get mode()` 从当前文本派生（首字符是 `/` 且紧跟一个已注册 verb 才算命令），所以不存在「模式没切回来」这类 bug。开头留一个空格是逃生舱（`/usr/local` 这种路径仍能当搜索词）。

已实现的命令（唯一源是 `app.js` 的 `COMMANDS` 数组）：

| 命令 | 作用 | 后端 |
|------|------|------|
| `/lib` | 列出已声明的知识库（名字 / 根目录 / 块数 / 是否 default）。块数取自**内存**，不触发刷新、不抢 bbolt 锁 | `App.ListLibraries` |
| `/lib add [name]` | 加库：先弹**原生目录选择框**，选完出一行**确认行**，再 Enter 才真正写入 | `App.PickDirectory` → `App.AddLibrary`（→ `core.AddLibrary`） |
| `/reindex` | 强制重建全部库的索引 | `App.Reindex` |
| `/theme <name>` | 切换主题；`<name>` 会展开成三行枚举候选。**纯前端**（只赋值 `this.theme`，`:data-theme` 是绑定），**没有桥接方法、不写 `launcher.toml`**，重启后回到配置文件里的值 | — |
| `/help` | 列出全部命令（等价于把输入框设成 `/`） | — |

四条不变量（改这块前先读 `../ext/zoro-launcher/frontend/README.md` 的「命令面」节，权衡与否决项见 `facts/decisions.md` AD-17）：

1. **一种行形状**：搜索命中 / 命令 / 库列表 / 确认行都产出 `{id, title, indexHtml, meta, action}`，模板里没有任何按模式分叉的 DOM，漏一个分支在结构上不可能。
2. **Enter 只有一个出口**：`runActive()` 按 `action` 分发；新增命令 = 往 `COMMANDS` 加一项 + 在组件上写同名方法，不改模板、不改按键处理。
3. **不可逆写入必须过确认行**：`/lib add` 弹完目录框只是把 `pendingAdd` 挂上，真正写 `zoro.toml` 要用户再按一次 Enter；失败时**保留** `pendingAdd`，用户改个名字就能重试。取消（目录框返回 `""`）不是错误。
4. **面板永不为空**：verb 对得上但实参对不上时，兜底列出该 verb 的全部形态（`facts/pitfalls.md` P-15）。

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
| **↑ / ↓** | 导航当前列表（首尾环绕）；搜索命中时选中即并排展示预览 |
| **Enter** | 走 `runActive()` **同一个 switch**，动作由当前行类型决定：搜索命中 → **详情模式**（全屏预览，可拖拽整窗）；命令行 → 执行；库列表行 → 在 Finder 打开；确认行 → 真正写入 `zoro.toml`。0 命中且非命令时不动作 |
| **⌘/Ctrl + Enter** | 用默认编辑器打开源文件（仅搜索命中行） |
| **⌘/Ctrl + C** | 复制该块原始 Markdown 到剪贴板（仅搜索命中行） |
| **Esc** | **四级阶梯**，从最内层状态往外退：确认行 → 取消添加；数据视图（库列表）→ 关闭返回列表；详情模式 → 返回搜索；否则隐藏 launcher |
| **Cmd + Shift + Z** | 全局唤起 / 隐藏（Carbon 热键，见 AD-10） |

> 中文输入法组词期间的 Enter/Esc/方向键原样交给 IME（`app.js` `onKeydown` 第一句守卫），不会误触发详情模式或误执行命令。
> Go 侧的 `PopoutResult`（系统浏览器弹出）仍保留，但前端**未绑定任何按键或按钮**；`app.js` 里对应方法 `popoutActive()` 有注释说明，不要当死代码删掉。

### 汇总行（结果面板的头行）

汇总是**列表面板的第一行**，与候选行共用同一块玻璃、同一条边框，中间用 `border-b` 分隔；面板整体由 Alpine `x-if` 控制存在性，空列表时整块从 DOM 移除（不是隐藏），因此不会留下透明空框。

文案来自 `app.js` 的 `get summaryText()`，它按 `rowKind` 派生四种值——**没有任何手写更新点**：

| rowKind | 汇总文案 |
|---------|---------|
| `confirm` | `确认添加知识库` |
| `data` | 该数据视图自带的 `label`（`/lib` 的是 `N 个知识库`） |
| `command` | `N 条命令` |
| `result` | `N 条命中`（0 命中时为空串，此时面板已整体移除） |

### 注意事项

- `~/.zoro/launcher.toml` 只决定**启动时**的主题；运行中用 `/theme <name>` 即时换肤（`:data-theme` 是绑定），但**不写回配置文件**，重启后回到文件里的值
- 配置文件不存在时使用默认主题 `light-glass`
- Wails v2 不支持多窗口，故 Popout 只能借系统浏览器；真正的原生多窗口等 v3 升级（`../../todos.md` 9.6）

## Mac 实测清单（9.55 前端重构 + 9.8 命令面）

> 这两项在 Linux 侧只能做到「编译通过 + jsdom 断言通过」，GUI 行为必须在 Mac 上眼看。
> **任何一项不符合预期，先往 `facts/pitfalls.md` 补一条新坑（触发/症状/根因/修法），再改代码**——
> 静默修掉等于让下一个维护者重踩。

**窗口与透明**

- [ ] 唤起后窗口就是**一条搜索框那么高**（约 76px），下方没有任何透明空框或阴影残留
- [ ] 输入查询、结果出现时窗口**向下生长**，**顶边不动**（Spotlight 式）
- [ ] 清空输入（⌫ 或点 ✕）后窗口**收回一条搜索框**，无残留边框
- [ ] 结果很多时窗口最高停在 580px，列表内部滚动，**不抖动**（抖动 = ResizeObserver 自激，P-10）

**拖拽**

- [ ] 按住搜索框那一行能拖动整个窗口
- [ ] 进入详情模式（Enter）后，按住**预览区正文**也能拖动整窗（`.wails-drag` 靠自定义属性继承）
- [ ] 输入框内选文字、点 ✕、点返回按钮**不会**误触发拖拽（那些位置是 `wails-no-drag`）

**搜索与预览**

- [ ] ↑↓ 切换候选时，**结果列表宽度不变**（有预览时固定 16rem，无预览时占满），预览与列表**并排**而非上下
- [ ] 汇总行（`N 条命中`）与候选列表**同一块玻璃、同一条边框**，中间一条分隔线
- [ ] 命中词有 `<mark>` 高亮底色；预览区 Markdown 有样式（标题/列表/代码块/表格都成形）——两者任一失效 = P-8/P-9 复发
- [ ] ⌘↵ 用默认编辑器打开源文件；⌘C 复制该块原始 Markdown
- [ ] 详情模式 Esc 返回搜索；非详情模式 Esc 隐藏窗口；Cmd+Shift+Z 再次唤起
- [ ] **中文输入法**组词期间按 Enter 是确认候选词，**不会**进详情模式

**命令面**

- [ ] 输入 `/` → 列出全部命令；输入 `/lib` → 只剩 lib 的形态
- [ ] `/lib add` 回车 → **先弹原生目录选择框**（不是直接写文件）；点「取消」→ 状态栏是「已取消（没有选择目录）」，**不是红字错误**
- [ ] 选完目录 → 出现一行**确认行**（显示库名与路径），再按 Enter 才真正写入
- [ ] 加库成功后状态栏是「已添加知识库 X（N 个块）」，且**立刻能搜到新库里的内容**（不用重启）
- [ ] 用一个重名库再试 → 报「库名已存在」，**确认行还在**，改个名字按 Enter 能直接重试
- [ ] 手改过的 `zoro.toml`（带注释、带 `data_dir`）加库后**注释还在**
- [ ] `/lib` → 列出库名/根目录/块数/默认标记；在某一行按 Enter → Finder 打开该目录；Esc → 回到命令列表
- [ ] `/reindex` → 状态栏推进度、结束给结果；`/theme dark-glass` → **立刻换肤**
- [ ] `/lib xyz`（实参对不上）→ 面板**不消失**，而是列出 `/lib` 的全部形态（P-15）
- [ ] `/root/zoro` 这种以 `/` 开头的**路径仍然是搜索**，不进命令模式；` /lib`（前导空格）也是搜索

**主题**

- [ ] `light-glass` / `dark-glass` / `minimal` 三套都正常，前两套有毛玻璃模糊（失效 = 少了 `-webkit-backdrop-filter`，V20）
- [ ] 重启后回到 `~/.zoro/launcher.toml` 里的主题（`/theme` 不写回文件）
