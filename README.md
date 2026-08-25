# zoro

纯本地、零 agent、零网络的**通用知识速查表工具**。

- 查询入口：`zoro` 命令 + fzf 交互
- 工具与内容分离：本仓库只含工具，真实速查内容在**外部内容库**，通过环境变量 `ZORO_ROOT` 接入
- 运行时零 agent、零网络；依赖 fzf（必装）、rg / bat（可降级为 grep / cat）
- 设计与开发规范见 [agents.md](agents.md)
