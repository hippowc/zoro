# Wails v3 多窗口升级方案

**日期**: 2026-09-08  
**状态**: 规划阶段（等待用户确认）  
**来源任务**: UI 弹出窗口体验优化需求

## 需求背景

用户需要类似于截图工具的浮动窗口：
1. **始终在最前方** - Always on Top
2. **跨平台支持** - macOS + Windows
3. **体验优秀** - 可拖动、可缩放、独立生命周期
4. **简洁轻量** - 不占用浏览器标签页

## 为什么选择 Wails v3

### Wails v2 的限制
- ❌ 单 WebView 实例，无法创建多个窗口
- ❌ 没有原生的多窗口 API
- ❌ 跨平台窗口管理依赖 OS 特定实现
- ❌ 无法实现真正的 Always on Top

### Wails v3 的优势
- ✅ **原生多窗口支持** - `runtime.WindowCreate()` API
- ✅ **跨平台统一** - macOS/Windows/Linux 一致行为
- ✅ **Always on Top** - `WindowSetAlwaysOnTop(ctx, windowID, true)`
- ✅ **无边框透明** - 完美适配毛玻璃 UI
- ✅ **性能提升 40%** - 体积仅 12MB
- ✅ **已正式发布** - 2026-08-04 发布 v3.0.0

参考：[Wails v3 发布公告](https://tonybai.com/2026/08/04/wails-v3-go-desktop-framework/)

## 架构设计

### 窗口层级

```
┌─────────────────────────────────────────────┐
│ 主窗口 (Launcher)                            │
│ - Frameless + AlwaysOnTop                   │
│ - Cmd+Shift+Z 唤起/隐藏                      │
│ - 搜索 + 结果列表 + 预览                     │
│                                              │
│ 按 Enter → 创建浮动窗口                      │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│ 浮动窗口 #1 (Popout)                         │
│ - Frameless + AlwaysOnTop                   │
│ - 独立位置、独立大小                         │
│ - 可拖动、可缩放                             │
│ - 渲染 HTML 内容                             │
│ - 右上角关闭按钮                             │
└─────────────────────────────────────────────┘

可同时创建多个浮动窗口（建议限制最多 3 个）
```

### 窗口管理策略

```go
type PopoutManager struct {
    windows map[string]int64  // windowID -> creationTime
    maxCount int              // 最大窗口数（默认 3）
}

func (pm *PopoutManager) Create(ctx context.Context, html string) error {
    if len(pm.windows) >= pm.maxCount {
        // 关闭最旧的窗口
        pm.closeOldest(ctx)
    }
    
    windowID, err := runtime.WindowCreate(ctx, &options.Window{
        Title:         "zoro result",
        Width:         800,
        Height:        600,
        MinWidth:      400,
        MinHeight:     300,
        Frameless:     true,
        AlwaysOnTop:   true,
        HideWindowOnClose: false,
    })
    
    if err != nil {
        return err
    }
    
    pm.windows[windowID] = time.Now().Unix()
    
    // 加载 HTML 内容
    runtime.WindowLoadHTML(ctx, windowID, html)
    runtime.WindowShow(ctx, windowID)
    
    return nil
}
```

## 实施步骤

### Phase 1: 依赖升级（1-2 天）

1. **更新 go.mod**
   ```bash
   cd ext/zoro-launcher
   go get github.com/wailsapp/wails/v3@v3.0.0
   go mod tidy
   ```

2. **更新 main.go**
   - 修改 import 路径：`github.com/wailsapp/wails/v3`
   - 更新 options 结构（v3 API 变化）
   - 添加 WindowCreate 调用

3. **测试基础功能**
   - 确保主窗口正常启动
   - 验证热键工作正常
   - 检查主题加载

### Phase 2: 多窗口实现（2-3 天）

1. **创建 PopoutManager**
   - 文件：`ext/zoro-launcher/popout.go`
   - 功能：窗口创建、生命周期管理、数量限制

2. **修改 App.PopoutResult**
   ```go
   func (a *App) PopoutResult(library, path string, start int) error {
       raw, err := a.LoadRaw(library, path, start)
       if err != nil {
           return err
       }
       
       html, err := a.RenderHTML(raw)
       if err != nil {
           return err
       }
       
       return a.popoutManager.Create(a.ctx, html)
   }
   ```

3. **创建浮动窗口 HTML 模板**
   - 文件：`frontend/dist/popout.html`
   - 简洁布局：标题栏 + 内容区 + 关闭按钮
   - 继承主题系统

### Phase 3: UI 优化（1-2 天）

1. **浮动窗口样式**
   - 毛玻璃效果（与主窗口一致）
   - 可拖动区域（标题栏）
   - 关闭按钮（右上角）
   - 响应式布局

2. **窗口管理 UI**
   - 显示当前打开的浮动窗口数量
   - 提供"关闭所有"按钮
   - 快捷键支持（Cmd+W 关闭当前）

3. **动画效果**
   - 窗口出现/消失动画
   - 平滑过渡

### Phase 4: 跨平台测试（1-2 天）

1. **macOS 测试**
   - Apple Silicon (arm64)
   - Intel Mac (amd64)
   - 验证 Always on Top 行为
   - 检查毛玻璃效果

2. **Windows 测试**
   - Windows 10/11
   - 验证窗口置顶
   - 检查 DPI 缩放

3. **Linux 测试**（可选）
   - Ubuntu 22.04+
   - GNOME/KDE 桌面

## 代码示例

### main.go (Wails v3)

```go
package main

import (
    "context"
    "embed"
    
    "github.com/wailsapp/wails/v3/pkg/application"
    "github.com/wailsapp/wails/v3/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
    app := application.New()
    
    mainWindow := app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
        Title:         "zoro",
        Width:         780,
        Height:        580,
        MinWidth:      560,
        MinHeight:     380,
        Frameless:     true,
        AlwaysOnTop:   true,
        Hidden:        true,
        BackgroundColour: application.NewRGB(27, 38, 54),
        Assets:        assets,
    })
    
    mainApp := NewApp()
    mainWindow.Bind(mainApp)
    
    mainWindow.OnDomReady(func(ctx context.Context) {
        mainApp.startup(ctx)
        mainApp.domReady(ctx)
    })
    
    mainWindow.OnWindowClose(func(ctx context.Context) bool {
        mainApp.shutdown(ctx)
        return false // 不关闭窗口，只是隐藏
    })
    
    app.Run()
}
```

### popout.go

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/wailsapp/wails/v3/pkg/application"
    "github.com/wailsapp/wails/v3/pkg/runtime"
)

type PopoutManager struct {
    app      *application.App
    windows  map[string]int64
    maxCount int
}

func NewPopoutManager(app *application.App) *PopoutManager {
    return &PopoutManager{
        app:      app,
        windows:  make(map[string]int64),
        maxCount: 3,
    }
}

func (pm *PopoutManager) Create(ctx context.Context, html string) error {
    if len(pm.windows) >= pm.maxCount {
        pm.closeOldest(ctx)
    }
    
    window := pm.app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
        Title:         "zoro result",
        Width:         800,
        Height:        600,
        MinWidth:      400,
        MinHeight:     300,
        Frameless:     true,
        AlwaysOnTop:   true,
        Hidden:        false,
        BackgroundColour: application.NewRGB(255, 255, 255),
    })
    
    windowID := window.GetID()
    pm.windows[windowID] = time.Now().Unix()
    
    window.LoadHTML(html)
    window.Show()
    
    window.OnWindowClose(func(ctx context.Context) bool {
        delete(pm.windows, windowID)
        return false
    })
    
    return nil
}

func (pm *PopoutManager) closeOldest(ctx context.Context) {
    var oldestID string
    var oldestTime int64 = 9999999999
    
    for id, t := range pm.windows {
        if t < oldestTime {
            oldestTime = t
            oldestID = id
        }
    }
    
    if oldestID != "" {
        runtime.WindowClose(ctx, oldestID)
        delete(pm.windows, oldestID)
    }
}

func (pm *PopoutManager) CloseAll(ctx context.Context) {
    for id := range pm.windows {
        runtime.WindowClose(ctx, id)
    }
    pm.windows = make(map[string]int64)
}
```

## 风险评估

### 技术风险

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| Wails v3 API 不稳定 | 低 | 中 | 使用稳定版 v3.0.0，避免 beta 特性 |
| 跨平台行为不一致 | 中 | 中 | 充分测试 macOS + Windows |
| 内存泄漏（多窗口） | 低 | 高 | 严格管理窗口生命周期，设置最大数量 |
| 性能下降 | 低 | 中 | Wails v3 性能更好，监控资源使用 |

### 时间风险

- **乐观估计**: 5-7 天完成
- **保守估计**: 10-14 天（包含测试和 bug 修复）
- **风险因素**: Wails v3 学习曲线、跨平台兼容性问题

## 替代方案对比

如果 Wails v3 升级遇到困难，备选方案：

### 备选 A: Electron 辅助进程
- 用 Electron 创建浮动窗口
- Go 后端通过 IPC 通信
- ❌ 缺点：增加依赖、体积变大、复杂度提高

### 备选 B: 系统原生窗口
- macOS: NSWindow (Swift/Objective-C)
- Windows: Win32 API
- ❌ 缺点：需要维护两套代码、非跨平台

### 备选 C: 等待 Wails v3 更成熟
- 继续使用浏览器方案
- 等 3-6 个月后 Wails v3 生态更完善
- ❌ 缺点：延迟交付、用户体验不佳

**推荐**: 直接采用 Wails v3 主方案，备选 C 作为降级选项。

## 下一步行动

1. **用户确认方案** - 是否同意升级到 Wails v3？
2. **创建分支** - `feature/wails-v3-multiwindow`
3. **开始 Phase 1** - 依赖升级和基础测试
4. **每周同步进度** - 在 journal 中记录进展

## 参考资料

- [Wails v3 官方文档](https://wails.io/docs/v3/)
- [Wails v3 发布公告](https://tonybai.com/2026/08/04/wails-v3-go-desktop-framework/)
- [Wails v3 GitHub](https://github.com/wailsapp/wails/tree/v3)
- [Wails Runtime API](https://wails.io/docs/v3/runtime/intro)
