# 发布 macOS Launcher Release（skills/ship-launcher-release.md）

> 端到端 SOP：改 Launcher 后，经 GitHub Actions 产出用户可下载的 arm64 `.app` zip。
> 依赖 `tools/launcher-build-release.md`、`tools/build-test.md`、`facts/pitfalls.md`。

## 前置检查

1. 确认改动已通过本地验证：
   ```bash
   cd ext/zoro-launcher && go build -o /tmp/zoro-launcher-linux . && go vet ./...
   node --check ext/zoro-launcher/frontend/dist/app.js
   ```
2. 若改 UI 透明/原生窗口，三层一起核对（见 `facts/pitfalls.md` P-6）：
   - `main.go` 原生窗口配置
   - `frontend/dist/index.html` / `styles.css`
   - `frontend/dist/app.js`（桥接返回值判空）

## 提交与打 tag

```bash
cd /root/zoro
git add ext/zoro-launcher
git commit -m "fix/feat(launcher): <一句话>"
git push origin main
git tag launcher-macos-vX.Y
git push origin launcher-macos-vX.Y
```

> tag 名匹配 `launcher-*` 才会触发 GitHub Actions 构建发布。

## 等待 CI

```bash
curl -sL "https://api.github.com/repos/hippowc/zoro/actions/runs?per_page=5" \
  | jq -r '.workflow_runs[] | [.id, .head_branch, .status, .conclusion] | @tsv'
```

轮询到 `completed success`。

## 取下载地址与 SHA256

```bash
TAG=launcher-macos-vX.Y
curl -sL "https://api.github.com/repos/hippowc/zoro/releases/tags/$TAG" \
  | jq -r '.assets[] | [.name, .browser_download_url] | @tsv'
curl -sL "https://github.com/hippowc/zoro/releases/download/$TAG/zoro-launcher-macos-arm64.zip.sha256"
```

把 zip 地址与 sha256 发给用户；用户解压 `.app` 即用，无需安装 Wails CLI。

## 验证点（交付前）

- [ ] Actions 成功，Release 含 `zoro-launcher-macos-arm64.zip` 与 `.sha256`
- [ ] 查询无命中不报 `candidates.length` TypeError（P-2 已修）
- [ ] 窗口只有搜索框/结果/preview 有背景，其余透明，无深色色块（P-1 已修）
- [ ] 首次运行如被 Gatekeeper 拦，提示用户 `xattr -cr`（P-3）
