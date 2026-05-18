# AI Script Platform

AI Script 是一个短剧视频生成平台，仓库包含：

- `backend/`：后端 API 与 worker
- `frontend/`：前端控制台
- `deploy/`：Docker 部署清单
- `docs/`：设计、运维与使用文档

## 配置

使用根目录 [`.env.example`](.env.example) 作为唯一模板：

```bash
cp .env.example .env
```

关键变量：

- `JWT_SECRET`
- `CRYPTO_KEY_BASE64`
- `MYSQL_HOST` / `MYSQL_PORT` / `MYSQL_USER` / `MYSQL_PASSWORD` / `MYSQL_DATABASE`
- `REDIS_HOST` / `REDIS_PORT`
- `APP_ORIGINS`

## 运行方式

当前只推荐两种方案：

1. Docker 部署
2. 手动编译运行

详细步骤见 [部署指南](./docs/deployment-guide.md)。

## 文档

- [部署指南](./docs/deployment-guide.md)
- [运维指南](./docs/ops-guide.md)
- [技术设计](./docs/technical-design.md)
- [用户指南](./docs/user-guide.md)
- [需求说明](./docs/prd.md)
