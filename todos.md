# todos.md — zoro 下一步任务清单

> 本文件只记录**下一步做什么**；设计决策一律以 [agents.md](agents.md)（第一版）为唯一事实源。
> 任务由近及远排列；每项标注归属：`core`（核心库）/ `cmd/zoro`（CLI 前端）/ `ext`（扩展 workspace）。

## 进度快照（截至 2026-09-09）

- ✅ 技术栈迁移：Rust + Tauri → **Go + Wails v2**，agents.md 精简定稿为「第一版」
- ✅ core 已迁移至 `core/`（Go package）：Block 统一标签模型 / Registry / bbolt 元数据 store（`StoreSchema = 1`；`meta.go` 的 `Schema = 2` 是遗留 `meta.json` 的版本号，两者无关）/ Workspace / 库级配置
- ✅ CLI MVP 落地至 `cmd/zoro`：`add`（添加知识库并写回 zoro.toml）/ `search` / `preview` / `open`（打开源文件）/ `html` / `text` / `index`；`serve` 仍为 P2 占位
- ✅ `ext/zoro-launcher` Wails 骨架：Query/Preview/Copy 桥接已就位
- ✅ 桌面 Launcher macOS MVP 已就绪（**2026-09-09 用户 Mac 实测通过**）：Carbon 全局热键 Cmd+Shift+Z（无辅助功能/输入监控权限）+ 无边框置顶透明浮窗 + 查询/预览/复制/打开源文件
- ✅ Launcher 前端重构为 **Tailwind CSS + Alpine.js**，窗口高度随内容自适应（Spotlight 式，**Mac 实测通过**）：见看板 9.55 与 `kb/facts/decisions.md` AD-12
- ✅ **`v1.0.0` = 当前稳定基线**（用户 2026-09-09 验收：「超出了我的预期」）。版本号约定（`vMAJOR.MINOR.PATCH` 附注 tag，同时是 CI 触发器与 Release 名）与**四档回滚配方**见 `kb/tools/versioning-and-rollback.md`；主干直接开发、不开长期分支的依据见 AD-19
- ✅ **P0 store 独占锁已修**（原 🔴）：`bolt.Open` 加 300ms 超时 → `ErrStoreLocked`；`Library` 改为 `withStore(fn)`「用完即关」，从不跨调用持锁；刷新失败由 Launcher `Status()` 显式带出。见看板 5.5、`kb/facts/pitfalls.md` P-13
- ✅ **写入层落地**（「精准的增删改查」的地基）：`core/write.go` 块级增/改/删（三元组身份 + `expect` 乐观并发 + 就地写，三条写定律 L1–L3）、`core/configedit.go` 外科式改 `zoro.toml`（**注释与未知键活下来**）、`core.AddLibrary` 作为 CLI `zoro add` 与 Launcher `/lib add` 的**同一条链**。见 `kb/facts/architecture.md` 写入层、AD-15 / AD-16
- 🔄 Launcher **能力扩展方案部分落地**：输入框 `/verb` 命令面已实现 `/lib`、`/lib add`（原生目录框 + 确认行）、`/reindex`、`/theme`、`/help`；**未做**：`/lib rm`、`/lib default`、`/new`（片段捕获 UI，core 侧 `AppendBlock` 已就绪但前端无入口）、视图注册表、脑图视图。见看板 9.8–9.11、AD-17
- ✅ **可视化块（脑图/画布）方案已沉淀**：存储面永远是纯文本 Markdown，编辑面可以是可视化控件，中间按 kind 注册 codec；**core 不认识任何 kind 的内部结构**（节点级 API 已否决）。脑图 = Markdown 嵌套列表 + markmap（MIT），分两阶段，阶段 A 用**恒等 codec** 把「序列化幂等」风险直接消掉。D1–D6 **全部拍板**。见 AD-18、`kb/journal/2026-09-09-visual-blocks-storage-vs-editing-surface.md`
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
| 5.5 | ✅ **P0：store 独占锁**（已修，2026-09-09） | core + ext/zoro-launcher | 原症状：`OpenStore` 传 `nil` options → `Timeout=0` → 抢不到 flock 时**无限重试永不报错**，Launcher 开着时同库 `zoro search` 永久挂起。已落地：① `&bolt.Options{Timeout: 300*time.Millisecond}`，`bolt.ErrTimeout` → `ErrStoreLocked`（有名字的快速失败）；② `Library` 唯一入口 `withStore(fn)`，Open→用→Close 在一次调用内完成，**从不跨调用持锁**；③ `Query` 是「尽力刷新」，拿不到锁就用内存块继续搜且不报错，所以 Launcher `Status()` 显式 `RefreshAll()` 并把错误拼进返回串。见 `kb/facts/pitfalls.md` **P-13** |

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
| 9.55 | ✅ **前端架构重构：Tailwind CSS 3.4.19 + Alpine.js 3.17.2 + 窗口高度自适应**（代码就绪，待 Mac 实测） | ext/zoro-launcher | 目标是让反复出现的三类 UI bug 在**结构上不可能发生**：① Alpine `x-if` 把无结果区从 DOM 物理移除；② 窗口高度 = 内容高度（`fitWindowToContent()` 是唯一写入点，`main.go` MinHeight 380→60）；③ 单一 Alpine 状态源 + `get summaryText()` 派生汇总，零手写更新点。样式唯一源 `frontend/src/input.css`（`dist/styles.css` 是生成物）。见 `kb/facts/decisions.md` AD-12、`ext/zoro-launcher/frontend/README.md` |
| 9.6 | 🔄 **Wails v3 多窗口升级**（等待稳定版） | ext/zoro-launcher | **TODO**: 等 Wails v3 稳定版发布后，升级实现真正的原生多窗口 + Always on Top。当前「详情」是同窗口内的详情模式（Enter 进入）；Go 侧 `PopoutResult` 借系统浏览器，前端未绑定按键。参考：`kb/journal/2026-09-08-wails-v3-multiwindow-plan.md` |
| 9.7 | Launcher 三平台 + 回填（后续，视 Spike 结论） | ext/zoro-launcher | 回填按平台可选：macOS/Windows 可行，X11 凑合、Wayland 降级；GUI 大众分发走 Developer ID + 公证，延后到真正大众化阶段 |
| 9.8 | 🔄 **命令面：`/verb` 前缀命令**（框架 + 5 条命令已落地，`/lib rm`、`/lib default`、`/new` 待做） | ext/zoro-launcher | 已实现：`/lib`（列库）、`/lib add [name]`（原生目录框 → **确认行** → 写 `zoro.toml` → 重载 workspace → 立刻建索引）、`/reindex`、`/theme`（纯前端）、`/help`。判定不变：首字符是 `/`（无前导空格）且首个 token 在已注册动词表内 → 命令；否则仍是搜索（`/root/zoro` 照样能搜）；前导空格 = 逃生门；命令态**由文本派生**（`get mode()`），不另存内存状态。**四条不可破的不变量**：① 四种视图产出**同一种行形状** `{id,title,indexHtml,meta,action}`，模板里没有按模式分叉的 DOM；② Enter 只有 `runActive()` 一个出口；③ 不可逆写入必须过确认行（`pendingAdd`），取消不是错误；④ 面板永不为空（verb 对得上实参对不上时兜底列出全部形态）。见 `kb/facts/decisions.md` AD-17、`kb/facts/pitfalls.md` P-15、`ext/zoro-launcher/frontend/README.md` 命令面节 |
| 9.9 | **知识片段捕获 UI**（core 写 API 已 ✅，前端入口待做） | ext/zoro-launcher | core 侧已就绪：`core/write.go` 提供 `AppendBlock` / `UpdateBlock` / `DeleteBlock` / `PreviewDelete`（落点由 `CaptureFile()` 决定），身份是三元组 `(库, path, start)`，`expect` 做乐观并发，就地写（**不** tmp+rename），写完 `Analyze(false)` 让索引立刻可见——三条写定律 L1–L3 见 `kb/facts/architecture.md` 写入层、AD-15。**剩下的是前端**：`/new [kind]` 唤起捕获面板，复用详情模式的 glass-panel 外壳 + textarea，窗口自适应与 `x-if` 白拿；提交后必须 `resetResults()` + `doQuery()`，**禁止本地打补丁**（`start` 行号会漂，L1） |
| 9.10 | 🔄 **视图注册表 + 结果面板 CRUD**（core 写侧铁律已落地，前端注册表与动作待做） | ext/zoro-launcher + core | 两条正交轴：`target`（html/ansi/text）留在 core；`kind` → 视图留在前端 `zoroViews` 注册表（`{id, kind, title, maxHeight, actions, component(ctx)}`），窗口高度上限改为**由视图声明**。CRUD 三条铁律：① 一个动作一个 bridge 方法；② 任何写操作之后必须 `resetResults()` + `doQuery()`，**禁止本地打补丁**（`start` 行号会漂）；③ ~~core 写前必须按三元组重新定位并校验块~~ **已在 `core/write.go` 强制**（`resolveContentPath` + `expect` 前置校验，盲写行区间在 API 上不可能）。删除 = 真删 + 二次确认 + 靠 git 兜底（否决 `.zoro/trash/`，那是第二个内容源）。见方案文档 §6 |
| 9.11 | **脑图视图**（依赖 9.10；D5 已拍板 → AD-18） | ext/zoro-launcher | 载荷用 **Markdown 嵌套列表**（4 空格缩进会被 goldmark 当代码块、缩进的 `@` 行会被 `StripDirectives` 吃掉，所以只能用列表）；好处是 `RenderMarkdownHTML` 直接给出静态兜底渲染。可视化用 **markmap**（MIT `0.18.12`，已核验 npm：只有 lib/view/toolbar，**没有 editor 包** → 它是纯可视化，不给拖拽改结构；`markmap-view` 依赖 d3，vendor 前先量体积）。**分两阶段**：**A** textarea + 实时脑图预览——codec 是**恒等函数**，于是「序列化幂等/字节稳定」这个最难的验收标准自动满足、git diff 零噪声；**B** 拖拽节点——只有 A 用不顺才做，且**先评估 Tab/Shift-Tab 缩进**（A 的增量，同样不需要 codec）。若真做 B，`encode(decode(t))` 必须**逐字节相同**，做不到就不要上线。见 `kb/facts/decisions.md` AD-18、`kb/journal/2026-09-09-visual-blocks-storage-vs-editing-surface.md` §4 |
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
- **Launcher `DisableResize: true`**：窗口高度现在由内容驱动，用户手动拉伸理论上会与 `fitWindowToContent()` 打架；v1.0.0 实测未出现该反馈，**先不加**，真抖动了再开。
- **Launcher `popoutActive()` 未绑定**：Go 侧 `PopoutResult` 与前端方法都保留（`app.js` 有注释），等 Wails v3 多窗口或用户明确需求再决定绑到哪个键。

## 下一步建议顺序

1. **9.9 片段捕获 UI**（性价比最高）：core 写 API 已就绪，剩下纯前端——`/new` 命令 + 一个复用详情模式外壳的 textarea 面板。做完 Launcher 才真正从「只读搜索器」变成「能往里写东西的知识库入口」。
2. **9.10 视图注册表**：把「`kind` → 怎么展示 / 有哪些动作」从 if-else 里抽出来，是 9.11 的前置；顺带把删除/编辑动作接到已就绪的 `DeleteBlock` / `UpdateBlock` 上。
3. **9.11 脑图视图**：依赖 9.10 的注册表；载荷格式（D5）**已拍板**（AD-18：Markdown 嵌套列表 + markmap，阶段 A 恒等 codec），照 `kb/journal/2026-09-09-visual-blocks-storage-vs-editing-surface.md` §8 的五步做。
4. **并行轨**：P2 `zoro serve`（#6）与 P3 fzf（#7）互不依赖，可在上面任一步之间插入；`@shell` 执行（#8）放 P3 后半段，安全确认一起做。
5. **被上游阻塞**：9.6 Wails v3 多窗口等稳定版；9.7 三平台 + 回填等 Spike 结论。

> 每次交付仍走 `kb/skills/ship-launcher-release.md`（版本号 tag → CI → 通知用户下载）。Mac 实测清单见 `kb/tools/launcher-build-release.md`；**踩到新坑先补 `kb/facts/pitfalls.md` 再改代码**。
