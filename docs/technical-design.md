# AI Script 技术设计

本文描述当前仓库的实际架构。

## 架构

- `server`：HTTP API、WebSocket、认证鉴权
- `worker`：Asynq 消费、模型调用、FFmpeg、进度推送
- 共享依赖：MySQL、Redis、对象存储

## 运行模型

- API 只负责接收请求、校验参数、投递异步任务
- worker 负责执行耗时任务并写回状态
- 进度通过 Redis Pub/Sub 转发到 `GET /ws/progress`

## 配置模型

当前只保留：

- 环境变量
- 代码默认值
- 根目录 `.env.example`

关键约束：

- `JWT_SECRET` 必填
- `CRYPTO_KEY_BASE64` 必填，解码后必须是 32 字节
- `MYSQL_USER` / `MYSQL_PASSWORD` 必填；`MYSQL_HOST` / `MYSQL_PORT` / `MYSQL_DATABASE` / `MYSQL_CHARSET` / `MYSQL_PARSE_TIME` / `MYSQL_MULTI_STATEMENTS` 按需配置
- `REDIS_HOST` 必填，`REDIS_PORT` 可选
- 生产环境必须设置 `APP_ORIGINS`

## 对外接口

- 健康检查：`/healthz/live`、`/healthz/ready`
- 指标：`/metrics`
- WebSocket：`/ws/progress`

## 任务链路

典型链路：

`script -> episode -> prompt -> storyboard -> image -> short_video -> full_video -> review -> publish`

异步任务统一由 `worker` 处理，前端通过 WebSocket 获取进度。

## 部署约束

- Docker 部署使用根目录 `docker-compose.yml` 或 `deploy/docker-compose.yml`
- 手动编译运行使用 `backend/cmd/server`、`backend/cmd/worker` 和前端构建产物
- 不再把 Kubernetes 作为当前仓库的推荐运行方案
