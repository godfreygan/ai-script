# AI Script 运维指南

本文只描述当前仓库实际支持的运行和运维方式。

## 当前约定

- 后端只读取环境变量和代码默认值
- 唯一配置模板是根目录 `.env.example`
- 只使用 `CRYPTO_KEY_BASE64`
- 部署方式只保留两种：Docker、手动编译运行
- 生产环境必须设置 `APP_ORIGINS`

## 部署前检查

- 已准备 `.env`
- 已设置 `JWT_SECRET`
- 已设置 `CRYPTO_KEY_BASE64`
- 已设置 `MYSQL_HOST` / `MYSQL_PORT` / `MYSQL_USER` / `MYSQL_PASSWORD` / `MYSQL_DATABASE`
- 已设置 `REDIS_HOST` / `REDIS_PORT`
- 已设置 `OSS_PROVIDER`
- 已准备 `ffmpeg`

```bash
openssl rand -base64 48
openssl rand -base64 32
```

## 部署方式

- Docker：见 [部署指南](./deployment-guide.md#docker-部署)
- 手动编译运行：见 [部署指南](./deployment-guide.md#手动编译运行)

## 健康检查

- server：`GET /healthz/live`、`GET /healthz/ready`
- worker：`APP_PORT + 1000`，同样提供 `GET /healthz/live`、`GET /healthz/ready`、`GET /metrics`

## 常见问题

- `mysql.dsn is empty`：检查 `.env` 是否加载，以及是否已设置 `MYSQL_USER` / `MYSQL_PASSWORD`，并按需设置 `MYSQL_HOST` / `MYSQL_PORT` / `MYSQL_DATABASE`
- `redis.addr is empty`：检查 `.env` 是否加载，以及是否已设置 `REDIS_HOST`，并按需设置 `REDIS_PORT`
- `CRYPTO_KEY_BASE64` 格式错误：必须是 32 字节随机密钥的 base64
- `APP_ORIGINS` 缺失：生产环境会拒绝放开 CORS
- 视频生成失败：先确认 `ffmpeg` 可执行
