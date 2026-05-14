# AI-Script 部署与运维指南

> 版本:v2.0(2026-05-13 重组)
> 适用版本:Go 1.24 + AI-Script Backend
> 本文档按**部署方式**分组组织,选定一种方式后只看对应章节即可。

---

## 目录

- [0. 概览](#0-概览)
  - [0.1 系统架构](#01-系统架构)
  - [0.2 四种部署方式对比](#02-四种部署方式对比)
  - [0.3 部署前置清单](#03-部署前置清单)
- [1. 通用前置(所有部署方式必读)](#1-通用前置所有部署方式必读)
  - [1.1 系统需求](#11-系统需求)
  - [1.2 关键密钥与凭据](#12-关键密钥与凭据)
  - [1.3 配置与环境变量映射](#13-配置与环境变量映射)
  - [1.4 .env.example 三个文件指引](#14-envexample-三个文件指引)
  - [1.5 数据库初始化机制](#15-数据库初始化机制)
- [2. 部署方式 A:Docker Compose(推荐)](#2-部署方式-adocker-compose推荐)
- [3. 部署方式 B:Linux 二进制 + systemd](#3-部署方式-blinux-二进制--systemd)
- [4. 部署方式 C:Windows 二进制 + NSSM](#4-部署方式-cwindows-二进制--nssm)
- [5. 部署方式 D:Kubernetes](#5-部署方式-dkubernetes)
- [6. CI/CD 流水线(GitHub Actions)](#6-cicd-流水线github-actions)
- [7. 部署后验收](#7-部署后验收)
- [8. 运维通用项](#8-运维通用项)
- [9. FAQ — 常见问题速答](#9-faq--常见问题速答)
- [附录](#附录)

---

## 0. 概览

### 0.1 系统架构

AI-Script 采用**双进程架构**:

- **server**:HTTP API,基于 Gin 框架,对外提供 RESTful API + WebSocket 进度推送
- **worker**:异步任务消费者,基于 Asynq 从 Redis 拉取任务,执行 LLM 调用、媒体合成

共享依赖:**MySQL 8.0**(业务数据) + **Redis 7**(缓存/队列/Pub-Sub) + **对象存储**(本地/MinIO/OSS/S3/COS)

```
+--------+  HTTP  +--------+         +-------+
| Client | -----> | server | <-----> | MySQL |
+--------+        +--------+         +-------+
                      |  WS                |
                      v                    |
                  +-------+                |
                  | Redis | <----+         |
                  +-------+      |    +---------+
                      ^          +----| Storage |
                      |  Asynq        +---------+
                      |  Queue
                  +--------+
                  | worker |
                  +--------+
```

### 0.2 四种部署方式对比

| 部署方式 | 适用场景 | 单机/集群 | 操作系统 | 复杂度 | 推荐度 |
|---------|---------|----------|---------|--------|--------|
| **A. Docker Compose** | 开发 / 中小型生产 | 单机 | Linux/macOS/Windows | ⭐ | ⭐⭐⭐⭐⭐ |
| **B. Linux 二进制 + systemd** | 性能敏感的生产 / 已有 MySQL/Redis | 单机 | Linux | ⭐⭐ | ⭐⭐⭐⭐ |
| **C. Windows 二进制 + NSSM** | Windows Server / 内网部署 | 单机 | Windows | ⭐⭐ | ⭐⭐⭐ |
| **D. Kubernetes** | 高可用 / 弹性扩缩容 | 集群 | Linux | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |

> **怎么选?**
> - 第一次部署 / 不确定 → **A. Docker Compose**
> - 已有 MySQL/Redis,要榨干性能 → **B. Linux 二进制**
> - 客户内网只能用 Windows Server → **C. Windows 二进制**
> - 业务量大、要高可用、要灰度 → **D. Kubernetes**

### 0.3 部署前置清单

无论选哪种方式,部署前请确认:

- [ ] 服务器满足硬件需求(见 §1.1)
- [ ] 出站网络可访问 AI 模型网关(OpenAI / LiteLLM / 阿里云百炼等)
- [ ] 已准备好 **JWT_SECRET**(>= 32 字符随机)
- [ ] 已准备好 **CRYPTO_KEY**(AES-256,32 字节)— 用 `make` 或 `go run ./cmd/genkey` 生成
- [ ] 已准备好**对象存储**凭据(或决定先用本地存储/MinIO)
- [ ] 端口未冲突:**80**(前端)、**8080**(后端)、**3306**(MySQL)、**6379**(Redis)、**9000/9001**(MinIO,可选)

---

## 1. 通用前置(所有部署方式必读)

### 1.1 系统需求

#### 硬件

| 规模 | CPU | 内存 | 磁盘 | 网络 |
|------|-----|------|------|------|
| 开发/测试 | 2 核 | 4 GB | 50 GB SSD | 10 Mbps |
| 生产(单节点) | 4 核 | 8 GB | 200 GB SSD | 50 Mbps |
| 生产(高可用) | 8 核+ | 16 GB+ | 500 GB SSD | 100 Mbps+ |

> FFmpeg 媒体合成对 CPU 与磁盘 I/O 敏感,生产环境建议预留 2 倍余量。

#### 软件版本

| 组件 | 最低版本 | 用于哪种部署方式 |
|------|---------|----------------|
| **Go** | **1.24**(`go.mod` 锁定) | B/C(二进制部署需本机编译);A 用容器镜像,D 用 CI 产物 |
| MySQL | 5.7+(推荐 8.0) | 全部 |
| Redis | 6.0+(推荐 7.x) | 全部 |
| FFmpeg | 5.0+ | B/C(容器镜像已内置) |
| Docker | 20.10+ | A/D(K8s 集群节点) |
| Docker Compose | 2.0+(`docker compose` 命令) | A |

> ⚠️ **Go 版本一致性提示**:仓库内 `go.mod` 锁定 **1.24**,本机编译需要 Go 1.24+。镜像构建时若失败,确认 `backend/Dockerfile` 的 `FROM` 已升到 `golang:1.24-alpine` 即可。

#### 网络端口

| 端口 | 服务 | 是否对外 |
|------|------|---------|
| 80 | frontend (nginx) | 是 |
| 8080 | backend (server) | 视部署而定 |
| 3306 | MySQL | 仅内网 |
| 6379 | Redis | 仅内网 |
| 9000 | MinIO API | 仅内网 |
| 9001 | MinIO Console | 仅内网/可选对外 |

### 1.2 关键密钥与凭据

#### JWT_SECRET

```bash
# 生成 48 字节 base64(>= 32 字符)
openssl rand -base64 48
```

- **生产环境必须更换**默认值,长度建议 >= 64 字符
- 建议每 90 天轮换
- `server` 与 `worker` 必须使用**同一个**值

#### CRYPTO_KEY / CRYPTO_KEY_BASE64

模型 `api_key` 入库前用 **AES-256-GCM** 加密,需要一个 32 字节密钥。**两种写法二选一**:

```bash
# 写法 A:32 字节明文(适用 CRYPTO_KEY)
openssl rand -base64 32 | head -c 32

# 写法 B:base64 编码的 32 字节(适用 CRYPTO_KEY_BASE64)
openssl rand -base64 32

# 或用项目自带工具(如已实现)
cd backend && go run ./cmd/genkey
```

> **优先级**:当两者都设置时,通常 `CRYPTO_KEY` 优先。`deploy/docker-compose.yml`(生产 compose)使用 `CRYPTO_KEY_BASE64`,根 `docker-compose.yml`(开发 compose)使用 `CRYPTO_KEY` — 别搞混。

#### 默认管理员账号

首次启动后,种子数据会创建管理员账号:

| 字段 | 值 | 来源 |
|------|---|------|
| 用户名 | `admin` | `backend/internal/repo/seed.go` |
| 密码 | `admin123` | 同上(bcrypt) |
| 邮箱 | `admin@example.com` | 同上 |

> ⚠️ **首次登录后立即修改密码**。`README.md` 中曾出现 `admin@123` 的写法是历史误写,**以代码为准**。

### 1.3 配置与环境变量映射

配置加载顺序:

1. 读取 `backend/configs/config.yaml`(或 `CONFIG_FILE` 指定的文件)
2. 环境变量覆盖同名配置(下划线大写命名,如 `APP_ENV` 覆盖 `app.env`)
3. 支持 `${VAR:default}` 默认值语法

完整环境变量映射见 [附录 B](#b-环境变量速查表)。

### 1.4 .env.example 三个文件指引

仓库内有 **3 个** `.env.example`,**用途完全不同**,这是历史散乱点之一,务必看清:

| 文件 | 用途 | 配合的 compose | 关键差异 |
|------|------|---------------|---------|
| `/.env.example`(根目录) | 开发 / 本地 docker compose | `/docker-compose.yml` | 使用 `MYSQL_ROOT_PWD` / `CRYPTO_KEY` |
| `/deploy/.env.example` | 生产 / 容器化部署 | `/deploy/docker-compose.yml` | 使用 `MYSQL_ROOT_PASSWORD` / `CRYPTO_KEY_BASE64` |
| `/backend/.env.example` | 后端独立运行(B/C 部署方式) | 无 compose,供 systemd/NSSM 直接读取 | 简化版,只含 backend 必需变量 |

> **何时用哪个?**
> - 跑根目录 compose → `cp .env.example .env`
> - 跑 deploy/ compose → `cd deploy && cp .env.example .env`
> - Linux/Windows 二进制 → 用 `backend/.env.example` 作为参考,导出为环境变量或写入 systemd unit / NSSM 服务

> ⚠️ **变量名差异**:根 compose 用 `MYSQL_ROOT_PWD`,deploy compose 用 `MYSQL_ROOT_PASSWORD`。**两个文件不能互换 .env**。

### 1.5 数据库初始化机制

#### MVP 阶段:GORM AutoMigrate

`server` / `worker` 启动时自动执行 `GORM AutoMigrate`,**只增不删**:

涉及的表(节选):`users` / `departments` / `roles` / `projects` / `models` / `scripts` / `episodes` / `storyboards` / `pipelines` / `pipeline_runs` / `step_runs` / `review_flows` / `publishes` / `audit_logs` / `billing_quotas` 等。

#### 种子数据

通过 SQL 脚本或代码 seed 注入:

| 数据源 | 内容 | 触发时机 |
|--------|------|---------|
| `scripts/sql/001_init.sql` | DDL(可选,与 AutoMigrate 二选一) | docker compose 首次启动,`mysql:8.0` 的 `/docker-entrypoint-initdb.d/` 自动执行 |
| `scripts/sql/002_seed.sql` | 部门 / 权限点等业务数据 | 同上 |
| `backend/internal/repo/seed.go` | 管理员 `admin/admin123` / 默认角色 / 配额 | server 启动时代码注入 |

#### 生产环境建议

生产环境长期方案应改用 **golang-migrate** 或 **Atlas** 管理 schema 变更,见 §8.5。

---

## 2. 部署方式 A:Docker Compose(推荐)[docker-compose.yml](../docker-compose.yml)

> ⭐ 90% 的场景适用,适合开发、测试、小型生产。

### 2.1 选哪个 compose 文件

仓库内有**两个** docker-compose.yml,用途不同:

| 文件 | 用途 | 工作目录 | 配套 .env |
|------|------|---------|----------|
| `/docker-compose.yml`(根) | **开发 / 本地**,带 build 配置,变量名 `MYSQL_ROOT_PWD` | 仓库根目录 | `/.env` |
| `/deploy/docker-compose.yml` | **生产**,使用 `CRYPTO_KEY_BASE64`,`mysql:3306` 端口仅绑 127.0.0.1 | `/deploy` 目录 | `/deploy/.env` |

**选错文件最常见的失败:** "Did not find expected key" / "MYSQL_DSN is required" — 因为 .env 变量名与 compose 引用的不匹配。

### 2.2 开发环境:根目录 compose

```bash
# 1. 准备 .env(根目录)
cp .env.example .env
# 编辑:把 JWT_SECRET 和 CRYPTO_KEY 改成 32 字节随机串

# 2. 一键拉起全部服务
docker compose up -d

# 3. 查看状态 / 日志
docker compose ps
docker compose logs -f server
docker compose logs -f worker

# 4. 停止 / 清理
docker compose down          # 保留数据卷
docker compose down -v       # 连数据卷一起清除(慎用)
```

启动后:
- 前端:http://localhost/
- 后端 API:http://localhost:8080
- MinIO Console:http://localhost:9001(账号见 .env 中 `MINIO_ROOT_USER`)
- 默认登录:`admin` / `admin123`

### 2.3 生产环境:deploy/ compose

```bash
# 1. 准备 .env(deploy 目录)
cd deploy
cp .env.example .env
# 编辑:
#   MYSQL_ROOT_PASSWORD / MYSQL_PASSWORD(强密码)
#   JWT_SECRET(>= 32 字符随机)
#   CRYPTO_KEY_BASE64(base64 编码的 32 字节)
#   MINIO_ROOT_USER / MINIO_ROOT_PASSWORD
#   MODEL_GATEWAY_URL / MODEL_GATEWAY_KEY(指向 LiteLLM/OneAPI)

# 2. 启动
docker compose up -d --build

# 3. 健康检查
curl http://localhost:8080/healthz/live
curl http://localhost:8080/healthz/ready
```

#### 与开发版的关键差异

| 项 | 根 compose | deploy compose |
|----|-----------|---------------|
| 服务名 | `server` / `worker` / `frontend` | `backend` / `worker` / `frontend` |
| MySQL 端口绑定 | `127.0.0.1:${MYSQL_PORT:-3306}` | `127.0.0.1:3306`(固定) |
| 加密变量 | `CRYPTO_KEY`(明文 32 字节) | `CRYPTO_KEY_BASE64`(base64) |
| MYSQL_DSN | .env 单独提供 | compose 内部由 `MYSQL_USER`/`MYSQL_PASSWORD`/`MYSQL_DATABASE` 拼装 |
| `APP_ENV` 默认 | `local` | `prod` |
| 网络拓扑 | 单网 `ai-script-net` | 分网 `db-net` + `app-net`(安全隔离) |

### 2.4 单容器部署(已有外部 MySQL/Redis)

如果生产环境已经有现成的 MySQL / Redis,可以只跑 backend/worker 容器:

```bash
# 构建镜像
cd backend
docker build -t ai-script-backend:latest .

# 运行 server
docker run -d \
  --name ai-script-server \
  -p 8080:8080 \
  -e APP_ENV=prod \
  -e MYSQL_DSN="aiscript:password@tcp(host.docker.internal:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=Local" \
  -e REDIS_ADDR="host.docker.internal:6379" \
  -e JWT_SECRET="your-jwt-secret" \
  -e CRYPTO_KEY_BASE64="your-base64-key" \
  ai-script-backend:latest

# 运行 worker(同镜像,不同 entrypoint)
docker run -d \
  --name ai-script-worker \
  -e APP_ENV=prod \
  -e MYSQL_DSN="..." \
  -e REDIS_ADDR="host.docker.internal:6379" \
  -e JWT_SECRET="..." \
  -e CRYPTO_KEY_BASE64="..." \
  --entrypoint /app/worker \
  ai-script-backend:latest
```

### 2.5 升级(Docker Compose)

```bash
# 拉新代码
git pull origin main

# 重建镜像并滚动重启
docker compose up -d --build

# 或分步:
docker compose build
docker compose up -d --no-deps server worker  # 不重启 MySQL/Redis

# 验证
docker compose ps
docker compose logs -f server
curl http://localhost:8080/healthz/ready
```

### 2.6 Docker 方式常见问题

| 问题 | 速答 |
|------|------|
| `MYSQL_DSN is required` 启动失败 | .env 缺变量;检查是否用错 compose 配套的 .env(见 §1.4) |
| 容器 `unhealthy` 一直起不来 | `backend/Dockerfile` 的 HEALTHCHECK 当前 endpoint 是 `/healthz`,实际应是 `/healthz/live`(详见 §9.2 第 6 条) |
| `mysql_native_password` 报错 | 根 compose 已设 `--default-authentication-plugin=mysql_native_password`;deploy compose 没设,如需对接老客户端工具请加上 |
| 删完容器数据还在 | 数据卷未清,`docker compose down -v` 才会清 |
| 改了 .env 不生效 | 必须 `docker compose down && up -d`,只 restart 不重新读取 env_file |

---

## 3. 部署方式 B:Linux 二进制 + systemd

> 适合对性能、资源、安全有精确控制的生产环境;已有 MySQL/Redis 时尤其推荐。

### 3.1 依赖安装

#### MySQL 5.7+ / 8.0(Ubuntu/Debian)

```bash
sudo apt update
sudo apt install -y mysql-server-8.0
sudo mysql_secure_installation

sudo mysql -u root -p <<'SQL'
CREATE DATABASE ai_script CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'aiscript'@'%' IDENTIFIED BY 'YourStrongPassword123!';
GRANT ALL PRIVILEGES ON ai_script.* TO 'aiscript'@'%';
FLUSH PRIVILEGES;
SQL
```

#### Redis 6.0+

```bash
sudo apt install -y redis-server
sudo systemctl enable --now redis-server

# 设置密码(/etc/redis/redis.conf):
#   requirepass YourRedisPassword
sudo systemctl restart redis-server
```

#### FFmpeg 5.0+

```bash
sudo apt install -y ffmpeg
ffmpeg -version   # 应 >= 5.0
```

### 3.2 编译二进制

```bash
cd backend

# 国内加速
export GOPROXY=https://goproxy.cn,direct
go mod download

# 构建(根 Makefile 与 backend/Makefile 都能用)
make build
# 产物:
#   - backend/Makefile 用法 → backend/out/server, backend/out/worker
#   - 根 Makefile 用法    → backend/bin/server, backend/bin/worker
```

> ⚠️ **两个 Makefile 的差异**:根 `Makefile` 输出到 `backend/bin/`,`backend/Makefile` 输出到 `backend/out/`。本指南以 `backend/out/` 为准。

### 3.3 目录规划

```
/opt/ai-script/
├── server                  # 二进制
├── worker                  # 二进制
├── configs/
│   ├── config.yaml         # 主配置
│   └── rbac_model.conf     # Casbin 模型
├── var/uploads/            # 本地存储(若用 OSS 可不要)
└── logs/                   # 日志输出
```

```bash
sudo useradd -r -s /bin/false ai-script
sudo mkdir -p /opt/ai-script/{configs,var/uploads,logs} /var/log/ai-script
sudo cp backend/out/server backend/out/worker /opt/ai-script/
sudo cp backend/configs/config.yaml /opt/ai-script/configs/
sudo chown -R ai-script:ai-script /opt/ai-script /var/log/ai-script
```

### 3.4 systemd 服务单元

`/etc/systemd/system/ai-script-server.service`:

```ini
[Unit]
Description=AI-Script Server
After=network.target mysql.service redis-server.service
Wants=mysql.service redis-server.service

[Service]
Type=simple
User=ai-script
Group=ai-script
WorkingDirectory=/opt/ai-script
EnvironmentFile=/opt/ai-script/.env
ExecStart=/opt/ai-script/server
ExecStop=/bin/kill -SIGTERM $MAINPID
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/ai-script/server.log
StandardError=append:/var/log/ai-script/server.log

[Install]
WantedBy=multi-user.target
```

`/etc/systemd/system/ai-script-worker.service`:

```ini
[Unit]
Description=AI-Script Worker
After=network.target mysql.service redis-server.service
Wants=mysql.service redis-server.service

[Service]
Type=simple
User=ai-script
Group=ai-script
WorkingDirectory=/opt/ai-script
EnvironmentFile=/opt/ai-script/.env
ExecStart=/opt/ai-script/worker
ExecStop=/bin/kill -SIGTERM $MAINPID
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/ai-script/worker.log
StandardError=append:/var/log/ai-script/worker.log

[Install]
WantedBy=multi-user.target
```

`/opt/ai-script/.env`(参考 `backend/.env.example`):

```bash
APP_ENV=prod
APP_PORT=8080
APP_LOG_LEVEL=info
GIN_MODE=release
MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=Local
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=YourRedisPassword
JWT_SECRET=your-64-char-random-jwt-secret
CRYPTO_KEY=your-32-byte-crypto-key
OSS_PROVIDER=local
OSS_BUCKET=/opt/ai-script/var/uploads
OSS_PUBLIC_HOST=/uploads
MODEL_GATEWAY_URL=https://your-litellm.example.com/v1
MODEL_GATEWAY_KEY=sk-your-gateway-key
```

```bash
sudo chmod 600 /opt/ai-script/.env
sudo chown ai-script:ai-script /opt/ai-script/.env
```

### 3.5 启用 / 启动 / 查日志

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ai-script-server ai-script-worker

# 状态
sudo systemctl status ai-script-server
sudo systemctl status ai-script-worker

# 日志(实时)
sudo journalctl -u ai-script-server -f
sudo journalctl -u ai-script-worker -f
# 或
tail -f /var/log/ai-script/server.log /var/log/ai-script/worker.log

# 重启 / 优雅停止
sudo systemctl restart ai-script-server
sudo systemctl stop    ai-script-server
```

### 3.6 优雅停止说明

server / worker 都监听 `SIGINT` / `SIGTERM`:
- **server**:停止接受新请求,10 秒内完成在处理的 HTTP 请求后退出
- **worker**:停止消费新任务,等待当前任务完成后退出

### 3.7 升级(二进制)

```bash
# 1. 拉新代码 + 编译
git pull origin main
cd backend && make build

# 2. 备份当前二进制(可回滚)
sudo cp /opt/ai-script/server  /opt/ai-script/server.bak
sudo cp /opt/ai-script/worker  /opt/ai-script/worker.bak

# 3. 替换 + 重启
sudo systemctl stop  ai-script-server ai-script-worker
sudo cp out/server out/worker /opt/ai-script/
sudo systemctl start ai-script-server ai-script-worker

# 4. 验证
curl http://localhost:8080/healthz/ready
sudo systemctl status ai-script-server

# 5. 失败回滚
sudo systemctl stop ai-script-server ai-script-worker
sudo cp /opt/ai-script/server.bak /opt/ai-script/server
sudo cp /opt/ai-script/worker.bak /opt/ai-script/worker
sudo systemctl start ai-script-server ai-script-worker
```

### 3.8 Linux 二进制常见问题

| 问题 | 速答 |
|------|------|
| `systemctl start` 后立即 failed | `journalctl -u ai-script-server -n 100` 看真实报错;通常是 .env 缺变量或 MySQL/Redis 未启动 |
| `EnvironmentFile` 没生效 | 文件权限不是 600 / 不是 ai-script 用户可读;`sudo -u ai-script cat /opt/ai-script/.env` 验证 |
| Bind: address already in use | 8080 被占用,`sudo ss -tlnp \| grep 8080` 找出占用进程 |

---

## 4. 部署方式 C:Windows 二进制 + NSSM

> 适用 Windows Server 部署或客户内网只能用 Windows 的场景。

### 4.1 依赖安装

#### MySQL 8.0

1. 下载 MySQL Installer:https://dev.mysql.com/downloads/installer/
2. 选择 **Server only** 安装类型
3. 记住 root 密码,启用 MySQL 服务(默认开机自启)
4. 在 MySQL Workbench / Command Line Client 中执行:

```sql
CREATE DATABASE ai_script CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'aiscript'@'%' IDENTIFIED BY 'YourStrongPassword123!';
GRANT ALL PRIVILEGES ON ai_script.* TO 'aiscript'@'%';
FLUSH PRIVILEGES;
```

#### Redis 7+

1. 下载 Redis for Windows(微软维护版或 [Memurai](https://www.memurai.com/))
2. 解压到 `C:\Redis`,加入 PATH
3. 注册为 Windows 服务(以管理员身份运行 PowerShell):

```powershell
cd C:\Redis
redis-server --service-install redis.windows.conf --loglevel verbose
redis-server --service-start
```

#### FFmpeg

1. 下载:https://www.gyan.dev/ffmpeg/builds/ (选 `ffmpeg-release-essentials.7z`)
2. 解压到 `C:\ffmpeg`,将 `C:\ffmpeg\bin` 加入系统 PATH
3. 验证:`ffmpeg -version`

### 4.2 编译 Windows 二进制

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go mod download

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o out/server.exe ./cmd/server
go build -o out/worker.exe ./cmd/worker
```

### 4.3 目录规划

```
C:\ai-script\
├── server.exe
├── worker.exe
├── configs\
│   ├── config.yaml
│   └── rbac_model.conf
├── var\uploads\         # 本地存储
└── logs\
```

### 4.4 NSSM 注册为 Windows Service

#### 下载 NSSM

```powershell
Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile "nssm.zip"
Expand-Archive -Path "nssm.zip" -DestinationPath "C:\nssm"
# 将 C:\nssm\win64 加入系统 PATH
```

#### 注册 Server 服务(以管理员身份运行 PowerShell)

```powershell
nssm install ai-script-server
# 在 GUI 中设置:
#   Path:              C:\ai-script\server.exe
#   Startup directory: C:\ai-script

# 环境变量
nssm set ai-script-server AppEnvironmentExtra `
  "APP_ENV=prod" `
  "APP_PORT=8080" `
  "APP_LOG_LEVEL=info" `
  "GIN_MODE=release" `
  "MYSQL_DSN=aiscript:password@tcp(127.0.0.1:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=Local" `
  "REDIS_ADDR=127.0.0.1:6379" `
  "REDIS_PASSWORD=YourRedisPassword" `
  "JWT_SECRET=your-64-char-random-jwt-secret" `
  "CRYPTO_KEY=your-32-byte-crypto-key" `
  "OSS_PROVIDER=local" `
  "OSS_BUCKET=C:\ai-script\var\uploads" `
  "OSS_PUBLIC_HOST=/uploads"

# 日志(NSSM 自动滚动)
nssm set ai-script-server AppStdout C:\ai-script\logs\server.log
nssm set ai-script-server AppStderr C:\ai-script\logs\server.log
nssm set ai-script-server AppRotateFiles 1
nssm set ai-script-server AppRotateBytes 10485760

nssm start ai-script-server
```

#### 注册 Worker 服务

```powershell
nssm install ai-script-worker
# Path: C:\ai-script\worker.exe
# Startup directory: C:\ai-script

nssm set ai-script-worker AppEnvironmentExtra `
  "APP_ENV=prod" `
  "MYSQL_DSN=..." `
  "REDIS_ADDR=127.0.0.1:6379" `
  "REDIS_PASSWORD=YourRedisPassword" `
  "JWT_SECRET=your-64-char-random-jwt-secret" `
  "CRYPTO_KEY=your-32-byte-crypto-key" `
  "OSS_PROVIDER=local" `
  "OSS_BUCKET=C:\ai-script\var\uploads"

nssm set ai-script-worker AppStdout C:\ai-script\logs\worker.log
nssm set ai-script-worker AppStderr C:\ai-script\logs\worker.log
nssm start ai-script-worker
```

### 4.5 服务管理

```powershell
# 状态 / 启停 / 重启
nssm status ai-script-server
nssm stop / nssm start / nssm restart ai-script-server
nssm remove ai-script-server confirm   # 卸载

# 或用 PowerShell 标准服务命令
Get-Service ai-script-server, ai-script-worker
Restart-Service ai-script-server
```

### 4.6 升级(Windows)

```powershell
# 停止服务
nssm stop ai-script-server
nssm stop ai-script-worker

# 备份 + 替换
Copy-Item C:\ai-script\server.exe C:\ai-script\server.exe.bak
Copy-Item C:\ai-script\worker.exe C:\ai-script\worker.exe.bak
Copy-Item .\out\server.exe C:\ai-script\
Copy-Item .\out\worker.exe C:\ai-script\

# 启动 + 验证
nssm start ai-script-server
nssm start ai-script-worker
Invoke-WebRequest http://localhost:8080/healthz/ready
```

### 4.7 Windows 常见问题

| 问题 | 速答 |
|------|------|
| `nssm install` 弹不出 GUI | 必须以**管理员身份**运行 PowerShell |
| 服务起来后立即停止 | 看 `C:\ai-script\logs\server.log`,通常是环境变量没设全 |
| 中文乱码 | 设置环境变量 `LANG=zh_CN.UTF-8`,或代码层使用 UTC 时区 |
| FFmpeg not found | `ffmpeg.exe` 没加入 PATH,或服务运行账户没有 PATH 访问权限 |

---

## 5. 部署方式 D:Kubernetes

> 适合需要高可用、自动扩缩容、灰度发布的生产环境。

### 5.1 现成 K8s 资源清单

仓库已提供完整的 K8s manifests,**直接使用而不要重新拼写**:

```
deploy/k8s/
├── namespace.yaml              # Namespace: ai-script
├── configmap.yaml              # 非敏感配置(APP_ENV / MYSQL_MAX_OPEN / OSS_REGION ...)
├── secret.yaml                 # 模板 — 部署前必须替换占位符
├── backend-deployment.yaml     # backend Deployment + Service(replicas: 2,RollingUpdate)
├── worker-deployment.yaml      # worker Deployment
├── frontend-deployment.yaml    # frontend Deployment + Service
└── ingress.yaml                # nginx Ingress + cert-manager TLS
```

特性已内置:`runAsNonRoot: true` / `readOnlyRootFilesystem` / `capabilities.drop: ALL` / `podAntiAffinity` / `readinessProbe` / `livenessProbe`。

### 5.2 镜像构建与推送

```bash
# 本地构建
cd backend
docker build -t registry.example.com/ai-script/backend:v1.0.0 .
docker push registry.example.com/ai-script/backend:v1.0.0

cd ../frontend
docker build -t registry.example.com/ai-script/frontend:v1.0.0 .
docker push registry.example.com/ai-script/frontend:v1.0.0
```

> 也可以由 [§6 CI/CD](#6-cicd-流水线github-actions) 自动构建并推送到 `ghcr.io`。

修改 `deploy/k8s/backend-deployment.yaml` / `frontend-deployment.yaml` 中的 `image:` 字段,指向你的镜像 registry。

### 5.3 准备 ConfigMap / Secret

#### ConfigMap(非敏感)

`deploy/k8s/configmap.yaml` 已经写好默认值,根据生产环境调整:

```yaml
data:
  APP_ENV: "production"
  APP_LOG_LEVEL: "info"
  MYSQL_MAX_OPEN: "100"
  OSS_PROVIDER: "oss"
  OSS_REGION: "cn-hangzhou"
  OSS_BUCKET: "ai-script-prod"
  # ...其余见文件
```

#### Secret(敏感)

`deploy/k8s/secret.yaml` 是**模板**,所有值是 `BASE64_ENCODED_xxx` 占位符,**必须替换**:

```bash
# 替换方式 A:命令行生成(推荐)
kubectl create secret generic ai-script-secret -n ai-script \
  --from-literal=MYSQL_DSN="aiscript:password@tcp(mysql:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=Local" \
  --from-literal=REDIS_PASSWORD="YourRedisPwd" \
  --from-literal=JWT_SECRET="$(openssl rand -base64 48)" \
  --from-literal=CRYPTO_KEY="$(openssl rand -base64 32 | head -c 32)" \
  --from-literal=OSS_ENDPOINT="oss-cn-hangzhou.aliyuncs.com" \
  --from-literal=OSS_ACCESS_KEY="your-oss-ak" \
  --from-literal=OSS_SECRET_KEY="your-oss-sk" \
  --from-literal=MODEL_GATEWAY_URL="https://your-litellm.example.com/v1" \
  --from-literal=MODEL_GATEWAY_KEY="sk-your-gateway-key" \
  --from-literal=MIGRATE_DSN="aiscript:password@tcp(mysql:3306)/ai_script" \
  --from-literal=MIGRATE_SOURCE="file:///app/migrations"

# 替换方式 B:在 secret.yaml 中手动 base64 编码后填入
echo -n 'your-value' | base64
```

### 5.4 数据存储

K8s 默认不带 MySQL/Redis。生产建议:

| 组件 | 推荐做法 |
|------|---------|
| MySQL | 使用云厂商托管(RDS / PolarDB / Aurora)而非 in-cluster |
| Redis | 同上,使用云托管 Redis 集群 |
| 对象存储 | 阿里云 OSS / AWS S3 / 腾讯云 COS,通过 OSS_* secret 注入 |

如必须 in-cluster,使用 StatefulSet + PersistentVolumeClaim,不在本指南范围。

### 5.5 部署命令

```bash
# 按顺序 apply
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/configmap.yaml
# secret 用命令行已经创建则跳过 secret.yaml
kubectl apply -f deploy/k8s/backend-deployment.yaml
kubectl apply -f deploy/k8s/worker-deployment.yaml
kubectl apply -f deploy/k8s/frontend-deployment.yaml
kubectl apply -f deploy/k8s/ingress.yaml

# 查看状态
kubectl get pods -n ai-script
kubectl get svc,ing -n ai-script

# 查看日志
kubectl logs -f deploy/backend -n ai-script
kubectl logs -f deploy/worker  -n ai-script
```

### 5.6 滚动升级与回滚

```bash
# 滚动升级
kubectl set image -n ai-script deploy/backend backend=registry.example.com/ai-script/backend:v1.1.0
kubectl set image -n ai-script deploy/worker  worker=registry.example.com/ai-script/backend:v1.1.0

# 监控滚动状态
kubectl rollout status -n ai-script deploy/backend
kubectl rollout status -n ai-script deploy/worker

# 回滚到上一版
kubectl rollout undo -n ai-script deploy/backend
kubectl rollout undo -n ai-script deploy/worker

# 回滚到指定 revision
kubectl rollout history -n ai-script deploy/backend
kubectl rollout undo -n ai-script deploy/backend --to-revision=2
```

### 5.7 K8s 常见问题

| 问题 | 速答 |
|------|------|
| Pod `CrashLoopBackOff` | `kubectl describe pod -n ai-script <pod>` + `kubectl logs --previous`;通常是 Secret 占位符没替换 |
| `ImagePullBackOff` | 私有 registry 缺 imagePullSecret;`kubectl create secret docker-registry regcred ...` 后在 spec 中引用 |
| Ingress 不通 | 集群没装 ingress-nginx 控制器,或域名没指到 LB;`kubectl get ingress -n ai-script` 看 ADDRESS 列 |
| TLS 证书没签发 | cert-manager 未装,或 ClusterIssuer 名字不是 `letsencrypt-prod`,修改 `ingress.yaml` 注解 |
| `secret.yaml` 模板原样 apply 失败 | `data:` 字段值不是合法 base64;用命令行 `kubectl create secret generic ...` 跳过模板 |

---

## 6. CI/CD 流水线(GitHub Actions)

### 6.1 流水线总览

文件:`.github/workflows/ci.yml`(后端)。触发条件:

- `push` 到 `main` / `master` / `develop`,且 `backend/**` 或 `.github/workflows/**` 有改动
- 同样路径的 PR

| Job | 内容 | 是否阻塞合入 |
|-----|------|------------|
| **lint** | `golangci-lint` 静态检查(配置:`backend/.golangci.yml`) | ✅ |
| **test** | 起 MySQL + Redis service,跑 `go test ./... -race -coverprofile=coverage.out`,上报 Codecov | ✅ |
| **security** | `gosec` 安全扫描,产物上传到 GitHub Security 标签 | ❌(`-no-fail`) |
| **build** | 构建 `server` + `worker` 二进制,产物 `backend/out/` 上传 artifact | ✅(依赖 lint/test) |
| **docker** | `docker buildx` 构建多架构镜像(amd64 + arm64),推送到 `ghcr.io/<owner>/<repo>/backend` | 仅 `push` 触发 |

### 6.2 镜像 tag 规则

镜像名:`ghcr.io/<owner>/<repo>/backend`

| Tag | 含义 |
|-----|------|
| `<branch>` | 分支推送时打 |
| `<branch>-<sha>` | 提交 SHA(short) |
| `latest` | 仅 default branch(main) |

### 6.3 集成到部署

```bash
# K8s 部署使用 CI 镜像
kubectl set image -n ai-script deploy/backend \
  backend=ghcr.io/<owner>/<repo>/backend:main-abc1234

# Docker Compose 用 CI 镜像(替代本地 build)
# 修改 deploy/docker-compose.yml:
#   image: ghcr.io/<owner>/<repo>/backend:latest
# 并去掉 build: 段
```

> **注意**:本流水线**只构建后端镜像**,前端镜像构建尚未配入 CI。如需 CI 构建前端,新增 `frontend-ci.yml`,使用 `frontend/Dockerfile`。

---

## 7. 部署后验收

### 7.1 健康检查端点

| 端点 | 用途 | 返回 |
|------|------|------|
| `GET /healthz/live` | **Liveness 探针**(K8s/Docker) — 进程存活即 200 | `{"status":"ok"}` |
| `GET /healthz/ready` | **Readiness 探针** — 含 DB + Redis ping | 200 `{"status":"ok"}` 或 503 `{"status":"unhealthy","errors":[...]}` |
| `GET /metrics` | Prometheus 文本格式指标(runtime + 业务) | 文本 |

```bash
curl http://localhost:8080/healthz/live    # 必须 200
curl http://localhost:8080/healthz/ready   # 必须 200,503 表示 DB/Redis 异常
```

> ⚠️ **历史 bug**:`backend/Dockerfile` 的 `HEALTHCHECK` 仍指向 `/healthz`(不存在),容器 `unhealthy`。修复方案见 §9.2 第 6 条。

### 7.2 冒烟测试

仓库内 `scripts/smoke.sh` 是端到端 smoke 测试,登录后探测 16 个关键 GET 端点:

```bash
# 默认 BASE_URL=http://localhost:8080
./scripts/smoke.sh

# 自定义
BASE_URL=https://api.example.com USERNAME=admin PASSWORD=admin123 ./scripts/smoke.sh
```

依赖:`bash` / `curl` / `jq`(Windows 用 git-bash)。

退出码 0 = 全通,非 0 = 第一个失败就退。

### 7.3 验收清单(任何部署方式都要走一遍)

- [ ] `curl /healthz/live` 返回 200
- [ ] `curl /healthz/ready` 返回 200(DB + Redis 都通)
- [ ] 默认账号 `admin/admin123` 能登录 — **立即修改密码**
- [ ] `./scripts/smoke.sh` 全通(16/16)
- [ ] 上传一张图片 → 能在 storage 中看到文件
- [ ] 创建一个 Pipeline Run → worker 能消费(查 worker 日志有 `task accepted`)
- [ ] WebSocket `/ws/pipeline-runs/{run_id}` 能收到进度推送
- [ ] 日志无 ERROR 级别输出(`grep ERROR logs/server.log`)

---

## 8. 运维通用项

### 8.1 日志位置速查

| 部署方式 | 位置 / 命令 |
|---------|------------|
| 根 docker compose | `docker compose logs -f server worker` |
| deploy docker compose | `docker compose logs -f backend worker` |
| Linux systemd | `journalctl -u ai-script-server -f` 或 `/var/log/ai-script/server.log` |
| Windows NSSM | `C:\ai-script\logs\server.log` |
| K8s | `kubectl logs -f deploy/backend -n ai-script` |

日志格式:`app.env=prod` 时 JSON(适配 ELK/Loki),`env=dev` 时彩色控制台。

### 8.2 监控指标

| 指标 | 检查方式 | 告警阈值 |
|------|---------|---------|
| HTTP 可用性 | `curl /healthz/live`(连续 3 次失败告警) | - |
| 数据依赖健康 | `curl /healthz/ready`(503 告警) | - |
| 任务队列积压 | `redis-cli LLEN asynq:{default}` | > 1000 |
| MySQL 连接数 | `SHOW STATUS LIKE 'Threads_connected'` | > 80% `max_connections` |
| Redis 内存 | `redis-cli INFO memory` | > 80% maxmemory |
| 磁盘空间 | `df -h` | > 80% |
| Prometheus | `GET /metrics`(集成 Prometheus + Grafana) | 自定义 |

### 8.3 备份与恢复

#### MySQL 每日全量

```bash
# 定时任务(crontab -e)
0 2 * * * mysqldump -u aiscript -p'password' --single-transaction --routines ai_script | gzip > /backup/ai-script-$(date +\%Y\%m\%d).sql.gz

# 保留 7 天
0 3 * * * find /backup -name "ai-script-*.sql.gz" -mtime +7 -delete
```

恢复:

```bash
gunzip < /backup/ai-script-20260513.sql.gz | mysql -u aiscript -p ai_script
```

#### Redis

```bash
# 启用 AOF
echo "appendonly yes" >> /etc/redis/redis.conf
sudo systemctl restart redis-server

# 手动快照
redis-cli BGSAVE
cp /var/lib/redis/dump.rdb /backup/redis-$(date +%Y%m%d).rdb
```

#### 对象存储

- 本地存储:rsync 定期同步到备份服务器
- 云存储:开启 versioning + 跨区域复制
- MinIO:`mc mirror local/ai-script backup/ai-script-backup`

#### 配置 / 密钥

```bash
tar czf /backup/ai-script-config-$(date +%Y%m%d).tar.gz \
  /opt/ai-script/configs \
  /opt/ai-script/.env
chmod 600 /backup/ai-script-config-*.tar.gz
```

### 8.4 安全加固

| 项 | 建议 |
|----|------|
| **JWT_SECRET** | `openssl rand -base64 48`,>= 64 字符,每 90 天轮换 |
| **CRYPTO_KEY** | 永远不要提交到 git;K8s 用 Secret;裸机用 600 权限的 .env |
| **数据库** | 禁止 root 远程登录;业务用户最小权限;启用 SSL/TLS;定期改密 |
| **Redis** | 绑定 127.0.0.1 或内网;设 `requirepass`;`bind` + `protected-mode yes` |
| **HTTPS** | Nginx / Ingress 反代;不要让 8080 直接对公网 |
| **CORS** | 生产关闭 `AllowAllOrigins`,改白名单 |
| **对象存储** | bucket 不公开;用预签名 URL |
| **容器** | 使用非 root(`appuser:1000`);定期 `trivy image ai-script-backend:latest` 扫漏洞 |
| **RBAC** | 数据库预置角色-权限;敏感操作二次确认;定期审计 |

### 8.5 数据库 Schema 迁移(生产建议)

GORM AutoMigrate 只增不删,不适合长期生产。建议改用 **golang-migrate**:

```bash
# 安装
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 创建新迁移
migrate create -ext sql -dir backend/migrations -seq add_xxx_column

# 执行
migrate -path backend/migrations \
  -database "mysql://aiscript:password@tcp(localhost:3306)/ai_script" up

# 回滚一个版本
migrate -path backend/migrations \
  -database "mysql://aiscript:password@tcp(localhost:3306)/ai_script" down 1
```

K8s 中可以用 `Job` 在新版本上线前先跑迁移,或者在 backend 启动时通过 `MIGRATE_MODE=manual` 跳过 AutoMigrate(已经在 `deploy/k8s/configmap.yaml` 中预留 `MIGRATE_MODE` 字段)。

---

## 9. FAQ — 常见问题速答

### 9.1 环境与依赖

**Q1: Go 版本到底是多少?**
`go.mod` 锁定 `1.24.0`,这是事实标准。本机编译与镜像构建均需要 Go 1.24+。

**Q2: 必须装 FFmpeg 吗?**
B/C 二进制部署必装(系统级);A 容器部署不用(`backend/Dockerfile` 通过 `apk add ffmpeg` 内置);D K8s 用同一个 backend 镜像,自动包含。

**Q3: 必须装 MinIO 吗?**
不必须。三选一:本地文件系统(开发用)、MinIO(自建对象存储)、阿里云 OSS / AWS S3 / 腾讯云 COS。配置 `OSS_PROVIDER` 切换:`local` / `minio` / `oss` / `s3` / `cos`。

**Q4: 必须有模型网关(LiteLLM/OneAPI)吗?**
对于实际生成图片/视频的功能 → 必须。仅做接口联调 → 可空着 `MODEL_GATEWAY_URL/KEY`,但相关接口会报错。

### 9.2 启动失败类

**Q1: `JWT_SECRET is required` / `MYSQL_DSN is required`**
.env 没填或填错。检查:`.env` 文件是否存在 / 是否在 compose 同目录 / 变量名是否拼错(根 compose 用 `MYSQL_ROOT_PWD`,deploy compose 用 `MYSQL_ROOT_PASSWORD`,**不通用**)。

**Q2: `CRYPTO_KEY` 长度报错**
- 走 `CRYPTO_KEY` 路径:必须正好 **32 字节明文**(不是 32 字符的可见字符,而是字节)
- 走 `CRYPTO_KEY_BASE64` 路径:必须是 32 字节的 **base64 编码字符串**(`openssl rand -base64 32`)
- 两者**二选一**

**Q3: `Error 1045: Access denied` 连不上 MySQL**
1. 用户名/密码错;
2. 用户没远程权限:`GRANT ALL ON ai_script.* TO 'aiscript'@'%'`;
3. 容器内访问:DSN 用容器名 `tcp(mysql:3306)`,不是 `127.0.0.1`。

**Q4: `dial tcp [::1]:6379: connection refused` 连不上 Redis**
- 裸机:`sudo systemctl status redis-server`
- Docker:`REDIS_ADDR` 应该是容器名 `redis:6379`,不是 `localhost`
- 远程 Redis:防火墙、`bind` 配置、`requirepass` 三项核对

**Q5: server 启动后立即退出,无明显报错**
- systemd:`journalctl -u ai-script-server -n 200` 看完整日志
- NSSM:看 `C:\ai-script\logs\server.log`
- Docker:`docker compose logs server`
- 常见根因:`configs/config.yaml` 不存在 / 权限不足 / `CRYPTO_KEY` 格式错

**Q6: Docker 容器一直 `unhealthy` / Compose `depends_on` 卡住**
`backend/Dockerfile` 的 `HEALTHCHECK` 写的是 `http://127.0.0.1:8080/healthz`,但代码里实际只有 `/healthz/live` 和 `/healthz/ready`。两个解决方案:
- 修 Dockerfile,把 `HEALTHCHECK` 改为 `/healthz/live`(根本修复)
- 或在 compose 的 `healthcheck.test` 覆盖容器内置健康检查(本仓库的 compose 已经这样做了,所以 compose 启动是 OK 的,只是直接 `docker run` 会卡)

### 9.3 配置类

**Q1: 三个 `.env.example` 用哪个?**
见 [§1.4](#14-envexample-三个文件指引)。简单说:用根 compose → 根 `.env.example`;用 deploy compose → `deploy/.env.example`;裸机/Windows → `backend/.env.example`。

**Q2: 两个 Makefile 怎么选?**
- 根 `Makefile`:打包 backend + frontend + compose 操作,适合**整个项目**层面
- `backend/Makefile`:只管 backend,带 `help` 自描述,适合**后端开发者**
- 同名命令行为不同:根 `make docker` = `docker compose build`,`backend/make docker` = `docker build -t ai-script:latest .`

**Q3: 改了 .env 不生效?**
- Docker:必须 `down` 后 `up -d`,`restart` 不会重读 env_file
- systemd:改了 `EnvironmentFile` 后 `systemctl daemon-reload` + `systemctl restart`
- NSSM:`nssm set <svc> AppEnvironmentExtra ...` 后 `nssm restart`

**Q4: 时区不对(显示 UTC)**
- MySQL 容器命令行已加 `--default-time-zone=+08:00`
- 应用层:DSN 加 `loc=Local`(注:目前 deploy compose 是 `loc=Local`,旧 ops-guide 写的 `loc=UTC` 是历史值)
- 容器层:`TZ=Asia/Shanghai` 环境变量

### 9.4 性能与资源

**Q1: Worker 不消费任务**
1. worker 进程死了:`ps aux | grep worker` / `kubectl get pods | grep worker`
2. Redis DB 编号不一致:server 和 worker 的 `REDIS_DB` 必须相同
3. 任务 handler panic:看 worker 日志最后几行
4. 队列名错:Asynq 默认队列 `asynq:{default}`,用 `redis-cli LLEN asynq:{default}` 看积压

**Q2: 媒体合成 OOM**
- FFmpeg 默认会吃满 CPU 和大量内存
- Docker:在 deploy compose 加大 worker 的 `deploy.resources.limits.memory`(默认 1G,可调到 4G)
- K8s:同理调 `worker-deployment.yaml`
- 长任务超时:调 ConfigMap 的 `TIMEOUT_VIDEO_COMPOSE`(秒)

**Q3: MySQL 连接耗尽**
- 调 `MYSQL_MAX_OPEN`(默认 100)
- 检查 `MYSQL_CONN_MAX_LIFETIME`(默认 3600s),避免老连接挂死
- MySQL 服务端 `max_connections`(compose 里设了 500,生产可加大)

### 9.5 网络与权限

**Q1: 前端访问后端跨域报错**
- 开发环境:`frontend/.env.example` 的 `VITE_API_BASE` 指向后端
- 生产环境:让 nginx/Ingress 把 `/api` 与 `/ws` 反代给 backend(K8s 的 `ingress.yaml` 已经这么做)
- 不要在生产 CORS 设 `AllowAllOrigins`

**Q2: WebSocket 连不上**
- nginx 需要加 `Upgrade` / `Connection` 头
- Ingress 注解可能需要 `nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"`
- 反代后路径要保持 `/ws/...`

**Q3: 上传文件 413 Request Entity Too Large**
- nginx 默认 1MB;deploy/k8s/ingress.yaml 已设 `proxy-body-size: 100m`
- 自建 nginx:`client_max_body_size 100m;`
- 应用层有 `bodySizeLimitMiddleware` 默认 1MB(非上传路由),上传路由是放行的

### 9.6 升级与回滚

**Q1: 升级前要不要备份?**
**必须**。`mysqldump` + `configs/` + `.env` 都备份。详见 §8.3。

**Q2: Schema 变更如何处理?**
当前阶段靠 GORM AutoMigrate 自动增列。**减列 / 改类型**必须手写 SQL 或用 golang-migrate(§8.5),否则 AutoMigrate 不会执行。

**Q3: K8s 怎么灰度?**
- 简单做法:把 backend deployment 复制一份(`backend-canary`),Service 加 `canary` 标签筛选,Ingress 加 `canary` 注解按百分比路由
- 进阶:Istio / Argo Rollouts

**Q4: 回滚到旧版?**
- Docker:`docker compose pull <旧 tag> && docker compose up -d`
- 二进制:用部署前备份的 `.bak`(见 §3.7 / §4.6)
- K8s:`kubectl rollout undo`

### 9.7 其他

**Q1: 默认密码 `admin/admin@123` 还是 `admin/admin123`?**
**`admin/admin123`**(seed.go 写死)。README 中 `admin@123` 是历史误写,已在本指南 §1.2 修正。

**Q2: 加密密钥 `CRYPTO_KEY` vs `CRYPTO_KEY_BASE64` 选哪个?**
功能等价,二选一。新部署推荐 `CRYPTO_KEY_BASE64`(`openssl rand -base64 32` 一行生成,K8s Secret 友好)。

**Q3: 怎么生成 `CRYPTO_KEY`?**
代码里有 `cmd/genkey`(若已实现)。或:
- `CRYPTO_KEY`:`openssl rand -base64 32 | head -c 32`
- `CRYPTO_KEY_BASE64`:`openssl rand -base64 32`

**Q4: ARM 服务器能跑吗?**
- 容器:CI 已构建 `linux/amd64 + linux/arm64` 多架构镜像,直接拉 latest 即可
- 裸机 ARM:`GOARCH=arm64 go build` 重新编译

---

## 附录

### A. Makefile 命令速查

#### 根 Makefile

| 命令 | 说明 | 备注 |
|------|------|------|
| `make build` | 构建 backend(server + worker)→ `backend/bin/` | 默认目标 |
| `make build-frontend` | 前端 `npm ci && npm run build` | |
| `make build-all` | 后端 + 前端 | |
| `make run` / `make server` | 本地启动 backend server | |
| `make worker` | 本地启动 backend worker | |
| `make migrate` | 跑 `cmd/server -migrate` | |
| `make test` | 后端 `go test ./... -race -count=1` | |
| `make test-frontend` | 前端 npm test | |
| `make lint` | 后端 golangci-lint | |
| `make lint-frontend` | 前端 ESLint | |
| `make fmt` | gofmt -w | |
| `make swagger` | 生成 OpenAPI 文档 | |
| `make docker` | `docker compose build` | ⚠️ 与 `backend/make docker` 行为不同 |
| `make up` | `docker compose up -d` | |
| `make down` | `docker compose down` | |
| `make clean` | 清 `backend/bin/` | |

#### `backend/Makefile`

| 命令 | 说明 | 备注 |
|------|------|------|
| `make help` | 自描述帮助 | |
| `make tidy` | `go mod tidy` | |
| `make run` | 本地启动 server | |
| `make worker` | 本地启动 worker | |
| `make build` | 构建 → `backend/out/` | ⚠️ 路径与根 Makefile 不同 |
| `make test` | `go test ./...` | |
| `make lint` | golangci-lint | |
| `make docker` | `docker build -t ai-script:latest .` | ⚠️ 与根 `make docker` 行为不同 |
| `make swagger` | 生成 swagger | |

### B. 环境变量速查表

```bash
# ---- App ----
APP_ENV=prod                    # local / dev / staging / prod
APP_PORT=8080
APP_LOG_LEVEL=info              # debug / info / warn / error
GIN_MODE=release
CONFIG_FILE=/app/configs/config.yaml

# ---- MySQL ----
MYSQL_DSN=aiscript:password@tcp(host:3306)/ai_script?charset=utf8mb4&parseTime=True&loc=Local
MYSQL_MAX_IDLE=10
MYSQL_MAX_OPEN=100
MYSQL_CONN_MAX_LIFETIME=3600
MYSQL_CONN_MAX_IDLE_TIME=1800

# ---- Redis ----
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=20
REDIS_MIN_IDLE_CONNS=5
REDIS_POOL_TIMEOUT=5

# ---- JWT ----
JWT_SECRET=                     # >= 32 字符随机
JWT_EXPIRES_IN=7200             # access token 秒数
JWT_REFRESH_EXPIRES_IN=604800   # refresh token 秒数

# ---- Crypto(二选一)----
CRYPTO_KEY=                     # 32 字节明文
CRYPTO_KEY_BASE64=              # base64 编码的 32 字节

# ---- 对象存储 ----
OSS_PROVIDER=local              # local / minio / oss / s3 / cos
OSS_ENDPOINT=
OSS_REGION=
OSS_BUCKET=./var/uploads
OSS_ACCESS_KEY=
OSS_SECRET_KEY=
OSS_PUBLIC_HOST=/uploads

# ---- 模型网关 ----
MODEL_GATEWAY_URL=
MODEL_GATEWAY_KEY=

# ---- 迁移(可选,生产 golang-migrate)----
MIGRATE_MODE=auto               # auto / manual
MIGRATE_DSN=
MIGRATE_SOURCE=file:///app/migrations

# ---- 业务超时(秒)----
TIMEOUT_IMAGE_GEN=120
TIMEOUT_VIDEO_GEN=120
TIMEOUT_VIDEO_COMPOSE=300
TIMEOUT_MODEL_HEALTH=30
TIMEOUT_PIPELINE_RUN=600
```

### C. 端口占用速查

| 端口 | 服务 | Compose 是否暴露 | K8s 是否暴露 |
|------|------|----------------|-------------|
| 80 | frontend nginx | ✅ | Ingress |
| 8080 | backend server | ✅ | Service ClusterIP + Ingress |
| 3306 | MySQL | 127.0.0.1:3306 only | 仅 cluster 内 |
| 6379 | Redis | 127.0.0.1:6379 only | 仅 cluster 内 |
| 9000 | MinIO API | 127.0.0.1:9000 only | 不部署 |
| 9001 | MinIO Console | 127.0.0.1:9001 only | 不部署 |

### D. 部署文件清单(对照 repo)

```
ai-script/
├── README.md                      # 项目入口(快速开始)
├── Makefile                       # 项目级 Makefile(整合 backend + frontend + compose)
├── docker-compose.yml             # ⭐ 开发用 compose
├── .env.example                   # ⭐ 开发用 env 模板
├── backend/
│   ├── Dockerfile                 # backend 镜像(server + worker)
│   ├── Makefile                   # 后端独立 Makefile
│   ├── .env.example               # backend 独立运行用 env 模板(systemd/NSSM)
│   ├── configs/
│   │   └── config.yaml            # 主配置(被环境变量覆盖)
│   └── cmd/
│       ├── server/                # HTTP API 入口
│       └── worker/                # Asynq worker 入口
├── frontend/
│   ├── Dockerfile                 # 前端镜像(nginx + dist)
│   ├── nginx.conf                 # nginx 配置
│   └── .env.example               # VITE_API_BASE 等前端 env
├── deploy/
│   ├── docker-compose.yml         # ⭐ 生产用 compose
│   ├── .env.example               # ⭐ 生产用 env 模板
│   └── k8s/                       # ⭐ K8s manifests
│       ├── namespace.yaml
│       ├── configmap.yaml
│       ├── secret.yaml            # ⚠️ 模板,部署前替换占位符
│       ├── backend-deployment.yaml
│       ├── worker-deployment.yaml
│       ├── frontend-deployment.yaml
│       └── ingress.yaml
├── scripts/
│   ├── smoke.sh                   # ⭐ 部署后冒烟测试
│   └── sql/
│       ├── 001_init.sql           # DDL 备份(主要靠 GORM AutoMigrate)
│       └── 002_seed.sql           # 部门/权限点种子数据
└── .github/
    └── workflows/
        └── ci.yml                 # ⭐ CI/CD 流水线
```

### E. 文档协作约定

- 本文档由 **DevOps 团队** 维护
- 修改建议先开 issue 讨论,涉及部署方式新增/删除需评审
- 与代码不一致时,**以代码为准**,文档应同步更新
- 与 README 不一致时,**以本文档为准**

---

> 有问题?先查 [§9 FAQ](#9-faq--常见问题速答),再查对应部署方式章节的"常见问题",找不到答案再联系运维。
