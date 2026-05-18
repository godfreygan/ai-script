# AI Script 部署指南

当前仓库只保留两种运行方案：

1. Docker 部署
2. 手动编译运行

## 通用前提

所有方案都使用根目录 `[.env.example](/E:/data/go/src/github.com/godfreygan/ai-script/.env.example:1)` 作为模板：

```bash
cp .env.example .env
```

至少需要配置：

- `JWT_SECRET`
- `CRYPTO_KEY_BASE64`
- `MYSQL_HOST`
- `MYSQL_PORT`
- `MYSQL_USER`
- `MYSQL_PASSWORD`
- `MYSQL_DATABASE`
- `REDIS_HOST`
- `REDIS_PORT`
- `APP_ORIGINS`

对象存储有两种常见做法：

- 本地开发：`OSS_PROVIDER=local`，`OSS_BUCKET` 指向可写目录
- 部署环境：使用 MinIO、S3、OSS、COS 之一

## Docker 部署

适用场景：

- 本地开发
- 测试环境
- 单机部署
- 需要快速拉起完整依赖

### 依赖

- Docker Engine 24+
- Docker Compose v2

Windows 和 macOS 推荐使用 Docker Desktop。

### 本地一键运行

```bash
docker compose up -d --build
```

默认会启动：

- `frontend`
- `server`
- `worker`
- `mysql`
- `redis`
- `minio`

常用地址：

- 前端：`http://localhost`
- 后端：`http://localhost:8080`
- Server 健康检查：`http://localhost:8080/healthz/live`

### 单机部署

如果你想用偏生产的单机编排，使用：

```bash
docker compose -f deploy/docker-compose.yml --env-file .env up -d --build
```

建议：

- `MYSQL_*`、`REDIS_*`、`OSS_*` 指向外部真实服务
- 前端域名写入 `APP_ORIGINS`
- 不要长期把数据库和业务容器放在同一台临时机器上

### Docker 方案优点

- 环境一致性最好
- Windows/macOS/Linux 都容易落地
- `ffmpeg`、Nginx、Go 运行时都已打进镜像

### Docker 方案限制

- 更依赖容器运行环境
- 单机 compose 不是高可用方案

## 手动编译运行

适用场景：

- 不能使用 Docker 的服务器
- 受限内网环境
- 需要接入已有 Nginx / systemd 体系

更推荐 Linux。Windows 如需手动运行，建议使用 WSL2，而不是直接在原生 Windows 服务里拼装运行时。

### 依赖

- Go 1.25+
- Node.js 20+
- npm
- ffmpeg
- Nginx 或其他静态文件服务器
- MySQL 8+
- Redis 7+

### 1. 准备配置

复制模板：

```bash
cp .env.example .env
```

将 `.env` 中的数据库、Redis、对象存储、JWT、加密密钥改为真实值。

### 2. 编译后端

```bash
cd backend
go build -trimpath -ldflags="-s -w" -o out/server ./cmd/server
go build -trimpath -ldflags="-s -w" -o out/worker ./cmd/worker
```

说明：

- `server` 提供 HTTP API 和 WebSocket
- `worker` 负责异步任务、媒体处理和健康检查

### 3. 构建前端

```bash
cd frontend
npm ci
npm run build
```

构建结果在 `frontend/dist/`。

### 4. 启动后端与 worker

Linux / macOS：

```bash
cd backend
./out/server
./out/worker
```

Windows PowerShell：

```powershell
cd backend
.\out\server.exe
.\out\worker.exe
```

建议不要直接前台长期运行，生产环境请交给 `systemd`、`supervisord` 或同类进程管理器。

### 5. 发布前端

有两种常见方式：

- 开发验证：`npm run preview -- --host 0.0.0.0 --port 80`
- 正式运行：把 `frontend/dist/` 交给 Nginx

如果使用 Nginx，可以参考仓库中的 [frontend/nginx.conf](/E:/data/go/src/github.com/godfreygan/ai-script/frontend/nginx.conf:1)：

- `/` 提供前端静态资源
- `/api/` 反向代理到后端
- `/ws/` 反向代理到 WebSocket

手动部署时，需要把其中的 `backend:8080` 改成你的实际后端地址，例如 `127.0.0.1:8080` 或内网域名。

### 6. 健康检查

- server：`GET /healthz/live`
- server：`GET /healthz/ready`
- worker：`GET http://<host>:<APP_PORT+1000>/healthz/live`
- worker：`GET http://<host>:<APP_PORT+1000>/healthz/ready`

### 手动方案优点

- 不依赖 Docker
- 更容易接入已有主机运维体系
- 适合强管控环境

### 手动方案限制

- 环境一致性不如 Docker
- 需要自己保证 `ffmpeg`、Nginx、Go、Node 的版本和可用性
- 部署步骤更多，出错点也更多

## 方案选择建议

- 开发联调：优先 Docker
- 单机测试：优先 Docker
- 受限服务器：使用手动编译运行
- 长期维护成本优先低：优先 Docker

