# 在 macOS 上构建 zoro-launcher

## 前提条件

在您的 macOS 机器上执行以下步骤：

### 1. 安装 Xcode Command Line Tools

```bash
xcode-select --install
```

### 2. 安装 Go 1.27+

从 https://go.dev/dl/ 下载并安装 Go，或使用 Homebrew：

```bash
brew install go
```

验证安装：

```bash
go version  # 应该显示 go1.27.x 或更高版本
```

### 3. 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
```

## 构建步骤

### 方法 A：使用构建脚本（推荐）

```bash
cd /path/to/zoro/ext/zoro-launcher
./build-macos.sh
```

这会自动：
- 检查并安装 Wails CLI（如果未安装）
- 构建 darwin/arm64（Apple Silicon）或 darwin/amd64（Intel）应用
- 进行 ad-hoc 签名
- 输出到 `build/bin/zoro-launcher.app`

### 方法 B：手动构建

```bash
cd /path/to/zoro/ext/zoro-launcher

# Apple Silicon (M1/M2/M3)
wails build -clean -platform darwin/arm64 -o zoro-launcher

# Intel Mac
wails build -clean -platform darwin/amd64 -o zoro-launcher
```

## 运行应用

构建完成后：

```bash
open build/bin/zoro-launcher.app
```

如果首次运行被 Gatekeeper 拦截：

```bash
xattr -cr build/bin/zoro-launcher.app
open build/bin/zoro-launcher.app
```

## 配置主题

创建或编辑 `~/.zoro/launcher.toml`：

```toml
# 可选值: "light-glass" | "dark-glass" | "minimal"
theme = "dark-glass"
```

## 新功能测试清单

构建完成后，请测试以下功能：

1. **主题切换**
   - [ ] 修改 `~/.zoro/launcher.toml` 中的 theme 值
   - [ ] 重启 launcher，确认主题已切换
   - [ ] 测试三个主题：light-glass, dark-glass, minimal

2. **弹出窗口**
   - [ ] 输入搜索关键词
   - [ ] 用方向键选择结果
   - [ ] 按 Enter，确认在浏览器中打开结果

3. **键盘快捷键**
   - [ ] Enter：弹出结果窗口
   - [ ] ⌘+C：复制到剪贴板
   - [ ] ⌘+Enter：打开源文件
   - [ ] Esc：隐藏 launcher

4. **搜索结果汇总**
   - [ ] 输入搜索词后，搜索框下方显示 "X 条命中"
   - [ ] 清空搜索词后，汇总信息消失

## 故障排除

### 找不到 wails 命令

确保 GOPATH/bin 在 PATH 中：

```bash
export PATH=$(go env GOPATH)/bin:$PATH
```

### 构建失败：缺少依赖

```bash
cd /path/to/zoro/ext/zoro-launcher
go mod download
go mod tidy
```

### 应用无法启动

检查控制台日志：

```bash
log stream --predicate 'process == "zoro-launcher"' --info
```

### 全局热键不工作

确保在 macOS 系统偏好设置中授予了辅助功能权限：
系统偏好设置 → 安全性与隐私 → 隐私 → 辅助功能 → 勾选 zoro-launcher
