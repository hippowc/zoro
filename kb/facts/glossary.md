# zoro 术语表（facts/glossary.md）

> 新 agent 免猜；20–40 行量级。术语以 `core` 代码为准，本表是快速对齐。

| 术语 | 含义 |
|---|---|
| Workspace / 工作区 | `zoro.toml` 声明的一组知识库（Library）集合 |
| Library / 知识库 | 一个被声明的内容目录，有 `name` 与 `root`，可带库级配置 |
| Block / 块 | 统一标签模型的最小内容单元：kind（标签名）+ terms（搜索词）+ raw（负载） |
| kind / 标签名 | 块语义类型，如 `index` / `shell` / `video`；登记在 `core/registry.go` |
| terms / index | `@` 标签行的搜索词，主搜索面；按空白拆词、词间 AND、词内子序列匹配 |
| raw / 原文 | 块的负载正文，渲染与全文检索的原料，不存进 manifest |
| `@index` | 叙述/定位块，负载是到下一标签之间的正文 |
| `@shell` | 终端执行块，负载是紧随的 fenced code block，可复制/执行（默认需确认） |
| title | 块标题：标签向上最近的 `##`；无则搜索词首词，再兜底 kind |
| manifest / meta.json | 库级派生物（`<库根>/.zoro/meta.json`），schema v2；可删可重建 |
| zoro-index.tsv | manifest 可读视图，四列：title / index / start / path |
| 身份三元组 | 块唯一身份 `(库名, path, start)`；库名必须进展示路径 / 候选行 / URL |
| Candidate | 查询候选：`library, title, index, path, start, score, matches` |
| matches | 命中区间（UTF-8 byte offsets），供前端高亮 |
| target | 展示产物形态：HTML / ANSI / 纯文本 |
| kind（展示维） | 与 target 正交的展示类型，决定前端视图（叙述 / 代码 / 媒体 / 图） |
| fzf | CLI 前端交互控件（捆绑、不进 core）；Launcher 不用 fzf |
| `ZORO_WORKSPACE` | 指定工作区 `zoro.toml` 的环境变量，优先级最高 |
| `~/.zoro` | 全局默认目录：`zoro.toml`、`kb/`、`index/<库名>/` |
| P0–P7 | 路线图阶段：P0 标签体系 / P1 渲染 / P2 Web / P3 CLI / P3.5 Launcher / P4 全文 / P5 站点 / P6 同步 / P7 加密 |
| ad-hoc 签名 | macOS 本地免费签名方式（自用开发）；正式分发另走 Developer ID + 公证 |
| Wails bridge | Wails 将 Go 方法暴露给前端 JS 的桥（`window.go.main.App`） |
