# AI-Script 系统运维与部署指南

> 版本：v1.0
> 更新日期：2026-05-12
> 适用版本：Go 1.22 + AI-Script Backend

---

## 目录

1. [系统架构概述](#1-系统架构概述)
2. [系统需求](#2-系统需求)
3. [依赖安装](#3-依赖安装)
4. [配置详解](#4-配置详解)
5. [部署方式](#5-部署方式)
   - 5.1 Docker 部署（推荐）
   - 5.2 Linux 二进制部署
   - 5.3 Windows 二进制部署
   - 5.4 Kubernetes 部署
6. [数据库初始化与迁移](#6-数据库初始化与迁移)
7. [启动与停止](#7-启动与停止)
8. [监控与日志](#8-监控与日志)
9. [备份与恢复策略](#9-备份与恢复策略)
10. [故障排查手册](#10-故障排查手册)
11. [安全加固建议](#11-安全加固建议)
12. [升级指南](#12-升级指南)

---

## 1. 系统架构概述

AI-Script 采用**双进程架构**：

- **server**：HTTP API 服务，基于 Gin 框架，对外提供 RESTful API 与 WebSocket 进度推送。
- **worker**：异步任务消费者，基于 Asynq 从 Redis 拉取任务，执行 LLM 调用、媒体合成等耗时操作。

两进程共享：
- **MySQL**：业务数据持久化（GORM）
- **Redis**：缓存 + Asynq 任务队列 + WebSocket Pub/Sub 桥接
- **对象存储**：上传文件与生成资源（S3 / MinIO / 本地文件系统）

```
+--------+      HTTP       +---------+
| Client | <-------------> |  server |
+--------+                 +---------+
                              | WS
                              v
+--------+     Redis      +---------+
| Worker | <------------> |  Redis  |
+--------+   Asynq Queue  +---------+
     |                            |
     v                            v
+---------+                +----------+
|  MySQL  |                | Storage  |
+---------+                +----------+
```

---

## 2. 系统需求

### 2.1 硬件需求

| 规模 | CPU | 内存 | 磁盘 | 网络 |
|------|-----|------|------|------|
| 开发/测试 | 2 核 | 4 GB | 50 GB SSD | 10 Mbps |
| 生产（单节点） | 4 核 | 8 GB | 200 GB SSD | 50 Mbps |
| 生产（高可用） | 8 核+ | 16 GB+ | 500 GB SSD | 100 Mbps+ |

> 注：媒体处理（FFmpeg 合成）对 CPU 与磁盘 I/O 敏感，建议生产环境预留 2 倍余量。

### 2.2 软件需求

| 组件 | 最低版本 | 说明 |
|------|---------|------|
| Go | 1.22 | 编译必需 |
| MySQL | 5.7+ | 主数据库，推荐 8.0 |
| Redis | 6.0+ | 缓存 + 队列 + Pub/Sub |
| FFmpeg | 5.0+ | 媒体处理（Docker 镜像已内置） |
| Docker | 20.10+ | 容器化部署 |
| Docker Compose | 2.0+ | 多服务编排 |

### 2.3 网络需求

- server 暴露端口：**8080**（HTTP API）
- MySQL 端口：**3306**
- Redis 端口：**6379**
- 对象存储端口：视 provider 而定（S3: 443, MinIO: 9000/9001）
- 出站：需访问 AI 模型网关（OpenAI / 阿里云 / 自建 LiteLLM 等）

---

## 3. 依赖安装

### 3.1 MySQL 5.7+ / 8.0

**Linux (Ubuntu/Debian)**

```bash
sudo apt update
sudo apt install -y mysql-server-8.0
sudo mysql_secure_installation

# 创建数据库与用户
sudo mysql -u root -p
```

**Windows**

1. 下载 MySQL Installer：https://dev.mysql.com/downloads/installer/
2. 选择 **Server only** 安装类型
3. 记住 root 密码，启用 MySQL 服务（默认开机自启）
4. 使用 MySQL Command Line Client 或 MySQL Workbench 执行初始化 SQL：

```sql
CREATE DATABASE ai_script CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'aiscript'@'%' IDENTIFIED BY 'YourStrongPassword123!';
GRANT ALL PRIVILEGES ON ai_script.* TO 'aiscript'@'%';
FLUSH PRIVILEGES;
```

### 3.2 Redis 6.0+

**Linux (Ubuntu/Debian)**

```bash
sudo apt install -y redis-server
sudo systemctl enable redis-server
sudo systemctl start redis-server

# 如需密码，修改 /etc/redis/redis.conf
# requirepass YourRedisPassword
sudo systemctl restart redis-server
```

**Windows**

1. 下载 Redis for Windows（微软维护版或 Memurai）：
   - 推荐：https://github.com/microsoftarchive/redis/releases
   - 或商业版 Memurai：https://www.memurai.com/
2. 解压到 `C:\Redis`，将目录加入系统 PATH
3. 启动 Redis 服务：

```powershell
# 注册为 Windows 服务（以管理员身份运行 PowerShell）
cd C:\Redis
redis-server --service-install redis.windows.conf --loglevel verbose
redis-server --service-start
```

### 3.3 FFmpeg

**Linux (Ubuntu/Debian)**

```bash
sudo apt install -y ffmpeg
ffmpeg -version
```

**macOS**

```bash
brew install ffmpeg
```

**Windows**

1. 下载 Windows 构建版：https://www.gyan.dev/ffmpeg/builds/（选择 `ffmpeg-release-essentials.7z`）
2. 解压到 `C:\ffmpeg`
3. 将 `C:\ffmpeg\bin` 加入系统 PATH 环境变量
4. 验证：

```powershell
ffmpeg -version
```

### 3.4 对象存储（三选一）

#### 选项 A：本地文件系统（开发/测试）

无需额外安装，配置 `provider: local`，程序自动创建 `./var/uploads` 目录。

#### 选项 B：MinIO（自建对象存储）

```bash
docker run -d \
  --name minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin123 \
  -v /data/minio:/data \
  minio/minio server /data --console-address ":9001"
```

创建 bucket：

```bash
mc alias set local http://localhost:9000 minioadmin minioadmin123
mc mb local/ai-script
mc anonymous set download local/ai-script
```

#### 选项 C：阿里云 OSS / AWS S3 / 腾讯云 COS

在对应云控制台创建 bucket，获取 AccessKey / SecretKey，记录 Endpoint 与 Region。

---

## 4. 配置详解

配置文件路径：`configs/config.yaml`

配置加载顺序：
1. 读取 `configs/config.yaml`
2. 环境变量以 `_` 分隔覆盖同名配置（如 `APP_ENV` 覆盖 `app.env`）
3. 支持 `${VAR:default}` 语法展开

### 4.1 完整配置示例

```yaml
app:
  name: ai-script
  env: prod
  port: 8080
  log_level: info

jwt:
  secret: "change-me-to-64-char-random-string-in-production"
  access_expires_in: 7200
  refresh_expires_in: 604800

mysql:
  dsn: "aiscript:YourStrongPassword123!@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC&time_zone=%27%2B00%3A00%27"
  max_idle: 10
  max_open: 100

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0

storage:
  provider: local          # local | s3 | aliyun | cos
  endpoint: ""
  region: ""
  bucket: "./var/uploads"  # local 时为目录路径；云存储时为 bucket 名
  access_key: ""
  secret_key: ""
  public_host: "/uploads"  # 本地为 URL 前缀；云存储为 CDN 域名

crypto:
  key: "Aix2XN/944vtHhBM5bd2Ibv5SQTCgZ/GwHheRVlwyiQ="

model_gateway:
  url: "https://your-litellm.example.com/v1"
  key: "sk-your-gateway-key"
```

### 4.2 配置项说明

| 配置项 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `app.name` | string | 是 | `ai-script` | 应用名称，用于日志标识 |
| `app.env` | string | 是 | `dev` | 环境：`dev` / `staging` / `prod` |
| `app.port` | int | 是 | `8080` | HTTP 监听端口 |
| `app.log_level` | string | 是 | `info` | 日志级别：`debug` / `info` / `warn` / `error` |
| `jwt.secret` | string | 是 | - | JWT 签名密钥，**生产环境必须更换** |
| `jwt.access_expires_in` | int | 是 | `7200` | Access Token 过期时间（秒） |
| `jwt.refresh_expires_in` | int | 是 | `604800` | Refresh Token 过期时间（秒） |
| `mysql.dsn` | string | 是 | - | GORM MySQL DSN |
| `mysql.max_idle` | int | 否 | `10` | 连接池最大空闲连接数 |
| `mysql.max_open` | int | 否 | `100` | 连接池最大打开连接数 |
| `redis.addr` | string | 是 | `127.0.0.1:6379` | Redis 地址 |
| `redis.password` | string | 否 | - | Redis 密码 |
| `redis.db` | int | 否 | `0` | Redis 数据库编号 |
| `storage.provider` | string | 是 | `local` | 存储提供商 |
| `storage.endpoint` | string | 条件 | - | S3/MinIO Endpoint |
| `storage.region` | string | 条件 | - | S3/阿里云 Region |
| `storage.bucket` | string | 是 | - | Bucket 名或本地目录 |
| `storage.access_key` | string | 条件 | - | 云存储 AccessKey |
| `storage.secret_key` | string | 条件 | - | 云存储 SecretKey |
| `storage.public_host` | string | 是 | - | 对外访问的 URL 前缀 |
| `crypto.key` | string | 是 | - | AES-256-GCM 加密密钥（Base64，32 字节） |
| `model_gateway.url` | string | 否 | - | LiteLLM / OneAPI 网关地址 |
| `model_gateway.key` | string | 否 | - | 网关 API Key |

### 4.3 环境变量映射

所有配置项均可通过环境变量覆盖，规则为：将配置路径的点 `.` 替换为下划线 `_`，并转为大写。

| 环境变量 | 对应配置 |
|----------|----------|
| `APP_ENV` | `app.env` |
| `APP_PORT` | `app.port` |
| `APP_LOG_LEVEL` | `app.log_level` |
| `JWT_SECRET` | `jwt.secret` |
| `JWT_EXPIRES_IN` | `jwt.access_expires_in` |
| `JWT_REFRESH_EXPIRES_IN` | `jwt.refresh_expires_in` |
| `MYSQL_DSN` | `mysql.dsn` |
| `MYSQL_MAX_IDLE` | `mysql.max_idle` |
| `MYSQL_MAX_OPEN` | `mysql.max_open` |
| `REDIS_ADDR` | `redis.addr` |
| `REDIS_PASSWORD` | `redis.password` |
| `REDIS_DB` | `redis.db` |
| `OSS_PROVIDER` | `storage.provider` |
| `OSS_ENDPOINT` | `storage.endpoint` |
| `OSS_REGION` | `storage.region` |
| `OSS_BUCKET` | `storage.bucket` |
| `OSS_ACCESS_KEY` | `storage.access_key` |
| `OSS_SECRET_KEY` | `storage.secret_key` |
| `OSS_PUBLIC_HOST` | `storage.public_host` |
| `CRYPTO_KEY` | `crypto.key` |
| `MODEL_GATEWAY_URL` | `model_gateway.url` |
| `MODEL_GATEWAY_KEY` | `model_gateway.key` |

### 4.4 生成加密密钥

```bash
cd /path/to/ai-script/backend
go run ./cmd/genkey
# 输出示例：Aix2XN/944vtHhBM5bd2Ibv5SQTCgZ/GwHheRVlwyiQ=
```

---

---

## 5. 部署方式

> 平台支持三种部署方式：
> - **Docker 部署（推荐）**：一键拉起全部依赖，适合开发测试与小型生产环境
> - **Linux 二进制部署**：高性能、资源可控，适合生产环境
> - **Windows 二进制部署**：适合 Windows Server 或本地开发机

### 5.1 Docker 部署（推荐）

Docker 部署包含全套依赖（MySQL、Redis、Backend、Worker、Frontend），适合快速验证与小型生产环境。

#### 前置条件

- Docker 20.10+、Docker Compose 2.0+
- 无需单独安装 MySQL / Redis / FFmpeg

#### 步骤 1：准备环境

```bash
cd deploy
cp .env.example .env
# 编辑 .env，修改 JWT_SECRET 和 CRYPTO_KEY_BASE64
```

#### 步骤 2：启动全部服务

```bash
docker compose up -d

# 查看状态
docker compose ps

# 查看日志
docker compose logs -f backend
docker compose logs -f worker

# 停止
docker compose down

# 停止并清除数据（慎用）
docker compose down -v
```

服务启动后访问：
- 前端：http://localhost/
- 后端 API：http://localhost:8080

#### 步骤 3：初始化数据

首次启动后，MySQL 会自动执行 `scripts/sql/001_init.sql` 中的 DDL 和种子数据。默认管理员账号为 `admin` / `admin@123`。

#### Docker 单容器部署（高级）

如已有外部 MySQL / Redis，可仅部署 backend / worker 容器：

```bash
# 构建镜像
cd backend
docker build -t ai-script:latest .

# 运行 server
docker run -d \
  --name ai-script-server \
  -p 8080:8080 \
  -e APP_ENV=prod \
  -e MYSQL_DSN="aiscript:password@tcp(host.docker.internal:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC" \
  -e REDIS_ADDR="host.docker.internal:6379" \
  -e CRYPTO_KEY="your-generated-key" \
  ai-script:latest

# 运行 worker
docker run -d \
  --name ai-script-worker \
  -e APP_ENV=prod \
  -e MYSQL_DSN="aiscript:password@tcp(host.docker.internal:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC" \
  -e REDIS_ADDR="host.docker.internal:6379" \
  -e CRYPTO_KEY="your-generated-key" \
  --entrypoint /app/worker \
  ai-script:latest
```

---

### 5.2 Linux 二进制部署

适合对性能和资源有精确控制需求的生产环境。

#### 编译

```bash
cd backend

# 安装依赖
export GOPROXY=https://goproxy.cn,direct
go mod download

# 构建二进制
make build
# 输出：out/server, out/worker
```

#### 目录结构

```
/opt/ai-script/
├── server              # 二进制
├── worker              # 二进制
├── configs/
│   ├── config.yaml     # 配置文件
│   └── rbac_model.conf # RBAC 模型定义
├── var/
│   └── uploads/        # 本地存储目录
└── logs/               # 日志目录
```

#### 手动启动

```bash
export APP_ENV=prod
export APP_PORT=8080
export MYSQL_DSN="aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC"
export REDIS_ADDR="127.0.0.1:6379"
export CRYPTO_KEY="your-generated-key"

./server
./worker
```

#### systemd 服务（推荐）

创建 `/etc/systemd/system/ai-script-server.service`：

```ini
[Unit]
Description=AI-Script Server
After=network.target mysql.service redis.service

[Service]
Type=simple
User=ai-script
Group=ai-script
WorkingDirectory=/opt/ai-script
Environment="APP_ENV=prod"
Environment="APP_PORT=8080"
Environment="MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC"
Environment="REDIS_ADDR=127.0.0.1:6379"
Environment="CRYPTO_KEY=your-generated-key"
Environment="GIN_MODE=release"
ExecStart=/opt/ai-script/server
ExecStop=/bin/kill -SIGTERM $MAINPID
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/ai-script/server.log
StandardError=append:/var/log/ai-script/server.log

[Install]
WantedBy=multi-user.target
```

创建 `/etc/systemd/system/ai-script-worker.service`：

```ini
[Unit]
Description=AI-Script Worker
After=network.target mysql.service redis.service

[Service]
Type=simple
User=ai-script
Group=ai-script
WorkingDirectory=/opt/ai-script
Environment="APP_ENV=prod"
Environment="MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC"
Environment="REDIS_ADDR=127.0.0.1:6379"
Environment="CRYPTO_KEY=your-generated-key"
ExecStart=/opt/ai-script/worker
ExecStop=/bin/kill -SIGTERM $MAINPID
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/ai-script/worker.log
StandardError=append:/var/log/ai-script/worker.log

[Install]
WantedBy=multi-user.target
```

启用并启动：

```bash
sudo useradd -r -s /bin/false ai-script
sudo mkdir -p /opt/ai-script /var/log/ai-script
sudo chown -R ai-script:ai-script /opt/ai-script /var/log/ai-script

sudo systemctl daemon-reload
sudo systemctl enable ai-script-server ai-script-worker
sudo systemctl start ai-script-server ai-script-worker
```

---

### 5.3 Windows 二进制部署

适合 Windows Server 或本地开发机部署。

#### 编译

```powershell
# 在 PowerShell 中
cd backend

# 设置 Go 代理
$env:GOPROXY = "https://goproxy.cn,direct"
go mod download

# 构建 Windows 二进制
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o out/server.exe ./cmd/server
go build -o out/worker.exe ./cmd/worker
```

#### 目录结构

```
C:\ai-script\
├── server.exe          # 二进制
├── worker.exe          # 二进制
├── configs\
│   ├── config.yaml     # 配置文件
│   └── rbac_model.conf # RBAC 模型定义
├── var\
│   └── uploads\        # 本地存储目录
└── logs\               # 日志目录
```

#### 手动启动

```powershell
$env:APP_ENV = "prod"
$env:APP_PORT = "8080"
$env:MYSQL_DSN = "aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC"
$env:REDIS_ADDR = "127.0.0.1:6379"
$env:CRYPTO_KEY = "your-generated-key"

.\server.exe
.\worker.exe
```

#### 注册为 Windows Service（推荐）

使用 **NSSM**（Non-Sucking Service Manager）将程序注册为 Windows 服务，支持开机自启、自动重启、日志轮转。

**步骤 1：下载 NSSM**

```powershell
# 下载 NSSM
Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile "nssm.zip"
Expand-Archive -Path "nssm.zip" -DestinationPath "C:\nssm"
# 将 C:\nssm\win64 加入系统 PATH
```

**步骤 2：注册 Server 服务**

```powershell
# 以管理员身份运行 PowerShell
nssm install ai-script-server
# 在弹出的 GUI 中设置：
#   Path: C:\ai-script\server.exe
#   Startup directory: C:\ai-script
#   Arguments: （留空）
# 然后点击 "Install service"

# 设置环境变量
nssm set ai-script-server AppEnvironmentExtra "APP_ENV=prod" "APP_PORT=8080" "MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC" "REDIS_ADDR=127.0.0.1:6379" "CRYPTO_KEY=your-generated-key"

# 设置日志输出（可选，NSSM 会自动轮转）
nssm set ai-script-server AppStdout C:\ai-script\logs\server.log
nssm set ai-script-server AppStderr C:\ai-script\logs\server.log

# 启动服务
nssm start ai-script-server
```

**步骤 3：注册 Worker 服务**

```powershell
nssm install ai-script-worker
# 在弹出的 GUI 中设置：
#   Path: C:\ai-script\worker.exe
#   Startup directory: C:\ai-script
#   Arguments: （留空）

nssm set ai-script-worker AppEnvironmentExtra "APP_ENV=prod" "MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC" "REDIS_ADDR=127.0.0.1:6379" "CRYPTO_KEY=your-generated-key"
nssm set ai-script-worker AppStdout C:\ai-script\logs\worker.log
nssm set ai-script-worker AppStderr C:\ai-script\logs\worker.log

nssm start ai-script-worker
```

**步骤 4：管理服务**

```powershell
# 查看状态
nssm status ai-script-server
nssm status ai-script-worker

# 停止
nssm stop ai-script-server
nssm stop ai-script-worker

# 重启
nssm restart ai-script-server
nssm restart ai-script-worker

# 删除服务（如需卸载）
nssm remove ai-script-server confirm
nssm remove ai-script-worker confirm
```

> **提示**：也可以使用 Windows 自带的 `sc.exe` 注册服务，但 NSSM 更灵活，支持环境变量、日志重定向和自动重启。

---

### 5.4 Kubernetes 部署

适合需要高可用、自动扩缩容的生产环境。

#### Namespace 与 ConfigMap

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ai-script
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ai-script-config
  namespace: ai-script
data:
  config.yaml: |
    app:
      name: ai-script
      env: prod
      port: 8080
      log_level: info
    jwt:
      secret: "k8s-secret-from-secrets"
      access_expires_in: 7200
      refresh_expires_in: 604800
    mysql:
      dsn: "aiscript:password@tcp(mysql.ai-script.svc.cluster.local:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC"
      max_idle: 10
      max_open: 100
    redis:
      addr: "redis.ai-script.svc.cluster.local:6379"
      password: ""
      db: 0
    storage:
      provider: s3
      endpoint: ""
      region: "cn-hangzhou"
      bucket: "ai-script"
      access_key: ""
      secret_key: ""
      public_host: "https://ai-script.oss-cn-hangzhou.aliyuncs.com"
    crypto:
      key: "k8s-crypto-from-secrets"
    model_gateway:
      url: ""
      key: ""
```

#### Secret（敏感信息）

```bash
# 生成 base64 编码的 secret
kubectl create secret generic ai-script-secrets \
  -n ai-script \
  --from-literal=jwt-secret="your-64-char-random-jwt-secret" \
  --from-literal=crypto-key="your-generated-crypto-key" \
  --from-literal=oss-access-key="your-oss-ak" \
  --from-literal=oss-secret-key="your-oss-sk" \
  --from-literal=model-gateway-key="your-gateway-key"
```

#### Server Deployment + Service + Ingress

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-script-server
  namespace: ai-script
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ai-script-server
  template:
    metadata:
      labels:
        app: ai-script-server
    spec:
      containers:
        - name: server
          image: your-registry/ai-script:latest
          imagePullPolicy: Always
          ports:
            - containerPort: 8080
          env:
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: jwt-secret
            - name: CRYPTO_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: crypto-key
            - name: OSS_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: oss-access-key
            - name: OSS_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: oss-secret-key
            - name: MODEL_GATEWAY_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: model-gateway-key
          volumeMounts:
            - name: config
              mountPath: /app/configs
              readOnly: true
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "1Gi"
              cpu: "1000m"
      volumes:
        - name: config
          configMap:
            name: ai-script-config
---
apiVersion: v1
kind: Service
metadata:
  name: ai-script-server
  namespace: ai-script
spec:
  selector:
    app: ai-script-server
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ai-script-ingress
  namespace: ai-script
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "100m"
spec:
  rules:
    - host: api.ai-script.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ai-script-server
                port:
                  number: 80
```

#### Worker Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-script-worker
  namespace: ai-script
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ai-script-worker
  template:
    metadata:
      labels:
        app: ai-script-worker
    spec:
      containers:
        - name: worker
          image: your-registry/ai-script:latest
          imagePullPolicy: Always
          command: ["/app/worker"]
          env:
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: jwt-secret
            - name: CRYPTO_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: crypto-key
            - name: OSS_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: oss-access-key
            - name: OSS_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: oss-secret-key
            - name: MODEL_GATEWAY_KEY
              valueFrom:
                secretKeyRef:
                  name: ai-script-secrets
                  key: model-gateway-key
          volumeMounts:
            - name: config
              mountPath: /app/configs
              readOnly: true
          resources:
            requests:
              memory: "512Mi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "2000m"
      volumes:
        - name: config
          configMap:
            name: ai-script-config
```

#### 部署命令

```bash
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f server-deployment.yaml
kubectl apply -f worker-deployment.yaml

# 查看状态
kubectl get pods -n ai-script
kubectl logs -f deployment/ai-script-server -n ai-script
kubectl logs -f deployment/ai-script-worker -n ai-script
```

---

## 6. 数据库初始化与迁移

### 6.1 自动迁移（MVP 阶段）

server / worker 启动时会自动执行 GORM AutoMigrate，创建所有业务表。该方式**只增不删**，适合 MVP 阶段快速迭代。

涉及的表：
- `users`, `departments`, `roles`, `projects`, `project_members`
- `user_api_tokens`, `permissions`, `role_permissions`, `user_roles`
- `models`
- `scripts`, `script_versions`, `episodes`, `episode_prompts`
- `storyboards`, `styles`, `storyboard_styles`
- `images`, `short_videos`, `full_videos`
- `pipelines`, `pipeline_runs`, `step_runs`
- `review_flows`, `review_nodes`, `review_records`, `review_node_records`
- `publishes`
- `model_pricings`, `model_invocations`
- `billing_quotas`, `billing_dailies`
- `audit_logs`, `sys_dicts`

### 6.2 生产环境迁移建议

生产环境建议使用 **golang-migrate** 或 **Atlas** 管理 schema 变更：

```bash
# 安装 golang-migrate
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 创建迁移文件
migrate create -ext sql -dir migrations -seq init_schema

# 执行迁移
migrate -path migrations -database "mysql://aiscript:password@tcp(localhost:3306)/ai_script" up

# 回滚
migrate -path migrations -database "mysql://aiscript:password@tcp(localhost:3306)/ai_script" down 1
```

### 6.3 初始化数据

首次部署后，建议执行以下初始化：

```sql
-- 创建系统管理员
INSERT INTO users (username, password_hash, nickname, email, status, created_at, updated_at)
VALUES ('admin', '$2a$10$...bcrypt-hash...', '系统管理员', 'admin@example.com', 1, NOW(), NOW());

-- 创建默认角色
INSERT INTO roles (code, name, description, status, created_at, updated_at)
VALUES ('admin', '管理员', '系统管理员', 1, NOW(), NOW()),
       ('user', '普通用户', '普通用户', 1, NOW(), NOW());
```

---

## 7. 启动与停止

### 7.1 手动启动

**Linux**

```bash
# 前台启动（调试）
./server
./worker

# 后台启动（nohup）
nohup ./server > logs/server.log 2>&1 &
nohup ./worker > logs/worker.log 2>&1 &
```

**Windows**

```powershell
# 前台启动（PowerShell）
.\server.exe
.\worker.exe

# 后台启动（使用 Start-Process）
Start-Process -FilePath ".\server.exe" -RedirectStandardOutput "logs\server.log" -RedirectStandardError "logs\server.log" -WindowStyle Hidden
Start-Process -FilePath ".\worker.exe" -RedirectStandardOutput "logs\worker.log" -RedirectStandardError "logs\worker.log" -WindowStyle Hidden
```

### 7.2 systemd 服务文件

创建 `/etc/systemd/system/ai-script-server.service`：

```ini
[Unit]
Description=AI-Script Server
After=network.target mysql.service redis.service

[Service]
Type=simple
User=ai-script
Group=ai-script
WorkingDirectory=/opt/ai-script
Environment="APP_ENV=prod"
Environment="APP_PORT=8080"
Environment="MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC"
Environment="REDIS_ADDR=127.0.0.1:6379"
Environment="CRYPTO_KEY=your-generated-key"
Environment="GIN_MODE=release"
ExecStart=/opt/ai-script/server
ExecStop=/bin/kill -SIGTERM $MAINPID
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/ai-script/server.log
StandardError=append:/var/log/ai-script/server.log

[Install]
WantedBy=multi-user.target
```

创建 `/etc/systemd/system/ai-script-worker.service`：

```ini
[Unit]
Description=AI-Script Worker
After=network.target mysql.service redis.service

[Service]
Type=simple
User=ai-script
Group=ai-script
WorkingDirectory=/opt/ai-script
Environment="APP_ENV=prod"
Environment="MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=UTC"
Environment="REDIS_ADDR=127.0.0.1:6379"
Environment="CRYPTO_KEY=your-generated-key"
ExecStart=/opt/ai-script/worker
ExecStop=/bin/kill -SIGTERM $MAINPID
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/ai-script/worker.log
StandardError=append:/var/log/ai-script/worker.log

[Install]
WantedBy=multi-user.target
```

#### 启用服务

```bash
sudo useradd -r -s /bin/false ai-script
sudo mkdir -p /opt/ai-script /var/log/ai-script
sudo chown -R ai-script:ai-script /opt/ai-script /var/log/ai-script

sudo systemctl daemon-reload
sudo systemctl enable ai-script-server ai-script-worker
sudo systemctl start ai-script-server ai-script-worker

# 查看状态
sudo systemctl status ai-script-server
sudo systemctl status ai-script-worker

# 查看日志
sudo journalctl -u ai-script-server -f
sudo journalctl -u ai-script-worker -f

# 重启
sudo systemctl restart ai-script-server
sudo systemctl restart ai-script-worker
```

### 7.3 Windows Service 管理

如使用 NSSM 注册为 Windows Service（见 §5.3）：

```powershell
# 查看状态
nssm status ai-script-server
nssm status ai-script-worker

# 停止
nssm stop ai-script-server
nssm stop ai-script-worker

# 重启
nssm restart ai-script-server
nssm restart ai-script-worker

# 删除服务（卸载）
nssm remove ai-script-server confirm
nssm remove ai-script-worker confirm
```

也可通过 Windows 服务管理器（`services.msc`）或 PowerShell 管理：

```powershell
# PowerShell
Get-Service ai-script-server, ai-script-worker
Stop-Service ai-script-server, ai-script-worker
Start-Service ai-script-server, ai-script-worker
Restart-Service ai-script-server, ai-script-worker
```

### 7.4 优雅停止

server 与 worker 均监听 `SIGINT` / `SIGTERM`，收到信号后：
- server：在 10 秒内完成正在处理的 HTTP 请求后退出
- worker：停止消费新任务，等待当前任务完成后退出

**Linux**

```bash
# 优雅停止
sudo systemctl stop ai-script-server
sudo systemctl stop ai-script-worker

# 强制停止（慎用）
sudo systemctl kill -s SIGKILL ai-script-server
```

**Windows**

```powershell
# NSSM 会自动发送 CTRL_CLOSE_EVENT，程序捕获后优雅退出
nssm stop ai-script-server
nssm stop ai-script-worker

# 强制停止（任务管理器或 PowerShell）
Stop-Process -Name "server" -Force
Stop-Process -Name "worker" -Force
```

---

## 8. 监控与日志

### 8.1 日志配置

日志基于 **Zap** 实现：

- `env=prod`：JSON 格式输出，适合日志收集系统（ELK / Loki）
- `env=dev`：带颜色的控制台输出，便于本地调试

日志级别：`debug` < `info` < `warn` < `error`

### 8.2 日志位置

| 部署方式 | 日志位置 |
|----------|----------|
| 手动启动 (Linux) | 标准输出 / nohup.out |
| 手动启动 (Windows) | PowerShell 窗口 / 重定向文件 |
| systemd | `/var/log/ai-script/*.log` 或 `journalctl` |
| Windows Service | `C:\ai-script\logs\*.log` |
| Docker | `docker logs <container>` |
| K8s | `kubectl logs <pod>` |

### 8.3 健康检查端点

```bash
curl http://localhost:8080/healthz
# 返回：{"status":"ok"}
```

### 8.4 关键监控指标

| 指标 | 检查方式 | 告警阈值 |
|------|----------|----------|
| HTTP 服务可用 | `curl /healthz` | 连续 3 次失败 |
| MySQL 连接 | 业务日志 | 连接超时 |
| Redis 连接 | 业务日志 | 连接超时 |
| 任务队列积压 | Redis `LLEN asynq:{default}` | > 1000 |
| 磁盘空间 | `df -h` | > 80% |
| 内存使用 | `free -m` | > 85% |

### 8.5 Prometheus 监控（扩展）

如需接入 Prometheus，可在 Gin 路由中增加 `/metrics` 端点，暴露 Go runtime 与业务自定义指标。

---

## 9. 备份与恢复策略

### 9.1 MySQL 备份

```bash
# 每日全量备份（crontab）
0 2 * * * mysqldump -u aiscript -p'password' --single-transaction --routines ai_script > /backup/ai-script-$(date +\%Y\%m\%d).sql

# 保留 7 天
find /backup -name "ai-script-*.sql" -mtime +7 -delete

# 压缩备份
mysqldump -u aiscript -p'password' --single-transaction ai_script | gzip > /backup/ai-script-$(date +%Y%m%d).sql.gz
```

### 9.2 MySQL 恢复

```bash
# 恢复全量备份
mysql -u aiscript -p ai_script < /backup/ai-script-20260512.sql

# 恢复压缩备份
gunzip < /backup/ai-script-20260512.sql.gz | mysql -u aiscript -p ai_script
```

### 9.3 Redis 备份

```bash
# 开启 AOF 持久化（/etc/redis/redis.conf）
appendonly yes

# 手动触发 RDB 保存
redis-cli BGSAVE

# 备份 RDB 文件
cp /var/lib/redis/dump.rdb /backup/redis-$(date +%Y%m%d).rdb
```

### 9.4 对象存储备份

- **本地存储**：定期 rsync 到备份服务器
- **云存储**：开启版本控制（Versioning）与跨区域复制
- **MinIO**：使用 `mc mirror` 同步到备份 bucket

```bash
mc mirror local/ai-script backup/ai-script-backup
```

### 9.5 配置文件备份

```bash
tar czf /backup/ai-script-config-$(date +%Y%m%d).tar.gz /opt/ai-script/configs /opt/ai-script/.env
```

---

## 10. 故障排查手册

### 10.1 服务无法启动

**现象**：执行 `./server` 后立即退出

**排查步骤**：

```bash
# 1. 检查配置文件是否存在
ls -la configs/config.yaml

# 2. 检查环境变量
echo $MYSQL_DSN $REDIS_ADDR $CRYPTO_KEY

# 3. 检查依赖服务
mysql -u aiscript -p -e "SELECT 1"
redis-cli ping

# 4. 查看详细日志
./server 2>&1 | head -50
```

**常见原因**：
- `configs/config.yaml` 不存在或权限不足
- `MYSQL_DSN` 为空或格式错误
- `CRYPTO_KEY` 为空或长度不对
- MySQL / Redis 未启动或网络不通

### 10.2 HTTP 请求无响应

**排查步骤**：

```bash
# 1. 检查端口监听
ss -tlnp | grep 8080

# 2. 健康检查
curl -v http://localhost:8080/healthz

# 3. 查看日志
tail -f logs/server.log

# 4. 检查防火墙
sudo iptables -L -n | grep 8080
```

### 10.3 Worker 不消费任务

**排查步骤**：

```bash
# 1. 检查 worker 进程是否存在
ps aux | grep worker

# 2. 检查 Redis 队列
redis-cli LLEN asynq:{default}
redis-cli LLEN asynq:{critical}

# 3. 检查 Asynq 监控面板（如已部署）
# asynqmon 可查看队列状态

# 4. 查看 worker 日志
tail -f logs/worker.log
```

**常见原因**：
- worker 未启动或崩溃
- Redis DB 与 server 不一致
- 任务 handler 未注册（panic 导致 worker 退出）

### 10.4 数据库连接报错

```
[mysql] failed to initialize database, got error Error 1045: Access denied
```

**解决**：
- 检查 DSN 用户名/密码
- 确认 MySQL 用户权限：`SHOW GRANTS FOR 'aiscript'@'%'`
- 检查 MySQL `max_connections`：`SHOW VARIABLES LIKE 'max_connections'`

### 10.5 JWT 认证失败

```
invalid token
```

**解决**：
- 确认 `jwt.secret` 在 server 与 worker 中一致
- 检查 token 是否过期
- 确认系统时间同步：`ntpdate -u pool.ntp.org`

### 10.6 文件上传/下载失败

**排查**：
- 检查存储 provider 配置
- 本地存储：确认目录权限 `chmod 755 /opt/ai-script/var/uploads`
- 云存储：确认 AccessKey / SecretKey 有效
- 检查 `public_host` 是否可公网访问

### 10.7 FFmpeg 处理失败

**排查**：
```bash
# 检查 FFmpeg 是否安装
ffmpeg -version

# 检查输入文件
ffprobe -v error /path/to/input.mp4

# 检查磁盘空间
df -h
```

---

## 11. 安全加固建议

### 11.1 JWT Secret

- **必须**在生产环境更换默认密钥
- 建议长度 >= 64 字符，使用 `openssl rand -base64 48` 生成
- 定期轮换（建议每 90 天）

```bash
openssl rand -base64 48
```

### 11.2 Crypto Key

- 使用 `go run ./cmd/genkey` 生成
- **绝对不要**提交到代码仓库
- 通过环境变量或 K8s Secret 注入

### 11.3 RBAC 配置

RBAC 模型文件：`configs/rbac_model.conf`

```
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

- 生产环境应在数据库中预置角色-权限策略
- 定期审计权限分配
- 敏感操作（删除、重置密码）增加二次确认

### 11.4 数据库安全

- 禁止 root 远程登录
- 使用独立业务用户，最小权限原则
- 启用 SSL/TLS 连接
- 定期更新密码

### 11.5 网络安全

- 使用 HTTPS（Nginx / Traefik / Ingress 反向代理）
- 配置 CORS 白名单（生产环境关闭 `AllowAllOrigins`）
- 限制 Redis 访问（绑定 127.0.0.1 或内网 IP）
- 对象存储 bucket 不要设置为 public-read，使用预签名 URL

### 11.6 容器安全

- 使用非 root 用户运行容器
- 定期更新基础镜像（`alpine:3.20`）
- 扫描镜像漏洞：`trivy image ai-script:latest`

---

## 12. 升级指南

### 12.1 升级前准备

1. **阅读 CHANGELOG**：确认是否有破坏性变更
2. **备份数据**：
   ```bash
   mysqldump -u aiscript -p ai_script > /backup/pre-upgrade-$(date +%Y%m%d).sql
   ```
3. **备份配置**：
   ```bash
   cp -r configs /backup/configs-$(date +%Y%m%d)
   ```

### 12.2 二进制升级

```bash
# 1. 拉取新代码
git pull origin main

# 2. 重新编译
make build

# 3. 停止旧服务
sudo systemctl stop ai-script-server ai-script-worker

# 4. 替换二进制
cp out/server /opt/ai-script/server
cp out/worker /opt/ai-script/worker

# 5. 如有配置变更，合并 configs/config.yaml

# 6. 启动新服务
sudo systemctl start ai-script-server ai-script-worker

# 7. 验证
sudo systemctl status ai-script-server
curl http://localhost:8080/healthz
```

### 12.3 Docker 升级

```bash
# 1. 拉取新代码并构建
git pull origin main
docker build -t ai-script:v2.0.0 .

# 2. 停止并移除旧容器
docker compose down

# 3. 更新镜像标签
docker tag ai-script:v2.0.0 ai-script:latest

# 4. 启动新容器
docker compose up -d

# 5. 验证
docker compose ps
docker compose logs -f server
```

### 12.4 K8s 升级

```bash
# 1. 构建并推送新镜像
docker build -t your-registry/ai-script:v2.0.0 .
docker push your-registry/ai-script:v2.0.0

# 2. 滚动更新
kubectl set image deployment/ai-script-server server=your-registry/ai-script:v2.0.0 -n ai-script
kubectl set image deployment/ai-script-worker worker=your-registry/ai-script:v2.0.0 -n ai-script

# 3. 监控滚动状态
kubectl rollout status deployment/ai-script-server -n ai-script
kubectl rollout status deployment/ai-script-worker -n ai-script

# 4. 回滚（如有问题）
kubectl rollout undo deployment/ai-script-server -n ai-script
kubectl rollout undo deployment/ai-script-worker -n ai-script
```

### 12.5 数据库 Schema 升级

如版本包含 schema 变更：

```bash
# 使用 golang-migrate
migrate -path migrations -database "mysql://aiscript:password@tcp(localhost:3306)/ai_script" up

# 或手动执行 SQL（生产环境需 DBA 审核）
mysql -u aiscript -p ai_script < migrations/0002_add_new_column.sql
```

### 12.6 升级后验证清单

- [ ] `/healthz` 返回 `{"status":"ok"}`
- [ ] 登录/获取 Token 正常
- [ ] 文件上传/下载正常
- [ ] 任务提交与执行正常（检查 worker 日志）
- [ ] WebSocket 进度推送正常
- [ ] 数据库连接无报错
- [ ] 日志无 ERROR 级别输出

---

## 附录

### A. Makefile 命令速查

| 命令 | 说明 |
|------|------|
| `make help` | 查看帮助 |
| `make tidy` | 整理 Go 模块依赖 |
| `make run` | 本地启动 server |
| `make worker` | 本地启动 worker |
| `make build` | 构建二进制到 `out/` |
| `make test` | 运行单元测试 |
| `make lint` | 静态代码检查 |
| `make docker` | 构建 Docker 镜像 |

### B. 常用环境变量速查

```bash
export APP_ENV=prod
export APP_PORT=8080
export APP_LOG_LEVEL=info
export JWT_SECRET="your-jwt-secret"
export JWT_EXPIRES_IN=7200
export JWT_REFRESH_EXPIRES_IN=604800
export MYSQL_DSN="user:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=True&loc=UTC"
export MYSQL_MAX_IDLE=10
export MYSQL_MAX_OPEN=100
export REDIS_ADDR="127.0.0.1:6379"
export REDIS_PASSWORD=""
export REDIS_DB=0
export OSS_PROVIDER=local
export OSS_BUCKET=./var/uploads
export OSS_PUBLIC_HOST=/uploads
export CRYPTO_KEY="your-crypto-key"
export MODEL_GATEWAY_URL=""
export MODEL_GATEWAY_KEY=""
```

### C. 相关文档

- `docs/technical-design.md` §6-7：数据库设计与技术架构
- `configs/rbac_model.conf`：RBAC 权限模型定义
- `.env.example`：环境变量示例

---

> 本文档由 DevOps 团队维护，如有问题请联系运维人员。
