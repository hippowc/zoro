# zoro 知识库索引（kb/INDEX.md）

> 这是路由表，不是目录。按「要做的事」查该读什么；宁少而准，只覆盖真实高频任务。
> 单级索引：当前规模不超过 200 行，不预拆多级。

## 任务 → 读什么

| 任务场景 | 先读 | 再按需读 |
|---|---|---|
| 刚接手项目，建立全貌 | `../agents.md`（工作流与约定）→ `facts/architecture.md` | `facts/glossary.md`；`../README.md` 用于命令速览 |
| 理解「为什么这么设计、否决过什么」 | `facts/decisions.md` | `facts/architecture.md` |
| 新增知识库并搜索 / 预览 / 打开 | `tools/cli-usage.md` → `skills/add-library-search-preview.md` | `facts/glossary.md` |
| 给 Markdown 写 `@` 标签并重建索引 | `tools/cli-usage.md` → `facts/architecture.md`（内容模型） | `skills/add-library-search-preview.md` |
| 改 core（模型 / analyze / 查询 / 渲染 / 扩展接口） | `facts/architecture.md` → `facts/decisions.md` | `tools/build-test.md` |
| 改 CLI 或 Web / 跑测试验证 | `tools/build-test.md` | `tools/verify.md`；`facts/pitfalls.md` |
| 构建 / 发布 macOS Launcher | `tools/launcher-build-release.md` → `skills/ship-launcher-release.md` | `facts/pitfalls.md` |
| 排查询询失败 / 透明窗口 / Gatekeeper 等已知坑 | `facts/pitfalls.md` | `facts/decisions.md`（相关决策） |
| 运行验收清单 | `tools/verify.md` | `scripts/verify.sh` |
| 维护公网下载网关 | `tools/download-gateway.md` | — |
| 看下一步做什么 | `../todos.md` | `facts/decisions.md`（待定 / 已否决项） |

## kb 结构

```
kb/
├── INDEX.md             # 本文件：索引入口
├── facts/               # 是什么：architecture / decisions / pitfalls / glossary
├── tools/               # 怎么做（原子配方）：构建 / CLI / 发布 / 验收 / 网关
├── skills/              # 怎么做（端到端 SOP）
└── journal/             # 时序事实（有观察时才创建文件，不预建空目录）
```

## 读取顺序（通用）

1. 先读 `agents.md`：知道工作流和禁区。
2. 再读本 INDEX，按任务只加载相关 `facts/`、`tools/` 或 `skills/`。
3. 最后读 `journal/` 中未固化的增量观察（如果存在）。
4. `todos.md` 保持「下一步做什么」的唯一事实源，设计细节不重复堆放。
