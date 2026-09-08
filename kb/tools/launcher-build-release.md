# macOS Launcher 构建与发布（tools/launcher-build-release.md）

> 原子配方：本地/CI 构建 `ext/zoro-launcher` 与发布 Release。端到端见 `skills/ship-launcher-release.md`。

## 产物与目录

- 源码：`ext/zoro-launcher`（独立 go.mod，`replace zoro => ../..`）
- 本地产物：`ext/zoro-launcher/build/bin/zoro-launcher.app`（`build-macos.sh` 输出）
- CI 产物：`zoro-launcher-macos-arm64.zip` + `.sha256`

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
- runner：`macos-14`；Go `1.27.1`；Wails CLI `wails@v2.15.0`。
- 步骤：checkout → setup-go → install Wails CLI → `wails build -clean -platform darwin/arm64 -o zoro-launcher` → codesign（ad-hoc）→ 打包 zip + sha256 → softprops 发布 Release。
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

## 打包注意

- bin/ zoro-launcher 已是 GUI `.app`；ad-hoc 签名用 `codesign --force --deep --sign -`。
- 改 UI 透明/原生窗口时，三层一起核对：`main.go` 原生窗口配置 → `frontend/dist/index.html` / `styles.css` → `frontend/dist/app.js` 桥接值判空（见 `facts/pitfalls.md` P-1/P-2/P-6）。

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

### 键盘快捷键（v3.2+）

| 快捷键 | 动作 |
|--------|------|
| **Enter** | 在浏览器中弹出结果窗口 |
| **⌘+C** | 复制原始 Markdown 到剪贴板 |
| **⌘+Enter** | 用默认编辑器打开源文件 |
| **↑ / ↓** | 导航搜索结果 |
| **Esc** | 隐藏 launcher |

### 搜索结果汇总

搜索框下方会显示命中数量（如 "5 条命中"），无结果时自动隐藏。

### 注意事项

- 主题切换需重启 launcher 生效
- 配置文件不存在时使用默认主题 `light-glass`
- Popout 窗口通过系统浏览器实现（Wails v2 不支持多窗口）
