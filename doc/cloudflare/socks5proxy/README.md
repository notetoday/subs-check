# socks5proxy - 节点池出口代理

当服务器到目标站点（如 Cloudflare Worker）被 SNI 级 DPI 阻断时，通过订阅节点池作为出口转发流量。

## 原理

服务器无法直连被封锁目标（TCP 443 带目标 SNI 即被 RST，UDP 443 QUIC 握手超时）。本工具内置 mihomo 内核，从订阅的 `mihomo.yaml` 读取全部节点作为节点池，对外提供 SOCKS5 + HTTP CONNECT 双协议代理，供 subs-check 的订阅抓取和 R2 上传走代理出站。

## 功能

- 双协议：SOCKS5 和 HTTP CONNECT（按前 2 字节 `0x05` / `CO` 自动识别）
- 节点池模式：读取 mihomo.yaml 全部节点，轮转 + 单节点 8s 超时 + 失败自动切换
- 单节点模式：通过 `PROXY_*` 环境变量直接指定一个节点
- 双向转发用 `CloseWrite` 半关闭，避免 HTTP 响应挂起

## 编译

### 本地编译

```bash
cd socks5proxy
GOPROXY=https://goproxy.cn,direct CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o socks5proxy .
```

产物为单静态二进制，约 40MB，可直接 scp 到服务器。

### GitHub Actions 自动构建

仓库已配置 `.github/workflows/build-socks5proxy.yml`：每次 push 修改 `doc/cloudflare/socks5proxy/` 即自动编译 linux/amd64 版，产物同时上传：

- **Actions Artifact**：`socks5proxy-linux-amd64`（保留 30 天）
- **Release**：tag `socks5proxy-latest`，服务器可直接下载：

```bash
curl -fsSL -o /opt/subs-check/socks5proxy.new \
  https://github.com/notetoday/subs-check/releases/download/socks5proxy-latest/socks5proxy_linux_amd64
chmod +x /opt/subs-check/socks5proxy.new
sha256sum -c socks5proxy_linux_amd64.sha256 && mv socks5proxy.new socks5proxy
systemctl restart socks5proxy
```

若服务器访问 GitHub 慢，可加 `https://ghfast.top/` 前缀加速。

## 部署

### 1. 上传二进制

```bash
scp -P 35312 socks5proxy root@61.164.243.125:/opt/subs-check/socks5proxy
```

### 2. systemd 服务

`/etc/systemd/system/socks5proxy.service`:

```ini
[Unit]
Description=SOCKS5 Proxy via node pool
After=network.target

[Service]
ExecStart=/opt/subs-check/socks5proxy
Environment=PROXY_LISTEN=127.0.0.1:7890
Environment=PROXY_POOL_FILE=/opt/subs-check/output/mihomo.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now socks5proxy
```

### 3. subs-check 接入

在 `/opt/subs-check/config/config.yaml` 配置：

```yaml
save-method: r2
worker-url: https://young-glitter-9dee.checkto.workers.dev
proxy: "http://127.0.0.1:7890"
```

`proxy` 仅用于订阅抓取和 R2 上传（`app.go` Initialize 时写入 `HTTP_PROXY`/`HTTPS_PROXY` 环境变量）。检测/测速使用节点直连（`check.go` 的 `baseTransport` 自定义 `DialContext`），不受代理影响。

## 验证

```bash
# 节点池加载
curl --socks5-hostname 127.0.0.1:7890 https://young-glitter-9dee.checkto.workers.dev/  -o /dev/null -w "%{http_code}\n"
# HTTP CONNECT
curl -x http://127.0.0.1:7890 https://young-glitter-9dee.checkto.workers.dev/ -o /dev/null -w "%{http_code}\n"
```

## 配置环境变量

| 变量 | 说明 |
|------|------|
| `PROXY_LISTEN` | 监听地址，默认 `127.0.0.1:7890` |
| `PROXY_POOL_FILE` | mihomo.yaml 路径，启用节点池模式 |
| `PROXY_TYPE` | 单节点模式：节点类型（vless/vmess 等） |
| `PROXY_SERVER` | 单节点模式：服务器地址 |
| `PROXY_PORT` | 单节点模式：端口 |
| `PROXY_UUID` | 单节点模式：uuid |
| `PROXY_NETWORK` | 单节点模式：传输网络 |
| `PROXY_FLOW` | 单节点模式：flow |
| `PROXY_SERVERNAME` | 单节点模式：servername |
| `PROXY_FINGERPRINT` | 单节点模式：client-fingerprint |
| `PROXY_PUBKEY` | 单节点模式：reality public-key（需配合 `PROXY_SHORTID`） |
| `PROXY_SHORTID` | 单节点模式：reality short-id |
| `PROXY_PASSWORD` | 单节点模式：密码 |
| `PROXY_USERNAME` | 单节点模式：用户名 |
| `PROXY_SNI` | 单节点模式：SNI |
| `PROXY_TLS` | 单节点模式：`true`/`false` |
| `PROXY_UDP` | 单节点模式：`true`/`false` |

## 注意事项

- 节点池数据源 `mihomo.yaml` 由 subs-check 每轮检测自动更新，socks5proxy 每次连接时重新读取该文件，无需重启即可感知节点变化
- 单个坏节点会被 8s 超时兜底，不影响整体（节点池轮转跳过）
- 代理只监本机 `127.0.0.1`，不对外暴露
