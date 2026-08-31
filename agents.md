# agents.md — zoro 通用知识库框架 · 开发指南（v9 定稿）

> 本文件是给「后续负责迭代开发的 agent / 人」读的**单一事实源**。
> 任何 agent 开始本仓库工作前，先读本文件；改动行为前先改本文件。
>
> 区分两个概念：
> - **运行时**：本地优先；一切网络能力均为可选插件，关掉即纯本地。
> - **开发期**：欢迎 agent 参与，但必须遵守下文"不改的设计"。
>
> 版本演进：
> - v6：单一索引文件 → 版本化元数据（manifest）；单内容根 → 多库工作区。
> - v7：**统一 Workspace 单一模型**——取消单库/多库两套入口，一律 `zoro.toml` 声明；引入**库级配置扩展**。
> - v8：新增**桌面前端 Launcher**（全局热键 + 浮窗，Tauri 2 + nucleo 自绘，不用 fzf）与 **macOS 分发/签名策略**。
> - v9：定稿 **Action 权限分级**（Copy/Open/Preview/Reveal/Execute/Insert）与 **v1 默认标签集**（仅 `@index` + `@cmd`；动作不新增标签）。

---

## 1. 定位与愿景

**zoro**：本地优先、可扩展的通用知识库框架。

- 核心 = 一个 Rust library（`zoro-core`）+ 一个最小 CLI 前端（`zoro`）。
- **非终端优先**：CLI / Web / Launcher 是三个平级前端——Web（`zoro serve`）是大众入口，CLI fzf 是终端效率入口，Launcher（全局快捷键唤起浮窗）是桌面神速入口，终端不是唯一形态。
- 愿景四步走：**标签体系 → Web 体验 → 站点/云/安全**（见 §12 产品路线图）。
- 内容源**只有一种**：Markdown（+ `@` 指令）。HTML 是渲染产物，不是源码。
- 运行三级流程：**内容（作者写）→ 元数据（analyze 产出）→ 消费（前端只读元数据 + 按需截原文）**。
- **一级对象是 Workspace**：工作区由 `zoro.toml` 声明多个 Library；单库只是列表长度为 1 的特例，不设特殊模式。

## 2. 核心边界（最重要的口径）

```
zoro-core（library，无 UI / 无 fzf / 无网络）
 ├─ 内容模型：Entry / Directive / Library / LibraryConfig / Workspace
 ├─ 工作区：Workspace ← zoro.toml 声明（库列表 + 库级配置）
 ├─ 分析（analyze）：扫描 @指令、分块 → 库级元数据（manifest）
 ├─ 元数据：库级 manifest（v1：索引视图 + 能力声明）+ mtime 脏检查
 ├─ 查询：Matcher trait（默认 nucleo）+ 候选模型
 ├─ 渲染：Markdown → AST/HTML/ANSI（多 target）
 └─ trait 扩展点：Render / Action / SyncProvider / Cipher
```

- **core 不依赖任何 UI**；fzf（仅限 CLI）、Web、Launcher、SSG 都是消费 core 的前端；Launcher 用 `nucleo` + 自绘列表，不引入 fzf。
- 依赖方向永远单向：`frontend → core`，绝不反向。
- **前端只消费元数据**，不直接扫正文；需要正文时按 `(库名, path, start)` 现场截取。
- 能力扩展走 trait；展示扩展 = 新 binary 依赖 `zoro-core`。

## 3. 原则（不可违背）

1. **目录自由，内容受限**：内容目录零约束；唯一技术约束在「`@` 标记行」格式上（§4）。
2. **Markdown 唯一源**：作者只写 Markdown + `@` 指令（含标签式组件）；HTML/CSS 属于产物与样式层。
3. **两个技术要素，单行标记（v1）**：`@index`（值型，必须）、`@cmd`（结构型，可选）。
4. **唯一强制物是库级元数据（manifest）**：版本化、可重建；索引只是它的一个视图。
5. **Workspace 单一模型**：一律由 `zoro.toml` 声明库集合；单库 = `libraries` 长度为 1，无特殊入口。
6. **库级身份三元组**：条目身份 = `(库名, path, start)`；库名必须出现在展示路径 / 候选行 / URL 中。

## 4. 条目格式（唯一内容契约 v1）

```markdown
## Git 丢弃本地修改（可读性标题，可选）

@index git checkout reset 丢弃 还原

@cmd
```bash
git checkout -- <file>
git reset --hard <commit>
```

正文说明。
```

### 4.1 指令分三形态（统一标签体系）

| 形态 | 内容从哪来 | 示例 |
|------|-----------|------|
| 值型 | 值本身 | `@index git checkout 丢弃` |
| 结构型 | 紧随的 Markdown 原生结构（fenced code block） | `@cmd` |
| 块组件型 | 紧随的 4 空格缩进块 | `@card title=概览` |

### 4.2 `@index`（值型，必须）

- 每条条目恰好一条，同时是**条目锚点**和**检索命中面**。
- 行首顶格 `^@index `，后跟至少一个词。

### 4.3 `@cmd`（结构型，可选）

- `@cmd` 后（跳过空行）必须紧跟 fenced code block；块内每行是该 `@cmd` 的负载。
- 语言取自 fence info string；一个 `@cmd` 绑一个块；无块则**警告并忽略**。
- v1：`@cmd` 被识别、校验、随正文展示；复制/执行见 §9 Action 扩展点（P3 落地）。

### 4.4 边界规则（现场推导，不存 end）

- **条目边界 = `@index` 行 → 下一个 `@index` 行 / 文件末尾**；`##` 不参与边界。

### 4.5 标题（title）规则

- `##` 紧贴 `@index` 上方；索引取"`@index` 向上最近的 `##`"作 `title`；无 `##` 取 `@index` 首词。

### 4.6 v1 默认标签集（最小集）

- 默认**只启用两个**：`@index`（值型，必须）、`@cmd`（结构型，可选）。
- 其余 `@xxx`（含 `@card`、`@alias` 等占名/未来标签）默认**不启用**：扫描器安全降级，不报错、不编译进 caps。
- **动作不新增标签**：`Copy`/`Preview`/`Open`/`Reveal` 由正文内容类型在渲染层推断或由运行时决策；`Execute` 由 `@cmd` 声明；`Insert`（回填）只是 `Execute` 的运行时策略。
- **新增标签的唯一判据**：出现"正文无法自动推断、且会改变消费方式"的语义，才加新标签。

## 5. 分析（analyze）与库级元数据（manifest）

### 5.1 定位

- `analyze` 把每个库的内容"编译"为该库的 manifest；一切查询/预览/执行以元数据为入口。
- **manifest 是派生物**：可删、可重建、幂等；真相永远在 Markdown + `@` 标签里。`.zoro/` 默认 gitignore，不入库。
- **需要正文时现场截取**：按 `(库名, path, start)` 截取到边界（边界现场推导）。元数据**不存 end、不存正文副本、不存配置**。

### 5.2 manifest 结构（schema v1）

- 文件位置：`<库根>/.zoro/meta.json`（隐藏目录，天然被扫描规则跳过）。
- **一个库一份**；workspace 的组织关系由 `zoro.toml` 承载，不写进库 manifest。

```json
{
  "schema": 1,
  "library": { "name": "sanji", "root": "/data/kb/sanji" },
  "generated_at": "1788166910",
  "entries": [
    {
      "title": "Git 丢弃本地修改",
      "index": "git checkout reset 丢弃 还原",
      "path": "git/常用操作.md",
      "start": 3,
      "caps": { "actions": [ { "kind": "cmd", "lang": "bash", "lines": [6, 7] } ] }
    }
  ]
}
```

- `library.name` 必须与 `zoro.toml` 声明一致（加载时校验）；`root` 仅作记录。
- `caps`（能力声明）由 `@` 标签编译而来，见 §5.3。

### 5.3 声明 vs 决策（关键边界）

**元数据只记录"内容声明了什么"；配置决定"工具怎么响应"。**

| 来源 | 进哪里 | 示例 |
|------|--------|------|
| 文档内 `@` 标签（`@cmd`/未来 `@preview`） | manifest.entries[].caps | 有 bash 块可执行 |
| 用户/机器偏好（默认库、预览偏好、是否允许执行） | `zoro.toml` 的库级配置 | preview=html、allow_exec=false |

- 库级配置在 `zoro.toml`（§6.3），**永不进 meta.json**：meta 删除重建不得丢失任何配置。
- 预览方式 / 执行方式的"策略"交给 `Render` / `Action` 注册表（§9）；manifest 里的 `caps` 是"有哪些能力可用"。

### 5.4 分块算法

逐行扫描；分块单位 = 条目。`@index` 开新条目、`@cmd` 记待绑定、fence 开==绑定、`##` 记最近标题、其余为正文。

### 5.5 生成与失效（预构建 + 脏检查）

- 主产物 `<库根>/.zoro/meta.json`；同时导出可读视图 `<库根>/zoro-index.tsv`（四列，可选分发/调试）。
- 查询前 mtime 脏检查，变更才重建；v1 可用"库内任一 `*.md` 晚于 meta 即 stale"，后续升级为文件级 fingerprint。
- 原子写：临时文件 + `mv`。

### 5.6 扫描范围

- 扫描对象 = Workspace 中**每个已声明 Library**；单库只扫其根下 `*.md` / `*.mdx`。
- 跳过隐藏目录（含 `.zoro/`、`.git/`）。

## 6. 工作区（Workspace）与库级配置

### 6.1 `zoro.toml`（工作区声明，作者维护，进 git）

```toml
# 可选：无参进入的默认库（按 name 指定）
default = "sanji"

[[libraries]]
name = "sanji"
root = "/data/kb/sanji"

# 库级配置（可选、可扩展）
[libraries.config]
preview = "html"          # 未来：默认预览 target
# allow_exec = false       # 未来：是否允许执行 @cmd

[[libraries]]
name = "robin"
root = "/data/kb/robin"
```

- `root` 支持相对路径，相对 `zoro.toml` 所在目录解析。
- **单库就是 `libraries` 只有一项**；不再有 `ZORO_ROOT` / `ZORO_LIBS`。
- CLI 查找顺序：`ZORO_WORKSPACE` 环境变量 → `./zoro.toml` → 报错。

### 6.2 Workspace 运行时

- `Workspace::from_toml_file(path)`：读声明 → 校验库名唯一 → 逐个库打开（冷启动优先）。
- 聚合查询跨库按分数排序；`load_raw(library, path, start)` 按三元组分发到对应库。

### 6.3 库级配置可扩展（LibraryConfig）

- 配置归属：**workspace 层**，不在内容解析层；未知键**前向兼容**（保留、不报错）。
- 模型：
  ```
  LibraryConfig {
    preview: Option<String>,            // 候选，未启用
    allow_exec: Option<bool>,           // 候选，未启用
    extra: <保留的未知键>               // 反序列化保留，前向兼容
  }
  ```
- 设计立场：库级配置只做"该库怎么被消费"的偏好，不做内容语义；内容语义一律走 `@` 标签。

## 7. 检索与匹配（fzf 属于前端，不在 core）

- **core**：`Matcher trait`（默认 `nucleo`）+ 候选模型 `Candidate{ library, title, index, path, start, match_ranges }`。
- **CLI 前端**：把全量 candidates 交给 **fzf** 交互（fzf 是该前端的默认控件，随前端捆绑）。
- **Web / Launcher 前端**：调用 `core.query(q)`，自绘输入框/列表/高亮；Launcher 是常驻进程 + 全局热键 + 无边框浮窗，匹配走 nucleo，不用 fzf。

fzf 三分离语义：**匹配字段（index）≠ 显示字段（title）≠ 载荷（(库,path)+start 即时预览）**。

## 8. 渲染管线与标签组件（多 target）

### 8.1 管线

```
Markdown 源
 ├─ 预处理：剥离 @指令（进元数据，不进 AST）
 ├─ pulldown-cmark 解析 → 事件流
 │   ├─ 普通块 → 标准 HTML
 │   ├─ fence + 已知语言 → syntect 高亮
 │   ├─ fence + mermaid/KaTeX → 前端渲染容器
 │   ├─ 标签组件(@card 等) → 组件注册表 → 自定义 HTML
 │   └─ raw HTML → sanitizer → 放行
 └─ 输出 target：HTML / ANSI / 纯文本
```

### 8.2 标签式组件（统一 `@` 语法）

```markdown
@card title=概览 size=large
    组件正文（可继续是 Markdown），
    用 4 空格缩进承载。
```

（组件注册表、未知组件安全降级、声明层与呈现层解耦、fence 只表真代码。）

### 8.3 样式与内嵌 HTML

- 样式 100% 由 CSS 主题层决定；一次性样式可用内嵌 HTML，但必须过 sanitizer。
- 不可信输入：关 raw HTML 透传或严格 sanitize，防 XSS。

### 8.4 表达力边界与逃生门

| 层 | 能力 | 边界 |
|----|------|------|
| L0 样式（CSS） | 外观/颜色/动画 | 无上限 |
| L1 行内排版 | 粗体/斜体/代码/链接/图片 + 少数内联扩展 | 内嵌 HTML span 兜底 |
| L2 块级结构 | Markdown 块 + `@` 标签组件 | 主力 |
| L3 精确/极端结构 | 一次性样式 | 内嵌 HTML（过 sanitizer） |
| L4 高度交互/应用 | 复杂交互界面 | 交给呈现层框架 |

## 9. 扩展机制

- **展示扩展** = 新 binary 依赖 `zoro-core`。
- **能力扩展** = core 定义 trait：`Action` / `SyncProvider` / `Cipher`。
- **动态加载暂缓**；执行安全：外部来源 `@cmd` 必须显式确认。

### 9.1 Action 权限分级（v9 定稿）

| 动作 | 权限 | 跨平台 | 定位 |
|------|------|--------|------|
| `Copy` 复制 | 无 | 一致 | 基础动作 |
| `Preview` 打开自家渲染视图 | 无 | 一致 | 主力 |
| `Open` 按内容类型调起系统应用 | 无 | 一致 | 主力 |
| `Reveal` 在文件管理器显示 | 无 | 一致 | 辅助 |
| `Execute` 调起终端执行 `@cmd` | 无特殊权限 | 可行 | 可选（需安全确认） |
| `Insert` 回填到前台应用 | macOS 需 Accessibility；Wayland 基本不可行 | 最差 | 平台可选，最后做 |

- 默认只保证 `Copy` + `Preview`/`Open`（三平台一致可用）。
- "打开"走系统 IPC（`NSWorkspace` / `ShellExecute` / `xdg-open`），不需要模拟输入权限；只有 `Insert` 涉及模拟输入。
- Launcher v1 只做 `Copy` + `Preview`/`Open`，不碰 `Insert`。

## 10. 目录布局（workspace）

```
zoro/                        # 本仓库（框架）
├── Cargo.toml
├── crates/
│   ├── zoro-core/
│   └── zoro/
├── ext/                     # zoro-server / zoro-launcher / zoro-publish
├── tests/fixtures/
├── README.md
├── agents.md
└── todos.md
```

内容库侧（示例）：

```
<workspace>/
├── zoro.toml                # 工作区声明（唯一入口，进 git）
├── sanji/
│   ├── .zoro/meta.json      # 库级元数据（派生物，gitignore）
│   ├── zoro-index.tsv       # 可读视图（可选分发）
│   └── ...任意 *.md
└── robin/ ...
```

## 11. 技术选型（Rust 生态）

| 能力 | 选择 |
|------|------|
| Markdown 解析 | `pulldown-cmark` + GFM 扩展 |
| 语法高亮 | `syntect` |
| 元数据序列化 | `serde` / `serde_json`（schema 版本化） |
| 工作区配置解析 | `toml` |
| 核心模糊匹配 | `nucleo`（纯 matcher） |
| CLI 交互 | `fzf`（前端捆绑，不进 core） |
| 全文搜索 | `tantivy`（P4） |
| 本地 Web | `axum`/`warp`（P2，ext） |
| 桌面壳 / Launcher | Tauri 2 + `tauri-plugin-global-shortcut`（ext/zoro-launcher）；匹配用 nucleo 自绘，不用 fzf |
| 站点搜索 | `pagefind`（P5，ext） |
| 同步 | git 首发，再 `object_store`/WebDAV（P6，ext） |
| 加密 | `age`/`rage`（P7，ext） |

## 12. 产品路线图（归属标注）

| 阶段 | 交付 | 归属 |
|------|------|------|
| P0 | 标签体系：`@index/@cmd/@card` 解析 + analyze 产出 manifest | core |
| P1 | 渲染管线：Markdown→HTML/ANSI（多 target） | core |
| P2 | 本地 Web：`zoro serve`（大众入口） | ext/zoro-server |
| P3 | CLI 前端：fzf 浏览 + `@cmd` 复制/执行（效率入口） | zoro |
| P3.5 | 桌面 Launcher：全局热键 + 浮窗，先单平台 Spike 验证体验 | ext/zoro-launcher |
| P4 | 全文搜索 tantivy | core |
| P5 | 静态站点发布 | ext/zoro-publish |
| P6 | 同步插件（git 首发） | core trait + ext |
| P7 | age 整库加密 + 密钥授权 | core trait + ext |

- Workspace 单一模型（`zoro.toml`）与库级配置属当前实际实现载体，随 P0/P1 落地。
- Launcher 分两步：v1 = 单平台 MVP（常驻 + 热键 + 浮窗 + nucleo 匹配 + 复制/打开渲染，不做回填）；v2 = 三平台 + 动作面板 + 回填（回填受平台限制，见 §14）。

## 13. 待定项（backlog）

### tag（重要，方向未定）

暂不实现 `@tag`。候选：目录名=tag / 构建期配置 / 目录级继承 / 库名级过滤。

### 稳定条目 id（暂缓）

当前用 `(库名, path, start)` 定位；需要跨编辑引用跳转时再引入内容锚点。

### 已推翻

- `ZORO_ROOT` / `ZORO_LIBS`：v7 起统一为 `zoro.toml`，单库不设特殊入口。
- `@run`：废除（行为外移为运行时扩展点）。
- `@lang`：可省（fence info string 已带语言）。

### 扩展指令占名（不实现）

`@alias`、`@desc`、`@ref`、`@hidden`。

## 14. 开发约定

- **不改设计**：§2–§9 的改动都是设计变更，先改本文件再动代码。
- **本地优先**：关掉网络/云/发布能力 = 纯本地工具。
- **性能目标**：未变更进入交互延迟近零；变更重建百毫秒级（千级条目）。
- **元数据是派生物**：`.zoro/` 默认 gitignore；禁止在 manifest 塞正文副本 / 环境状态 / 配置。
- **配置与内容分离**：偏好/策略进 `zoro.toml`，内容语义进 `@` 标签。
- **前向兼容**：未知 `@xxx`、未知组件、未知 fence 语言均安全降级；`LibraryConfig` 未知键保留不报错。
- **安全默认拒绝**：外部来源内容默认不执行、sanitize、显式确认。
- **测试轻量**：Rust `tests/` + 文本夹具。
- **平台分发策略（macOS 基准）**：开发/自用 = 本地构建 + ad-hoc 签名（免费）；CLI 大众分发 = Homebrew formula（源码编译，绕开 Gatekeeper）；GUI launcher 大众分发 = Developer ID 签名 + 公证（需 Apple Developer $99/年，延后到真正大众化再投入）。
- **Launcher 权限边界**：全局热键"唤起"通常无需特殊权限；"回填/模拟输入"需 Accessibility、且 Wayland 基本不可行，故按平台可选，默认只保证"复制到剪贴板 + 打开渲染"。

## 15. 验收标准（P0–P3 按阶段）

- [ ] 工作区单一模型：`zoro.toml` 单库/多库均可用；`ZORO_ROOT`/`ZORO_LIBS` 不再参与。
- [ ] 相对 `root` 按 `zoro.toml` 所在目录正确解析。
- [ ] 库级配置：`preview` 等已知键解析；自定义未知键保留、不报错。
- [ ] `analyze` 幂等：删 `.zoro/` 重建，结果逐字段一致。
- [ ] manifest 带 `schema`；条目含 `title/index/start/path/caps`；`zoro-index.tsv` 视图四列正确。
- [ ] 多库：两库同名条目不串库；候选行/展示路径带库名前缀；库名冲突报错。
- [ ] 连续两条 `@index` 条目展示正确、无粘连；`title` 规则正确；`@cmd` 正确绑定 fence。
- [ ] 未变更直接读元数据；某 `.md` 更新后自动重建。
- [ ] 【P3】CLI fzf 候选区显示标题；`--preview` 随选随显；回车全屏展示；无 TTY 可降级。
- [ ] 【P3】单二进制（捆绑 fzf）在新机器零安装跑通全链路。
- [ ] `zoro-core` 无 fzf / 无 UI 依赖（`cargo tree` 可验证）。
- [ ] 【P2】`zoro serve` 在浏览器完成浏览/搜索/渲染，全程无需终端。
- [ ] 【P3.5】Launcher 单平台 MVP：常驻进程 + 全局热键唤起 + 无边框浮窗 + nucleo 匹配 + 回车复制/打开渲染；无 fzf 依赖。
- [ ] 【P3.5 后】回填/执行能力平台可选：缺权限或不支持时降级为复制；"复制 + 打开"三平台一致可用。
- [ ] 默认标签集：扫描器仅启用 `@index`/`@cmd`；其余 `@xxx` 安全降级、不编译进 caps。
