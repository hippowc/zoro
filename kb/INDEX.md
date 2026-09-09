# zoro 知识库索引（kb/INDEX.md）

> 这是路由表，不是目录。按「要做的事」查该读什么；宁少而准，只覆盖真实高频任务。
> 单级索引：当前规模不超过 200 行，不预拆多级。

## 任务 → 读什么

| 任务场景 | 先读 | 再按需读 |
|---|---|---|
| 刚接手项目，建立全貌 | `../agents.md`（工作流与约定）→ `facts/architecture.md` | `facts/glossary.md`；命令速览在 `tools/cli-usage.md`（⚠️ `../README.md` 已**刻意清空**，不要填回去） |
| 理解「为什么这么设计、否决过什么」 | `facts/decisions.md` | `facts/architecture.md` |
| 新增知识库并搜索 / 预览 / 打开 | `tools/cli-usage.md` → `skills/add-library-search-preview.md` | `facts/glossary.md` |
| 给 Markdown 写 `@` 标签并重建索引 | `tools/cli-usage.md` → `facts/architecture.md`（内容模型） | `skills/add-library-search-preview.md` |
| 改 core（模型 / analyze / 查询 / 渲染 / 扩展接口） | `facts/architecture.md` → `facts/decisions.md` | `tools/build-test.md` |
| 改 CLI 或 Web / 跑测试验证 | `tools/build-test.md` | `tools/verify.md`；`facts/pitfalls.md` |
| 构建 / 发布 macOS Launcher | `tools/launcher-build-release.md` → `skills/ship-launcher-release.md` | `facts/pitfalls.md` |
| 打版本号 / **回滚到稳定版本** / 「要不要开分支」 | `tools/versioning-and-rollback.md`（四档回滚，够用就停） | `facts/decisions.md` **AD-19**（为什么用 tag 不用分支） |
| 改 Launcher 前端（Tailwind 样式 / Alpine 状态 / 窗口尺寸） | `../ext/zoro-launcher/frontend/README.md` → `tools/launcher-build-release.md`（前端构建节） | `facts/pitfalls.md` P-6…P-15；`facts/decisions.md` AD-12 |
| 给 Launcher **加命令 / 加视图**（`/lib`、`/reindex`、`/theme` 这一类） | `../ext/zoro-launcher/frontend/README.md`（命令面节：两步配方）→ `tools/launcher-build-release.md`（命令面节：已实现清单与四条不变量） | `facts/decisions.md` AD-17；`facts/pitfalls.md` P-15 |
| 给 Launcher 加**尚未落地**的能力（片段捕获 / 删除语义 / 视图注册表） | `journal/2026-09-09-launcher-command-surface-and-view-extension-plan.md`（已核验源码事实 F-1…F-18、禁令清单）⚠️ 其 §4.4 / §6.3 / §7 有 **7 处已被实现推翻**，对照表见 `journal/2026-09-09-visual-blocks-storage-vs-editing-surface.md` §9 | `../todos.md` 看板 9.9–9.11；`facts/decisions.md` AD-15…AD-18 |
| 做**脑图 / 画布 / 可视化块**（「存储一定要文本吗」「codec 放哪」） | `journal/2026-09-09-visual-blocks-storage-vs-editing-surface.md`（三层分离 + 业界调研 + 否决项 + 两阶段实施） | `facts/decisions.md` **AD-18**（D5 已定）、AD-4（Markdown 唯一源）；`../todos.md` 9.11 |
| **写** Markdown 内容（加块 / 改块 / 删块 / 加库） | `facts/architecture.md`（写入层：`core/write.go`、三条写定律 L1–L3）→ `facts/decisions.md` AD-15 / AD-16 | `tools/cli-usage.md`；`facts/pitfalls.md` P-13 |
| 配置 Launcher UI 主题与快捷键 | `tools/launcher-build-release.md`（UI 主题配置节） | `journal/2026-09-08-launcher-ui-enhancements.md` |
| 排查询询失败 / 透明窗口 / Gatekeeper / CLI 卡住不出结果 等已知坑 | `facts/pitfalls.md` | `facts/decisions.md`（相关决策） |
| 运行验收清单 | `tools/verify.md` | `scripts/verify.sh` |
| 维护公网下载网关 | `tools/download-gateway.md` | — |
| 看下一步做什么 | `../todos.md` | `facts/decisions.md`（待定 / 已否决项） |

## kb 结构

```
kb/
├── INDEX.md             # 本文件：索引入口
├── facts/               # 是什么：architecture / decisions / pitfalls / glossary
├── tools/               # 怎么做（原子配方）：构建 / CLI / 发布 / 版本回滚 / 验收 / 网关
├── skills/              # 怎么做（端到端 SOP）
└── journal/             # 时序事实（有观察时才创建文件，不预建空目录）
```

## 读取顺序（通用）

1. 先读 `agents.md`：知道工作流和禁区。
2. 再读本 INDEX，按任务只加载相关 `facts/`、`tools/` 或 `skills/`。
3. 最后读 `journal/` 中未固化的增量观察（如果存在）。
4. `todos.md` 保持「下一步做什么」的唯一事实源，设计细节不重复堆放。
