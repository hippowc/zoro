# 发布 macOS Launcher Release（skills/ship-launcher-release.md）

> 端到端 SOP：改 Launcher 后，经 GitHub Actions 产出用户可下载的 arm64 `.app` zip。
> 依赖 `tools/launcher-build-release.md`、`tools/build-test.md`、`facts/pitfalls.md`；改前端 UI 时先读 `../../ext/zoro-launcher/frontend/README.md`。

## 前置检查

1. 确认改动已通过本地验证：
   ```bash
   cd ext/zoro-launcher/frontend
   npm install && npm run build          # 生成 dist/styles.css（永不手改该文件）
   grep -c 'glass-panel\|wails-drag\|md-body' dist/styles.css   # 必须 > 0，否则 content 路径错、样式被 purge
   node --check dist/app.js
   cd .. && go build -o /tmp/zoro-launcher-linux . && go vet ./...
   ```
   > `styles.css` 接近空却照样构建成功，是本次架构最危险的静默失败（见 `facts/pitfalls.md` P-8/P-9）；CI 里也有 `Verify frontend assets were compiled` 兜底。
2. 若改 UI 透明/原生窗口/样式，四层一起核对（见 `facts/pitfalls.md` P-6）：
   - `main.go` 原生窗口配置（`MinHeight` 必须 ≤ `app.js` 的 `MIN_WINDOW_HEIGHT`）
   - `frontend/src/input.css`（样式唯一源；运行时 HTML 样式放 `@layer` 之外）
   - `frontend/dist/index.html`
   - `frontend/dist/app.js`（桥接返回值判空；窗口尺寸只在 `fitWindowToContent()` 写）

   改 UI 前先读 `frontend/README.md` 的五条防回归规则。

## 提交与打 tag

```bash
cd /root/zoro
git add ext/zoro-launcher            # 含 frontend/src、package*.json；node_modules 已被根 .gitignore 排除
git commit -m "fix/feat(launcher): <一句话>"
git push origin main
TAG="v1.2.3"                         # 语义化版本号；预发布用 v1.2.3-rc.1
git tag -a "$TAG" -m "<一句话说明这个版本>" && git push origin "$TAG"
```

> **版本号怎么选**：修 bug / 改文案 → patch（`v1.0.1`）；加命令、加视图、加标签类型 → minor（`v1.1.0`）；内容契约或 `zoro.toml` 格式不兼容 → major（`v2.0.0`）。
> tag 名匹配 `v*` 才会触发 GitHub Actions 构建发布（`launcher-*` 是兼容旧配方的别名，不要再新用）。**必须用 `git tag -a`（附注 tag）**：它是「稳定版本」的唯一锚点，回滚全靠它，见 `tools/versioning-and-rollback.md`。
> tag 要打在含当前 `.github/workflows/build-macos-launcher.yml` 的提交上，否则 Actions 用旧触发规则，会静默不构建。
> `frontend/dist/` 是生成物但**必须提交**：`//go:embed all:frontend/dist` 在 CI 上直接嵌入，且 `wails build -clean` 从不清 dist。

## 等待 CI

```bash
curl -sL "https://api.github.com/repos/hippowc/zoro/actions/runs?per_page=5" \
  | jq -r '.workflow_runs[] | [.id, .head_branch, .status, .conclusion] | @tsv'
```

轮询到 `completed success`。

## 取下载地址与 SHA256

```bash
TAG=v1.2.3
curl -sL "https://api.github.com/repos/hippowc/zoro/releases/tags/$TAG" \
  | jq -r '.assets[] | [.name, .browser_download_url] | @tsv'
curl -sL "https://github.com/hippowc/zoro/releases/download/$TAG/zoro-launcher-macos-arm64.zip.sha256"
```

> 本机 `gh` 未认证，查 Actions / Release 一律用 `curl` 打公开 API。

把 zip 地址与 sha256 发给用户；用户解压 `.app` 即用，无需安装 Wails CLI / Node。

## 验证点（交付前）

- [ ] Actions 成功，`Verify frontend assets were compiled` 步骤绿（styles.css 非空且含 glass-panel / wails-draggable）
- [ ] Release 含 `zoro-launcher-macos-arm64.zip` 与 `.sha256`
- [ ] 启动即「仅搜索框」高度（约 76px），下方**无**大块透明空白
- [ ] 有结果时窗口向下生长（顶边不动）；清空查询后收回 76px，无透明框残留（P-11）
- [ ] 详情模式整窗可拖拽（含 markdown 内容区，靠 `.wails-drag` 继承）；返回按钮可见可点
- [ ] 中文输入法组词期间按 Enter 只确认候选词，不进详情模式
- [ ] 查询无命中不报 `candidates.length` TypeError（P-2 已修）
- [ ] 窗口只有搜索框/结果/preview 有背景，其余透明，无深色色块（P-1 已修）
- [ ] 首次运行如被 Gatekeeper 拦，提示用户 `xattr -cr`（P-3）
