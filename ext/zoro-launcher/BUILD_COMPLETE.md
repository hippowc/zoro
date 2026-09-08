# 🎉 macOS Launcher 构建完成！

## ✅ 构建状态

GitHub Actions CI 构建已成功完成！

- **构建时间**: 2026-09-08 17:07 UTC
- **Tag**: `launcher-test-20260908-170448`
- **Runner**: macOS-14 (Apple Silicon)
- **产物大小**: ~4.8 MB

## 📥 下载链接

### Apple Silicon (M1/M2/M3) - 推荐

**主文件:**
```
https://github.com/hippowc/zoro/releases/download/launcher-test-20260908-170448/zoro-launcher-macos-arm64.zip
```

**SHA256 校验:**
```
https://github.com/hippowc/zoro/releases/download/launcher-test-20260908-170448/zoro-launcher-macos-arm64.zip.sha256
```

### 命令行下载

```bash
# 下载
curl -L -o zoro-launcher.zip \
  "https://github.com/hippowc/zoro/releases/download/launcher-test-20260908-170448/zoro-launcher-macos-arm64.zip"

# 验证 SHA256
curl -L -o zoro-launcher.zip.sha256 \
  "https://github.com/hippowc/zoro/releases/download/launcher-test-20260908-170448/zoro-launcher-macos-arm64.zip.sha256"
shasum -a 256 -c zoro-launcher.zip.sha256

# 解压
unzip zoro-launcher.zip
```

## 🚀 首次运行

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
- **辅助功能**: 用于全局热键（Cmd+Shift+Z）
- **文件访问**: 用于读取知识库文件

请在系统偏好设置中授予这些权限。

## 🎨 新功能测试清单

### 主题系统

创建配置文件 `~/.zoro/launcher.toml`:

```toml
# 尝试不同的主题
theme = "dark-glass"  # 或 "light-glass" | "minimal"
```

**测试步骤:**
1. 修改 theme 值为三个选项之一
2. 完全退出 launcher（Cmd+Q 或在 Activity Monitor 中结束进程）
3. 重新启动 launcher
4. 观察 UI 主题是否变化

**预期效果:**
- `light-glass`: 浅色毛玻璃，白色半透明背景
- `dark-glass`: 深色毛玻璃，深灰背景 + 浅色文字（更有质感）
- `minimal`: 扁平化设计，无模糊效果

### 弹出窗口功能

**测试步骤:**
1. 按 Cmd+Shift+Z 唤起 launcher
2. 输入搜索关键词（如 "git"）
3. 用 ↑↓ 方向键选择结果
4. 按 Enter 键

**预期效果:**
- 在默认浏览器中打开渲染好的 HTML 页面
- 窗口可以独立拖动和调整大小
- 可以同时打开多个结果进行对比

### 键盘快捷键

| 快捷键 | 动作 | 测试结果 |
|--------|------|---------|
| Enter | 弹出结果窗口 | [ ] |
| ⌘+C | 复制到剪贴板 | [ ] |
| ⌘+Enter | 打开源文件 | [ ] |
| ↑ / ↓ | 导航结果 | [ ] |
| Esc | 隐藏 launcher | [ ] |

### 搜索结果汇总

**测试步骤:**
1. 输入搜索词
2. 观察搜索框下方

**预期效果:**
- 显示 "X 条命中"
- 清空搜索词后，汇总信息消失
- 无结果时不显示汇总区域

## 📚 相关文档

- **BUILD_INSTRUCTIONS.md**: 本地构建指南
- **UI_FEATURES.md**: UI 功能详细说明
- **TEST_NEW_FEATURES.md**: 详细测试指南
- **kb/journal/2026-09-08-launcher-ui-enhancements.md**: 开发观察记录

## 🐛 问题反馈

如果遇到问题，请提供：

1. **macOS 版本**: `sw_vers -productVersion`
2. **Mac 架构**: `uname -m`（应该是 arm64）
3. **具体问题描述**和截图
4. **控制台日志**（如果有）:
   ```bash
   log stream --predicate 'process == "zoro-launcher"' --info
   ```

## 🧹 清理测试 Tag（可选）

测试完成后，可以删除测试 tag：

```bash
# 本地删除
git tag -d launcher-test-20260908-170448

# 远程删除
git push origin :refs/tags/launcher-test-20260908-170448
```

或者保留它作为参考。

---

**构建详情查看**: https://github.com/hippowc/zoro/actions/runs/34208019023

**Release 页面**: https://github.com/hippowc/zoro/releases/tag/launcher-test-20260908-170448
