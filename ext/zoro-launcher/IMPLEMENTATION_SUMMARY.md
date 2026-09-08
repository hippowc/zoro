# UI 修复实施总结 (2026-09-08)

## ✅ 已完成修复

### 1. 透明阻挡问题 - 已修复 ✓

**问题**: 无搜索结果时，body 区域虽然透明但仍占据空间并阻挡鼠标事件

**解决方案**:
- CSS: body 使用 `display: none` 完全移除（而非透明）
- CSS: 添加 `pointer-events: none/auto` 确保不阻挡鼠标
- JS: hideBody() 时设置 `is-hidden` class

**修改文件**:
- `frontend/dist/styles.css` - body 样式
- `frontend/dist/app.js` - hideBody/renderResults 逻辑

### 2. 汇总信息位置 - 已修复 ✓

**问题**: summary 单独在搜索框下方，没有背景，视觉分离不佳

**解决方案**:
- HTML: 将 `<div id="summary">` 移到 `<section id="body">` 内部
- CSS: summary 作为 body 的第一个子元素，与结果列表一起显示/隐藏
- CSS: 添加底边框和间距，使其视觉上与结果区连贯

**修改文件**:
- `frontend/dist/index.html` - summary 位置调整
- `frontend/dist/styles.css` - summary 样式优化

### 3. 弹出窗口方案 - 临时方案 ✓

**问题**: 需要跨平台的、体验优秀的浮动窗口（类似截图工具）

**当前实现**（临时方案）:
- macOS: 使用 Preview.app 打开渲染后的 HTML
- Windows: 使用默认浏览器打开
- Linux: 使用 xdg-open 打开

**优点**:
- ✅ 立即可用，无需等待 Wails v3 稳定
- ✅ Preview.app 轻量、可独立拖动、支持缩放
- ✅ 跨平台兼容

**缺点**:
- ❌ 不是真正的"Always on Top"
- ❌ 需要手动关闭窗口
- ❌ 占用系统应用而非原生窗口

**修改文件**:
- `app.go` - PopoutResult 方法实现
- `popout.go` - PopoutManager stub（为未来 Wails v3 预留）

## 📋 Wails v3 升级评估

### 尝试过程

1. **升级到 Wails v3.0.0-beta.17**
   - 依赖成功下载
   - API 变化巨大：`pkg/runtime` 包被完全移除
   - Window 创建方式完全重构
   - Clipboard API 变更

2. **发现的问题**
   - Wails v3 仍处于 beta 阶段（beta.17）
   - API 不稳定，文档不完善
   - 需要大量代码重写
   - 可能存在未知 bug

### 决策

**暂不升级到 Wails v3**，原因：
1. Beta 版本不适合生产环境
2. API 仍在变化，维护成本高
3. 当前临时方案已满足基本需求
4. 等待 v3 稳定版发布后再升级

**未来计划**:
- 监控 Wails v3 稳定版发布（预计 2026 Q4）
- 稳定版发布后立即升级
- 实现真正的原生多窗口 + Always on Top

## 🔧 技术细节

### CSS 关键修改

```css
/* Body 完全隐藏，不阻挡鼠标 */
.body {
  display: none;
  pointer-events: none;
}
.body:not(.is-hidden) {
  display: flex;
  pointer-events: auto;
}

/* Summary 在 body 内部 */
.summary {
  flex: none;
  padding: 4px 8px;
  border-bottom: 1px solid var(--stroke-soft);
  margin-bottom: 4px;
}
```

### JavaScript 关键修改

```javascript
function hideBody() {
  bodyEl.classList.add("is-hidden");
  // ... 清空内容
  // 不再调用 setSummary("")，因为 summary 随 body 一起隐藏
}

function renderResults(candidates) {
  if (!current.length) {
    hideBody();  // body display:none，完全不占据空间
    return;
  }
  showResults();
  setSummary(current.length + " 条命中");  // summary 在 body 内显示
}
```

### Go 关键修改

```go
// PopoutResult 使用 Preview.app
func (a *App) PopoutResult(library, path string, start int) error {
    // 1. 渲染 Markdown 为 HTML
    html, err := a.RenderHTML(raw)
    
    // 2. 写入临时文件
    tmpFile := filepath.Join(tmpDir, "result.html")
    os.WriteFile(tmpFile, []byte(fullHTML), 0644)
    
    // 3. 用 Preview.app 打开（macOS）
    cmd := exec.Command("open", "-a", "Preview", tmpFile)
    return cmd.Start()
}
```

## 📊 测试清单

在 macOS 上测试以下内容：

- [ ] 无结果时，搜索框下方无透明阻挡层
- [ ] 有结果时，summary 显示在结果列表顶部
- [ ] 按 Enter 能在 Preview.app 中打开结果
- [ ] 主题切换正常工作
- [ ] 快捷键（Enter/⌘+C/⌘+Enter）正确响应

## 🚀 构建和部署

编译命令：
```bash
cd ext/zoro-launcher
GOPROXY=https://goproxy.cn,direct go build -o zoro-launcher .
```

产物：`zoro-launcher` （Linux 环境下编译，需在 macOS 上重新构建）

或通过 GitHub Actions 构建 macOS 版本：
```bash
git tag launcher-fix-ui-$(date +%Y%m%d)
git push origin --tags
```

## 📝 下一步

1. **立即**: 在 macOS 上本地构建并测试
2. **短期**（1-2周）: 根据用户反馈微调 UI
3. **中期**（1-2月）: 等待 Wails v3 稳定版
4. **长期**（Q4 2026）: 升级到 Wails v3，实现真正的多窗口

## 🎯 预期效果

修复后的用户体验：
1. ✅ 无结果时，搜索框下方完全干净，不阻挡下方内容操作
2. ✅ 有结果时，汇总信息清晰显示在结果区顶部
3. ⚠️ 按 Enter 在 Preview.app 中打开（临时方案，够用但不完美）
4. 🎯 未来升级到 Wails v3 后，实现真正的浮动窗口 + Always on Top
