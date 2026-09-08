# 测试新功能指南

## CI 构建状态

GitHub Actions 正在构建 macOS 应用。您可以通过以下链接查看进度：

🔗 [查看构建状态](https://github.com/hippowc/zoro/actions/runs/34208019023)

## 下载构建产物

构建完成后（大约 5-10 分钟），您可以从以下地址下载：

### 方法 1：通过 GitHub Releases 页面

访问：https://github.com/hippowc/zoro/releases/tag/launcher-test-20260908-170448

下载 `zoro-launcher-macos-arm64.zip`（Apple Silicon）或对应的 Intel Mac 版本。

### 方法 2：使用命令行下载

```bash
# Apple Silicon (M1/M2/M3)
curl -L -o zoro-launcher.zip \
  "https://github.com/hippowc/zoro/releases/download/launcher-test-20260908-170448/zoro-launcher-macos-arm64.zip"

# Intel Mac
curl -L -o zoro-launcher.zip \
  "https://github.com/hippowc/zoro/releases/download/launcher-test-20260908-170448/zoro-launcher-macos-amd64.zip"

# 解压
unzip zoro-launcher.zip
```

## 首次运行

### 1. 移除隔离属性（如果需要）

```bash
xattr -cr zoro-launcher.app
```

### 2. 打开应用

```bash
open zoro-launcher.app
```

### 3. 授予权限

首次运行时，macOS 会请求以下权限：
- **辅助功能**：用于全局热键（Cmd+Shift+Z）
- **文件访问**：用于读取知识库文件

请在系统偏好设置中授予这些权限。

## 测试清单

### ✅ 主题系统测试

1. **创建配置文件**

```bash
mkdir -p ~/.zoro
cat > ~/.zoro/launcher.toml <<'EOF'
theme = "dark-glass"
EOF
```

2. **测试三个主题**

修改 `~/.zoro/launcher.toml` 中的 `theme` 值，重启 launcher：

- `light-glass`：浅色毛玻璃（默认）
- `dark-glass`：深色毛玻璃（更有质感）
- `minimal`：扁平化设计（无模糊效果）

3. **验证点**
   - [ ] 主题在启动时正确应用
   - [ ] 搜索框、结果列表、预览区都使用新主题
   - [ ] 颜色搭配协调，没有突兀的元素

### ✅ 弹出窗口测试

1. **启动 launcher**（Cmd+Shift+Z）

2. **输入搜索关键词**

3. **选择结果并按 Enter**
   - [ ] 结果在浏览器中打开
   - [ ] 内容是渲染好的 HTML（不是原始 Markdown）
   - [ ] 浏览器窗口可以独立拖动和调整大小

4. **同时打开多个结果**
   - [ ] 可以同时打开多个浏览器标签页
   - [ ] 每个标签页显示不同的结果

### ✅ 键盘快捷键测试

| 快捷键 | 预期行为 | 测试结果 |
|--------|---------|---------|
| Enter | 在浏览器中弹出结果 | [ ] |
| ⌘+C | 复制原始 Markdown 到剪贴板 | [ ] |
| ⌘+Enter | 用默认编辑器打开源文件 | [ ] |
| ↑ / ↓ | 导航搜索结果 | [ ] |
| Esc | 隐藏 launcher | [ ] |

### ✅ 搜索结果汇总测试

1. **输入搜索词**
   - [ ] 搜索框下方显示 "X 条命中"
   - [ ] 数字与实际结果数量一致

2. **清空搜索词**
   - [ ] 汇总信息消失
   - [ ] 界面保持简洁

3. **无结果时**
   - [ ] 显示 "没有匹配"
   - [ ] 不显示结果区域

## 常见问题

### Q: 应用无法打开，提示"无法验证开发者"

**解决方案：**

```bash
xattr -cr zoro-launcher.app
open zoro-launcher.app
```

或在系统偏好设置 → 安全性与隐私 → 通用 中点击"仍要打开"。

### Q: 全局热键不工作

**解决方案：**

1. 检查辅助功能权限：
   系统偏好设置 → 安全性与隐私 → 隐私 → 辅助功能
   确保 zoro-launcher 已勾选

2. 重启应用

### Q: 主题没有切换

**解决方案：**

1. 确认配置文件路径正确：`~/.zoro/launcher.toml`
2. 确认 theme 值是以下之一：`light-glass`、`dark-glass`、`minimal`
3. 完全退出并重新启动 launcher

### Q: 弹出窗口没有打开

**解决方案：**

1. 检查浏览器是否被阻止（查看浏览器弹窗拦截设置）
2. 查看控制台日志：
   ```bash
   log stream --predicate 'process == "zoro-launcher"' --info
   ```

## 反馈问题

如果遇到问题，请提供：

1. macOS 版本：`sw_vers -productVersion`
2. Mac 架构：`uname -m`（arm64 = Apple Silicon, x86_64 = Intel）
3. 具体问题描述和截图
4. 控制台日志（如果有）

## 清理测试

测试完成后，可以删除测试 tag：

```bash
git tag -d launcher-test-20260908-170448
git push origin :refs/tags/launcher-test-20260908-170448
```

或者保留它作为参考。
