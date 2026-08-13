# AGENTS.md

本文件记录本仓库的 CI 构建、服务器部署与关键操作约定，供后续开发与运维参考。

## 仓库结构

- 根目录：主程序 `subs-check`（Go，多平台构建见 `Makefile`）
- `doc/cloudflare/socks5proxy/`：独立 Go 模块，节点池出口代理工具（内置 mihomo 内核），有自己独立的 `go.mod`
- `.github/workflows/`：CI 配置

## CI 构建

### 主程序 subs-check

- `release.yml`：打 `v*.*.*` tag 时触发，goreleaser 构建多平台二进制 + Docker 镜像（ghcr.io + Docker Hub）

### socks5proxy

- `build-socks5proxy.yml`：每次 push 修改 `doc/cloudflare/socks5proxy/**` 时触发（可 `workflow_dispatch` 手动触发）
- 构建 linux/amd64 版，产物同时上传：
  - **Actions Artifact**：`socks5proxy-linux-amd64`（保留 30 天）
  - **Release**：tag `socks5proxy-latest`（prerelease，自动覆盖旧版）
- Release 下载地址：`https://github.com/notetoday/subs-check/releases/download/socks5proxy-latest/socks5proxy_linux_amd64`

## 服务器部署

> 注意：服务器 IP、端口、登录凭据、worker-url 等敏感信息**不要写入本文件**（公开仓库）。以下流程使用占位符 `<SERVER>` 表示服务器，实际部署时替换为真实值。

### socks5proxy 更新流程

```bash
curl -fsSL -o <DEPLOY_DIR>/socks5proxy.new \
  https://github.com/notetoday/subs-check/releases/download/socks5proxy-latest/socks5proxy_linux_amd64
chmod +x <DEPLOY_DIR>/socks5proxy.new
sha256sum -c socks5proxy_linux_amd64.sha256 && mv socks5proxy.new socks5proxy
systemctl restart socks5proxy
```

服务器访问 GitHub 慢时加 `https://ghfast.top/` 前缀加速。

### 服务器运行服务

- `socks5proxy.service`：监听 `127.0.0.1:7890`，节点池数据源 `<DEPLOY_DIR>/output/mihomo.yaml`（subs-check 每轮检测自动更新）
- `subs-check.service`：主服务，config 中 `proxy: "http://127.0.0.1:7890"`、`save-method: r2`、`worker-url: <WORKER_URL>`

### 节点代理方案背景

服务器到目标站点（Cloudflare Worker）被 SNI 级 DPI 阻断（TCP 443 带目标 SNI 即 RST，UDP 443 QUIC 超时，DNS 污染），因此通过 socks5proxy 节点池作为出站出口。检测/测速走节点直连（`check.go` 的 `baseTransport` 自定义 `DialContext`），不受代理影响；仅订阅抓取和 R2 上传走代理（`app.go` Initialize 时写入 `HTTP_PROXY`/`HTTPS_PROXY`）。

## 常用约定

- 推送 git 时若报 workflow 权限错误，说明 token 缺 `workflow` scope，需要用户提供带该 scope 的 token
- `proxy/rename.go`、`proxy/info.go`：节点命名规则（中文国家名、无 emoji、无速率标签、兜底"备用"、`01/02` 两位序号）
