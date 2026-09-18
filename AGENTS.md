# AGENTS.md

## 项目说明

本仓库是 `beck-8/subs-check` 的 fork（`notetoday/subs-check`），在上游基础上做了节点重命名定制。
上游仓库：https://github.com/beck-8/subs-check

## 本 fork 的定制改动（合并上游时必须保留）

| 文件 | 定制内容 |
|---|---|
| `proxy/rename.go` | 节点名 = 中文名 + 两位序号（如 `美国01`）；`regionCN` 中文对照表；地区查询失败兜底为 `备用`（替代上游 `❓Other`）；无 emoji 国旗 |
| `save/save.go` | `RenderNameParts(results[i], false)`——最终节点名不含速度标签（速度在 Web 面板结果页单独展示） |
| `check/render.go` | `includeSpeed` 参数保留但不再拼 SpeedTag |
| `check/render_test.go`、`proxy/rename_test.go` | 与上述行为配套的测试 |

**合并冲突原则**：如果上游也改了重命名/名字拼接相关代码，以本 fork 的逻辑为准，再吸收上游其他功能。合并后必须跑通构建与测试。

## 构建与测试

```bash
# 依赖 Go 1.25+；国内环境建议
export GOPROXY=https://goproxy.cn,direct GOSUMDB=off CGO_ENABLED=0

go build -trimpath -ldflags "-s -w" -o subs-check .
go test ./...
```

## 上游同步

### 自动（已配置）
`.github/workflows/sync-fork.yml`：每日定时 + 手动触发（workflow_dispatch）。
行为：无新提交则跳过；合并冲突或 `go build`/`go test` 失败则**不推送**，需人工介入。

### 手动
```bash
git remote add upstream https://github.com/beck-8/subs-check.git
git fetch upstream
git merge upstream/master
go build ./... && go test ./...
git push origin master
```

GitHub 页面上的 **Sync fork → Update branch** 按钮也可用，但撞冲突时仍需本地处理。
**切勿点击 "Discard commits"**，会抹掉本 fork 的定制补丁。

## 服务器部署

目标服务器：SSH 地址、端口与凭据均不写入仓库，以运维方式私下提供。

```bash
cd /opt/subs-check-src && git pull
bash /opt/subs-build.sh
```

构建产物：`/opt/subs-check-src/subs-check`（约 117MB）。

部署与回滚：
```bash
# 部署（先备份）
cp -a /opt/subs-check/subs-check /opt/subs-check/subs-check.bak.$(date +%Y%m%d%H%M)
cp -f /opt/subs-check-src/subs-check /opt/subs-check/subs-check
systemctl restart subs-check

# 回滚
cp -f /opt/subs-check/subs-check.bak.<时间戳> /opt/subs-check/subs-check
systemctl restart subs-check
```

验证清单：
1. `systemctl is-active subs-check` 为 active
2. 端口监听：8199（后端/Web UI，api-key 见服务器配置）、8299（sub-store/node）
3. 日志版本号与启动流水线正常（`journalctl -u subs-check -n 20`）
4. 输出节点名格式：`1x | DIRE | 美国01` / `1x | DIRE | 备用03`，无速度后缀、无 emoji
5. 测速高峰期 SSH/HTTP 可能短暂卡顿，属服务器负载现象，等待即可

配置文件：`/opt/subs-check/config/config.yaml`（`rename-node: true`、`node-prefix: "1x | DIRE | "`）。
输出目录：`/opt/subs-check/output/`（all.yaml、mihomo.yaml、base64.txt，含 R2 上传）。

## 分支与提交规范

- 分支命名：`YYMMDD-(feat|fix|chore|refactor)-简述`，如 `260918-feat-node-rename-cn`
- 提交信息：conventional commits（feat/fix/chore/style...），正文列要点
- 推送到远程前须征得使用者同意
- 服务器 clone 的 fetch URL 走 ghfast 镜像（只读代理），pull 失败时可换 GitHub 直连
