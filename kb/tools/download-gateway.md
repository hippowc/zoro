# 公网下载网关（tools/download-gateway.md）

> 原子配方：静态文件下载服务的位置与维护。仅记录环境变量名/公共信息，不放密钥。

## 现状

- 公网 IP：`8.146.233.2`（80 端口）
- 目录列表：http://8.146.233.2/
- 下载目录：`/tmp/zoro-downloads`，放入新文件后通过 `http://8.146.233.2/<文件名>` 下载
- 网关：nginx（systemd 管理），站点配置 `/etc/nginx/sites-available/zoro-downloads`

## 常用资产

| 文件 | URL |
|------|-----|
| macOS（Apple Silicon / arm64） | http://8.146.233.2/zoro-darwin-arm64 |
| Linux（amd64） | http://8.146.233.2/zoro-linux-amd64 |
| 校验和 | http://8.146.233.2/SHA256SUMS.txt |

## 更新文件

1. 把新产物放入 `/tmp/zoro-downloads/`。
2. 更新 `SHA256SUMS.txt`（`shasum -a 256 <file> >> SHA256SUMS.txt`，去重确认）。
3. 确认 nginx 仍托管该目录即可，无需额外发布步骤。

## 下载侧使用

macOS 浏览器下载会带 quarantine，先解除：

```bash
xattr -d com.apple.quarantine zoro-darwin-arm64
chmod +x zoro-darwin-arm64
./zoro-darwin-arm64
```

curl 下载一般无需处理 quarantine：

```bash
curl -L -o zoro http://8.146.233.2/zoro-darwin-arm64
chmod +x zoro
```
