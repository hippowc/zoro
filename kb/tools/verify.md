# 验收清单（tools/verify.md）

> 原子配方：阶段验收核对项；另见 `scripts/verify.sh`。
> 已勾选 = 历史已验收（P0/P1 基线），未勾选 = 后续阶段待验收。

## P0–P1 已验收基线（保持不回退）

- [x] 工作区单一模型：`zoro.toml` 单库/多库均可用；`ZORO_ROOT`/`ZORO_LIBS` 不再参与。
- [x] `root` 相对 `zoro.toml` 所在目录正确解析。
- [x] 库级配置：`preview` 解析；自定义未知键保留、不报错。
- [x] `analyze` 幂等：删 `.zoro/` 重建，结果逐字段一致。
- [x] manifest schema v2；块含 `title/kind/index/start/path`；`@shell` 含 `shell.lang/lines`；`zoro-index.tsv` 四列正确。
- [x] 多库：同名块不串库；候选行/展示路径带库名前缀；库名冲突报错。
- [x] 统一边界：任意 `@` 标签开新块、结束上一块；fence 内 `@` 不误判；`title` 规则正确。
- [x] `@shell` 正确绑定 fence（搜索词 + 语言 + 行号）；无 fence 按正文块降级。
- [x] 未注册标签降级 `unknown:<name>`，不报错、不丢内容。
- [x] 未变更直接读元数据；`.md` 更新后自动重建；schema 不匹配自动重建。
- [x] `core` 无 fzf / 无 UI / 无网络依赖。
- [x] 三面搜索：P1 完成 index terms 主面 + `matches` 高亮区间；title / raw 作为扩展点，不回归。
- [x] 搜索范围默认仅已声明库；无“全盘文件搜索”进入 core 主路径。
- [x] 渲染三 target（HTML / ANSI / 纯文本）可用；非 TTY 降级为纯文本。

## 后续阶段待验收

- [ ] 【P3】CLI fzf 候选显示 title；`--preview` 随选随显；回车全屏；无 TTY 降级。
- [ ] 【P3】单二进制（捆绑 fzf）新机器零安装跑通。
- [ ] 【P2】`zoro serve` 浏览器完成浏览/搜索/渲染。
- [ ] 【P3.5】Launcher 单平台 MVP：常驻 + 全局热键 + 无边框浮窗 + 匹配 + 回车复制/打开渲染；无 fzf 依赖。
- [ ] 【P3.5 后】回填/执行平台可选：缺权限或不支持时降级为复制；「复制 + 打开」三平台一致可用。

## 跑法

```bash
go test ./...
bash scripts/verify.sh   # 需要时
```
