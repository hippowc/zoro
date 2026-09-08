# Journal: Launcher UI Enhancements (2026-09-08)

**来源任务**: feat(launcher): 新增可配置主题系统、弹出窗口和搜索结果汇总  
**日期**: 2026-09-08  
**状态**: 待固化（需确认用户反馈后决定是否写入 facts/tools）

## 观察总结

### 1. Wails v2 不支持跨平台编译到 macOS
- **现象**: 在 Linux 环境执行 `wails build -platform darwin/amd64` 报错 "Crosscompiling to Mac not currently Supported"
- **根因**: Wails v2 依赖 macOS 原生 Xcode 工具链构建 `.app` bundle，无法在其他 OS 上交叉编译
- **解决方案**: 
  - 必须在 macOS 机器上本地构建
  - 或使用 GitHub Actions macOS runner（当前采用方案）
  - Wails v3 可能支持更好的跨平台支持（待验证）

### 2. Launcher UI 主题系统设计模式
- **实现方式**: CSS 自定义属性（variables）+ `[data-theme]` 选择器
- **三个预设主题**:
  - `light-glass`: 浅色毛玻璃（默认），白色半透明背景 + backdrop-filter blur
  - `dark-glass`: 深色毛玻璃，深灰半透明背景 + 浅色文字，更有质感
  - `minimal`: 扁平化设计，无模糊效果，纯色背景
- **配置加载**: Go 后端读取 `~/.zoro/launcher.toml`，通过 Wails bridge 传给前端
- **关键优势**: 早期应用主题避免 FOUC（Flash of Unstyled Content），无需重新构建即可切换

### 3. Popout 窗口的变通方案
- **问题**: Wails v2 不支持多窗口
- **变通方案**: 
  1. 渲染 Markdown 为 HTML
  2. 写入临时文件 `/tmp/zoro-popout-*/result.html`
  3. 用 `open` 命令在系统浏览器打开
- **优点**: 独立窗口、可拖动、可缩放、可利用浏览器功能
- **缺点**: 临时文件需手动清理；不是真正的原生窗口

### 4. 键盘快捷键的人机工程学调整
- **原设计**: Enter = 复制，Cmd+Enter = 打开源文件
- **新设计**: 
  - Enter = 弹出窗口（更频繁的操作）
  - Cmd+C = 复制（符合通用习惯）
  - Cmd+Enter = 打开源文件（保持不变）
- **理由**: 弹出窗口用于边工作边查看，使用频率高于复制；Cmd+C 是标准复制快捷键

### 5. 搜索结果汇总的渐进式披露
- **位置**: 搜索框正下方，结果列表上方
- **显示逻辑**: 
  - 有结果时显示 "X 条命中"
  - 无结果或空查询时完全隐藏（opacity: 0）
- **价值**: 提供即时反馈，不增加视觉噪音

## 待决策项

1. **临时文件清理策略**: 
   - 选项 A: 每次启动时清理旧的 popout 临时文件
   - 选项 B: 用户手动管理
   - 选项 C: 改为内存中渲染（需要 Wails v3 多窗口支持）

2. **主题扩展性**:
   - 是否允许用户自定义颜色值（而非仅从预设中选择）？
   - 是否需要暗色/亮色自动跟随系统？

3. **Popout 窗口数量限制**:
   - 是否限制同时打开的 popout 窗口数量（如最多 3 个）？
   - 是否需要窗口管理器（列出所有已打开的 popout）？

## 下一步

- [ ] 等待用户在真实 macOS 环境测试反馈
- [ ] 根据反馈决定是否固化为正式知识
- [ ] 如果主题系统稳定，写入 `tools/launcher-build-release.md` 或新建 `tools/launcher-ui-config.md`
- [ ] 如果 popout 方案被接受，记录到 `facts/pitfalls.md` 作为 "Wails v2 多窗口限制及变通方案"
