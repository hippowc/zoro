# todos.md — zoro 下一步任务清单

> 本文件只记录**下一步做什么**；设计决策一律以 [agents.md](agents.md)（第一版）为唯一事实源。
> 任务由近及远排列；每项标注归属：`core`（核心库）/ `cmd/zoro`（CLI 前端）/ `ext`（扩展 workspace）。

## 进度快照（截至 2026-09-07）

- ✅ 技术栈迁移：Rust + Tauri → **Go + Wails v2**，agents.md 精简定稿为「第一版」
- ✅ core 已迁移至 `core/`（Go package）：Block 统一标签模型 / Registry / manifest v2 / Workspace / 库级配置
- ✅ CLI MVP 落地至 `cmd/zoro`：`add`（添加知识库并写回 zoro.toml）/ `search` / `preview` / `open`（打开源文件）/ `html` / `text` / `index`；`serve` 仍为 P2 占位
- ✅ `ext/zoro-launcher` Wails 骨架：Query/Preview/Copy 桥接已就位
- ✅ 桌面 Launcher macOS MVP 已就绪（待用户 Mac 实测）：Carbon 全局热键 Cmd+Shift+Z（无辅助功能/输入监控权限）+ 无边框置顶透明浮窗 + 查询/预览/复制/打开源文件
- ✅ `go test ./...` 全绿（单测 + 集成）；示例工作区 `zoro index / search 定投 / preview 定投 / text 定投 / html 定投` 手动可用
- 当前等价阶段：**P0 完成；P1 收尾完成**（Matcher 接口 + 高亮区间 / HTML·ANSI·纯文本三 target / manifest 文件级指纹 / CLI 子命令 / 库级配置接入运行时分派）
- 下一阶段：P2 `zoro serve` 与 P3 CLI fzf + `@shell` Action（`allow_exec` helper 已就位，执行注册表到 P3 落地）

## 任务看板

### 一、立即：P1 收尾（core 地基冻结前置）

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 1 | ✅ 查询 matcher 明确为可插拔 `Matcher` 并补高亮区间 | core | 当前默认纯 Go 子序列模糊匹配；补齐 match ranges 供前端高亮（前端也可自行计算）；后续可接 `nucleo` 本轮无 UI 绑定；三面搜索见 agents.md §7，本轮只落 index terms 主面 |
| 2 | ✅ Markdown→ANSI/纯文本渲染 target | core | 粗体/代码块/标题上色；非 TTY 时 CLI 仍走纯文本；渲染管线多 target 框架落地 |
| 3 | ✅ manifest 脏检查升级为文件级指纹 | core | 当前是「库内任一 md 更新即全量重扫」；改为记录 `(path, mtime, size)` 指纹，缩小重建范围 |
| 4 | ✅ CLI 命令结构明确化 | cmd/zoro | 明确子命令：`add` / `search` / `preview` / `open` / `html` / `text` / `index` / `serve`；`add` 创建/写回 `zoro.toml`，`open` 按 `(库,path,start)` 打开源文件（`--print` 仅打印）；保留无参进 fzf 的目标 |
| 5 | ✅ 库级配置接入运行时行为 | core+cmd/zoro | `default` 库无参打开；`preview` 选择展示 target；`allow_exec` 参与 `@shell` 执行安全（当前仅解析保留，不生效） |

### 二、近期：P2 本地 Web + P3 CLI 前端（两个平级前端）

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 6 | `zoro serve` 本地 Web | ext/zoro-server | Go `net/http` 服务：浏览全量条目、搜索接口、命中渲染；浏览器可用，全程无终端；大众入口 |
| 7 | CLI + fzf 交互 | cmd/zoro | 候选行 `title<TAB>index<TAB>library<TAB>path<TAB>start`；`--with-nth` 显示 title，`--nth` 匹配 index；`--preview` 现场截取预览；回车全屏展示 |
| 8 | `@shell` 行为（按 §9 Action 分级落地） | core+cmd/zoro | core 定义 `Action` 接口；CLI 落 `ActionRegistry`：`Copy` 默认、`Execute` 确认、`Open`/`Preview` 按内容类型推断；`Insert` 回填平台可选延后 |
| 9 | 单二进制捆绑 fzf | cmd/zoro | release 资产按平台打包 fzf；新机器零安装可用；缺 fzf 时优雅降级非交互输出 |

### 三、中期：P3.5 Launcher + P4 全文搜索 + P5 静态站点

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 9.5 | ✅ Launcher 单平台 Spike（代码就绪，待 Mac 实测体验） | ext/zoro-launcher | Wails v2：常驻进程 + Carbon 全局热键（Cmd+Shift+Z）+ 无边框透明浮窗 + 自绘列表；动作「复制 + 打开渲染 + 打开源文件」；不回填；`./build-macos.sh` 在 Mac 上构建 |
| 9.6 | Launcher 三平台 + 回填（后续，视 Spike 结论） | ext/zoro-launcher | 回填按平台可选：macOS/Windows 可行，X11 凑合、Wayland 降级；GUI 大众分发走 Developer ID + 公证，延后到真正大众化阶段 |
| 10 | 全文搜索 | core | 三面搜索之正文兜底面（bleve/zinc 再定）：`index`=精确面、`title`=召回面、`raw`=兜底面（见 agents.md §7） |
| 11 | 静态站点发布 | ext/zoro-publish | 把库导出为静态 HTML + 站点搜索（pagefind）；个人站点/分享场景 |

### 四、远期：P6 同步 + P7 安全

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 12 | 同步插件 | core interface + ext | git 首发；`SyncProvider` 接口，后续 object_store/WebDAV 插件 |
| 13 | age 整库加密 | core interface + ext | 元数据/内容加密存储；密钥授权查看；`Cipher` 接口 |

## 待定回小区（backlog）

- **tag / facet**：标签名即 facet（`@shell`/`@video`…）；按 facet 过滤的 query 参数待做。
- **同主题块聚合**：`index` 块与同主题 `shell`/`video` 块命中并排显示的问题，留待 P3 交互前端设计。
- **稳定条目 id**：当前用 `(library, path, start)` 定位；跨编辑引用跳转需再引入内容锚点。
- **`@card` 等块组件**：语法已设计、渲染注册表未实现；等 Web/桌面 target 需要时再做。

## 下一步建议顺序

1. 先做 #1 matcher 高亮区间 + #2 ANSI target（core 冻结最后两块，性价比最高）。
2. 再做 #4 CLI 命令结构（把命令面稳住，为 P3 fzf 铺路）。
3. 然后 **P2 `zoro serve`** 与 **P3 fzf** 并行；期间抽一个 **P3.5 Launcher 单平台 Spike**（Wails + 全局热键）尽早验证「全局热键唤起」体验。
4. Launcher 正式三平台 + 回填，等 Spike 结论出来再排期；GUI 大众分发（Developer ID + 公证）延后；`@shell` 执行放 P3 后半段（安全确认一起做）。
