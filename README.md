# AI 短剧视频生产平台

面向内部团队的 AI 短剧生产工具。打通「剧本 → 分集提示词 → 分镜/风格 → 图片 → 短视频 → 完整视频 → 审核 → 发布」的全链路；每一步都可独立调用，也可通过可视化流水线编排串联。

## 技术栈

| 层 | 组件 |
|----|------|
| 前端 | React 18 + TypeScript + Vite + Ant Design 5 + ReactFlow + Zustand |
| 后端 | Go 1.22 + Gin + GORM + Casbin + Asynq + Zap |
| 存储 | MySQL 8.0 + Redis 7 + 对象存储 (OSS/COS) |
| 调度 | Asynq (Redis) |
| 模型网关 | LiteLLM / One-API (OpenAI 兼容) |

## 目录结构

```
ai-script/
├── backend/          # Go 后端 (server + worker)
├── frontend/         # React 前端
├── scripts/sql/      # DDL + 种子数据
├── docs/             # 需求 / 数据库 / API / MVP 计划
└── deploy/           # docker-compose + nginx
```

## 快速开始（开发）

```bash
# 1. 启动依赖
cd deploy && cp .env.example .env && docker-compose up -d mysql redis

# 2. 后端
cd ../backend
go mod tidy
make run            # 启动 HTTP server
make worker         # 另起一个终端启动 worker

# 3. 前端
cd ../frontend
npm install
npm run dev
```

默认管理员账号：`admin` / `admin@123`（首次登录后请立即修改）。

## 一键部署

```bash
cd deploy
cp .env.example .env  # 修改 JWT_SECRET / CRYPTO_KEY_BASE64
docker-compose up -d --build
```

访问 http://localhost/ 。

## 文档索引

- 产品需求与实施计划：[docs/prd.md](./docs/prd.md)
- 技术设计（架构 + 数据库 + API 约定）：[docs/technical-design.md](./docs/technical-design.md)
- OpenAPI 规范：`docs/openapi.yaml`
- 用户操作指南：[docs/user-guide.md](./docs/user-guide.md)
- 部署运维指南：[docs/ops-guide.md](./docs/ops-guide.md)
- DDL：`scripts/sql/001_init.sql`

## 开发约定

- 所有 HTTP 响应统一信封 `{code, message, data, request_id}`，详见 `docs/technical-design.md` §5.7
- 异步任务返回 `task_id`，进度走 WebSocket `/ws/pipeline-runs/{run_id}`
- 模型 `api_key` 入库前 AES-256-GCM 加密（`CRYPTO_KEY_BASE64`）
- 软删除统一使用 `gorm.DeletedAt`
- 数据范围基于 Casbin + `data_scope`(self/dept/all)
