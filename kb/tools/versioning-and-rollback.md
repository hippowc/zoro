# 版本号与回滚（tools/versioning-and-rollback.md）

> 原子配方：怎么给版本打 tag、出问题怎么退回稳定版本。发布流程见 `skills/ship-launcher-release.md`。
> 决策依据见 `facts/decisions.md` AD-19（为什么用 tag 而不是分支）。

## 1. 版本号约定

语义化版本号 `vMAJOR.MINOR.PATCH`，打在 main 上：

| 位 | 什么时候进 | 例子 |
|---|---|---|
| PATCH | 修 bug、改文案、改样式 | `v1.0.1` |
| MINOR | 加命令（`/xxx`）、加视图、加 `@标签` 类型，且旧内容/旧配置照常能用 | `v1.1.0` |
| MAJOR | 内容契约或 `zoro.toml` 格式不兼容（旧库读不出来 / 写回去会变样） | `v2.0.0` |
| 预发布 | 需要用户在 Mac 上先试、还不算正式版本 | `v1.1.0-rc.1` |

```bash
git tag -a v1.0.1 -m "修复 xxx" && git push origin v1.0.1
```

- **必须 `-a`（附注 tag）**。轻量 tag 只是一个指针，附注 tag 带打 tag 的人、时间、说明——`git tag -l -n1` 才列得出版本清单，将来回溯「这个版本当时为什么发」全靠它。
- 推 tag 会触发 GitHub Actions 构建并发布 Release（`v*` 匹配）。只想打标记不想构建，就本地打 tag 先不推。
- **已推出去的 tag 永不改名、永不移动、永不删除**。同一个 tag 名重新指向别的提交，会让已发布的 Release 与本地历史对不上，用户手里的 zip 也就无从对应。某个版本有严重 bug → 发下一个版本修，不要复用旧 tag 名。

查已有版本：

```bash
git tag -l -n1 --sort=-creatordate | head -10
```

## 2. 回滚：按需求选最轻的那一档

**先想清楚要回滚的是什么**——四档的破坏性差好几个数量级，够用就停，不要往下走。

### 档 1：只是想要旧版本的应用（最常见，零风险）

不用动仓库。旧版本的产物一直在 Release 上：

```
https://github.com/hippowc/zoro/releases/tag/v1.0.0
```

下载 `zoro-launcher-macos-arm64.zip` 解压即用。

### 档 2：想在旧代码上跑一下 / 对比行为（零风险，不碰 main）

```bash
git stash -u                      # 若有未提交改动，先存起来
git checkout v1.0.0               # detached HEAD，只读状态
# ……看代码、跑 go test ./... 、bash scripts/verify.sh
git checkout main && git stash pop
```

detached HEAD 下的提交不属于任何分支，回到 main 就看不见——**这正是它安全的原因**。要在此基础上改，先 `git switch -c fix-from-v1.0.0`。

### 档 3：main 上某个提交是坏的，要撤销它（推荐，可推送）

```bash
git revert <坏提交的 sha>          # 生成一个「反向提交」，历史保留
git push origin main
```

坏提交与它的撤销都在历史里，谁改坏的、怎么改回来的都可查。多个提交就 `git revert A^..B`。**这是唯一不需要改写历史、因此可以安全推送的方式。**

### 档 4：把 main 硬退回某个版本（破坏性，需明示授权）

```bash
git status                        # 先确认没有未提交的工作
git reset --hard v1.0.0           # 之后的提交从 main 上消失
git push --force-with-lease origin main
```

- `--force-with-lease` 而不是 `--force`：若远端有别人的提交，推送会被拒绝而不是覆盖掉。
- 被丢弃的提交在本地 reflog 里还能捞（`git reflog` → `git reset --hard <sha>`），但**远端 Release 不会跟着回退**，用户仍能下载到被丢弃版本的产物。
- 单人开发场景下，档 3 几乎总能替代档 4。选档 4 之前先问一句：我要的是「历史里没有这段」，还是「代码回到那个状态」？后者用档 3 就够，且代价小得多。

### 回滚后必须验证

```bash
export PATH=/usr/local/go/bin:/root/go/bin:$PATH
bash scripts/verify.sh            # 离线环境加：GOMODCACHE=/root/go/pkg/mod GOFLAGS=-mod=mod GOPROXY=off
```

## 3. 历史 tag 说明

`v1.0.0` 之前的 17 个 tag 都是 `launcher-<摘要>-<日期>` 形式的**轻量 tag**（如 `launcher-tailwind-alpine-20260909`）。它们是当时 CI 触发约定的产物，保留不动、不要转成附注 tag（改写已推送的 tag 见 §1 的禁令）。需要「当时的稳定版本」时，用 `git log` 找那个 tag 指向的提交即可。
