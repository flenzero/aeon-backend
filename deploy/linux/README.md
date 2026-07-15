# Aeonblight Linux 部署包

这个部署包用于在 Linux 服务器上部署当前后端仓库，包含：

- 已编译的 `account-api` Docker 镜像：宿主机回环端口 `8081`
- 已编译的 `economy-api` Docker 镜像：宿主机回环端口 `8082`
- 已编译的 `admin-api` Docker 镜像：宿主机回环端口 `8083`
- 已编译的 `economy-worker` Docker 镜像
- 已打包的 `db-migrate` 数据库迁移镜像
- `postgres`
- `redis`
- 宿主机 `nginx` 反向代理和 HTTPS 证书配置脚本

发布包不会携带 Go 源码、`internal/`、`cmd/` 或明文 JSON 配置目录。玩法 JSON 配置会被打进应用镜像里；服务器上只需要镜像包、compose、nginx 模板、运维脚本和 `.env`。`.env` 仍然必须留在服务器上，因为密钥、数据库密码、链上钱包私钥等不应该编译进镜像。

当前仓库不包含 WebGL 客户端，也不包含独立游戏服二进制。因此这个后端部署包默认只配置 `api.aeonblight.com` 和 `game.aeonblight.com`。`play.aeonblight.com` 应该指向你的客户端托管平台，不要指向这台后端服务器。

## 域名转发

- `api.aeonblight.com/api/auth/*`、`/api/public/*`、`/api/character/*`、`/api/game/*` 转发到 `account-api`
- `api.aeonblight.com/api/economy/*`、`/api/announcements/*`、`/api/chain/*` 转发到 `economy-api`
- `api.aeonblight.com/api/admin/*` 转发到 `admin-api`
- `game.aeonblight.com/*` 转发到 `.env` 里的 `GAME_UPSTREAM`，并带 WebSocket 头

如果游戏服使用的是原生 TCP 或 UDP，而不是 HTTP/WebSocket，不要把 `game.aeonblight.com` 当作最终游戏流量代理。那种情况应该单独开放游戏端口，或者额外配置 nginx `stream` / 负载均衡。

## 本地打包

在仓库根目录执行。默认构建 `linux/amd64` 镜像：

```bash
./deploy/package-docker.sh
```

如果服务器是 ARM64：

```bash
PLATFORM=linux/arm64 ./deploy/package-docker.sh
```

脚本会生成两个目录：

```text
outputs/packages/aeonblight-server
outputs/packages/aeonblight-server-时间戳
```

`aeonblight-server` 不带时间戳，方便每次覆盖传输；`aeonblight-server-时间戳` 用于本地备份和回滚。目录里面会有 `images/*.tar`，它们是 Docker 镜像归档，不是源码压缩包。

`RELEASE.txt` 只是人工查看包版本的信息文件。部署脚本不依赖它；`ops/load-images.sh` 会直接读取 `images/*.tar`，并把每个服务最新加载到 Docker 的镜像自动标记为 `latest`。

## 上传到服务器

示例：

```bash
scp -r outputs/packages/aeonblight-server user@YOUR_SERVER:/tmp/
```

登录服务器后放到 `/opt`：

```bash
sudo mkdir -p /opt/aeonblight
sudo rm -rf /opt/aeonblight/aeonblight-server
sudo cp -R /tmp/aeonblight-server /opt/aeonblight/
sudo chown -R $USER:$USER /opt/aeonblight/aeonblight-server
cd /opt/aeonblight/aeonblight-server
```

也可以把目录直接传到 `/opt/aeonblight/` 下。

## 首次部署流程

首次部署推荐顺序是：先让容器服务本机健康，再配置 nginx 的 HTTP 代理，再申请/切换 HTTPS。这样如果某一步失败，能清楚地区分是容器问题、nginx 问题，还是证书/DNS 问题。

```bash
# 1. 安装 Docker、Compose、nginx、certbot、psql 等依赖
sudo ops/install-deps.sh

# 2. 配置环境变量
cp .env.example .env
nano .env

# 3. 加载本地已经编译好的应用镜像和迁移镜像
ops/load-images.sh

# 4. 启动 postgres 和 redis
ops/start-infra.sh

# 5. 初始化数据库。全新数据库只执行一次。
ops/migrate-db.sh bootstrap

# 6. 启动三个 API
ops/deploy.sh account-api economy-api admin-api

# 7. 注册 economy-worker 的 service identity 后，再启动 worker
ops/deploy.sh economy-worker

# 8. 先做容器本机健康检查
ops/healthcheck.sh

# 9. 配置 HTTP nginx。这个模式可用于 Let's Encrypt 验证，也可临时 HTTP 测试 API 转发。
ops/configure-nginx.sh http

# 10. 确认 nginx 已启动并监听 80
systemctl status nginx --no-pager
ss -lntp | grep ':80'

# 如果准备让 nginx 负责 HTTPS，443 不能被 xray、其他 nginx、caddy、apache 等进程占用。
ss -lntp | grep ':443' || true

# 11. 申请 api.aeonblight.com + game.aeonblight.com 的 HTTPS 证书，并自动切到 HTTPS 配置。
# 如果证书已经存在且未到期，certbot 输出 Certificate not yet due for renewal 是正常的。
ops/issue-cert.sh

# 12. 外网 HTTPS 健康检查
curl -i https://api.aeonblight.com/health/account
curl -i https://api.aeonblight.com/ready/account
curl -i https://api.aeonblight.com/health/economy
curl -i https://api.aeonblight.com/ready/economy
curl -i https://api.aeonblight.com/health/admin
curl -i https://api.aeonblight.com/ready/admin
```

已有数据库不要执行 `bootstrap`，只有数据库结构有更新时才执行：

```bash
ops/migrate-db.sh up
```

## 日常更新流程

已有服务器更新代码包时，通常只需要覆盖 `images/` 目录、加载新镜像、重建应用容器。不要重复执行 `bootstrap`，也不需要重复申请证书。

```bash
cd /aeon/aeonblight-server
ops/load-images.sh
ops/deploy.sh account-api economy-api admin-api economy-worker
ops/healthcheck.sh
```

如果本次发布包含数据库变更，在启动应用前执行：

```bash
ops/start-infra.sh
ops/migrate-db.sh up
ops/deploy.sh account-api economy-api admin-api economy-worker
ops/healthcheck.sh
```

## 健康检查

```bash
ops/healthcheck.sh
curl https://api.aeonblight.com/health/account
curl https://api.aeonblight.com/ready/account
curl https://api.aeonblight.com/health/economy
curl https://api.aeonblight.com/ready/economy
curl https://api.aeonblight.com/health/admin
curl https://api.aeonblight.com/ready/admin
```

如果 HTTPS 失败，先从服务器上检查 nginx 和证书：

```bash
nginx -t
systemctl status nginx --no-pager
journalctl -xeu nginx.service
ss -lntp | grep -E ':80|:443'
ls -l /etc/letsencrypt/live/${CERT_NAME:-aeonblight-api-game}/
```

如果 `ss -lntp` 显示 `*:443` 被 `xray`、`caddy`、`apache` 或其他非 nginx 进程占用，那么外部 HTTPS 流量不会进入 nginx。默认部署方案要求 nginx 独占 443；请停止或迁移占用 443 的进程，然后重新执行：

```bash
ops/configure-nginx.sh ssl
```

如果你必须让 xray 继续占用 443，则需要改成“xray 终止 TLS 并按域名反向代理到 nginx 或 127.0.0.1:8081/8082/8083”的方案，不能同时让 nginx 直接监听 443。

## 日常运维

```bash
ops/deploy.sh                         # 启动或更新所有 compose 服务
ops/deploy.sh economy-api             # 启动或更新一个服务
ops/load-images.sh                    # 上传新发布包后，先加载新镜像
ops/start.sh                          # 启动所有 compose 服务
ops/start.sh account-api              # 启动一个服务
ops/stop.sh                           # 停止所有 compose 服务
ops/stop.sh account-api               # 停止一个服务
ops/restart.sh                        # 重启所有 compose 服务
ops/restart.sh account-api            # 重启一个服务
ops/restart-service.sh economy-worker # 显式重启指定服务
ops/logs.sh economy-api               # 查看指定服务日志
ops/status.sh                         # 查看容器状态
ops/migrate-db.sh status              # 查看已应用的数据库版本
ops/migrate-db.sh up                  # 仅在有数据库更新时执行
ops/configure-nginx.sh http           # 重新渲染并启动 HTTP nginx 配置
ops/configure-nginx.sh ssl            # 重新渲染并启动 HTTPS nginx 配置，要求证书已存在
ops/issue-cert.sh                     # 首次申请或续期证书，成功后自动切到 HTTPS
```

## 生产注意事项

生产 / staging 环境要求：

- `STUB_MODE=disabled`
- `INTERNAL_KEY=` 必须为空
- Solana RPC、mint、充值钱包、提现钱包、提现私钥必须是真实配置
- `economy-worker` 必须使用 Ed25519 service identity

启动 `economy-worker` 前，需要通过 `POST /api/admin/service-identities` 注册 worker 的公钥，并授予 `economy.worker` capability。
