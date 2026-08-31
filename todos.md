# todos.md — zoro 下一步任务清单

> 本文件只记录**下一步做什么**；设计决策一律以 [agents.md](agents.md)（v7）为唯一事实源。
> 任务由近及远排列；每项都标注归属：`core`（zoro-core）/ `zoro`（CLI 前端）/ `ext`（扩展 workspace）。

## 进度快照（截至 2026-08-31）

- ✅ 已提交：`05b2b0f` agents.md v6、`5fe1b40` v6 代码改造
- ✅ 已完成（待提交）：`agents.md` v7（Workspace 单一模型 + 库级配置扩展）+ 对应代码改造
- ✅ 已验证：14 单测 + 4 集成全绿；`zoro.toml` 单库 / 多库手动可用
- ✅ ZORO_ROOT / ZORO_LIBS 已移除，统一 `ZORO_WORKSPACE` → `./zoro.toml`
- 当前等价阶段：**P0 完成；P1 完成一半（HTML target 已做，ANSI 未做）**

## 任务看板

### 一、立即：P1 收尾（core 地基冻结前置）

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 1 | 查询 matcher 换 `nucleo` | core | 替换 `query.rs` 临时子序列匹配；保留 `Candidate` 模型与排序语义；用 `nucleo` 的 scorer 打分，`lib.query()` 行为不回归 |
| 2 | Markdown→ANSI 渲染 target | core | `render_markdown_ansi`：粗体/代码块/标题上色；非 TTY 时 CLI 仍走纯文本；渲染管线多 target 框架落地 |
| 3 | manifest 脏检查升级为文件级指纹 | core | 当前是“库内任一 md 更新即全量重扫”；改为记录 `(path, mtime, size)` 指纹，最小化重建范围 |
| 4 | CLI 命令结构明确化 | zoro | 把裸词优先级改为明确子命令（`search/index/serve/html`），避免 `zoro query git` 这类歧义；保留无参进 fzf 的目标 |
| 5 | 库级配置接入运行时行为 | core+zoro | `default` 库无参打开；`preview` 选择展示 target；`allow_exec` 参与 `@cmd` 执行安全（当前仅解析保留，不生效） |

### 二、近期：P2 本地 Web + P3 CLI 前端（两个平级前端）

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 6 | `zoro serve` 本地 Web | ext/zoro-server | axum/warp 服务：浏览全量条目、搜索接口、命中渲染；浏览器可用，全程无终端；大众入口 |
| 7 | CLI + fzf 交互 | zoro | 候选行结构 `title<TAB>index<TAB>library<TAB>path<TAB>start`；`--with-nth` 显示 title，`--nth` 匹配 index；`--preview` 现场截取预览；回车全屏展示 |
| 8 | `@cmd` 复制/执行 | core+zoro | core 定义 `Action` trait；CLI 落 `ActionRegistry`：复制（默认）、执行（外部来源必须显式确认） |
| 9 | 单二进制捆绑 fzf | zoro | release 资产按平台打包 fzf；新机器零安装可用；缺 fzf 时优雅降级非交互输出 |

### 三、中期：P4 全文搜索 + P5 静态站点

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 10 | tantivy 全文搜索 | core | index 之外的全文档检索通道；与 `@index` 检索分工（index=精确面，fulltext=兜底面） |
| 11 | 静态站点发布 | ext/zoro-publish | 把库导出为静态 HTML + 站点搜索（pagefind）；个人站点/分享场景 |

### 四、远期：P6 同步 + P7 安全

| # | 任务 | 归属 | 说明 / 验收 |
|---|------|------|-------------|
| 12 | 同步插件 | core trait + ext | git 首发；`SyncProvider` trait，后续 object_store/WebDAV 插件 |
| 13 | age 整库加密 | core trait + ext | 元数据/内容加密存储；密钥授权查看；`Cipher` trait |

## 待定回小区（backlog）

- **tag 体系**：先不定 `@tag`；方向候选 = 目录名 tag / 构建期配置 / 库名级过滤（与 Workspace 衔接）。
- **稳定条目 id**：当前用 `(library, path, start)` 定位；需要跨编辑引用跳转时再引入内容锚点。
- **`@card` 等块组件**：语法已设计、渲染注册表未实现；等 Web/桌面 target 需要时再做。
- **README 同步实现状态**：README 仍偏愿景描述，可在 P2 落地时一并更新。

## 下一步建议顺序

1. 先做 #1 `nucleo` + #2 `ANSI`（core 冻结最后两块，性价比最高）。
2. 再做 #4 CLI 命令结构（把命令面稳住，为 P3 fzf 铺路）。
3. 然后 **P2 `zoro serve`**（用户基数大、是定位“非终端优先”的关键一步）与 **P3 fzf** 并行。
4. `@cmd` 执行放到 P3 后半段（安全确认机制一起做）。
