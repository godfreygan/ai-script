# AI 短剧视频生成平台 · 技术设计文档

> 版本：v1.0
> 日期：2026-05-12
> 仓库：`git.myscrm.cn/ganqx01/ai-script/backend`
> 语言：Go 1.22
>
> 说明：本文档由 `architecture.md` + `database-design.md` + `api-design.md`(通用约定部分) 合并而成。

---

## 目录

- [1. 系统概述与目标](#1-系统概述与目标)
- [2. 技术架构图](#2-技术架构图)
- [3. 模块职责说明](#3-模块职责说明)
- [4. 数据流](#4-数据流)
- [5. 核心设计决策](#5-核心设计决策)
  - [5.7 API 通用约定](#57-api-通用约定)
- [6. 数据库实体关系概述](#6-数据库实体关系概述)
  - [6.4 详细表结构](#64-详细表结构)
  - [6.5 命名规约](#65-命名规约)
  - [6.6 容量估算](#66-容量估算)
- [7. 部署拓扑建议](#7-部署拓扑建议)
- [附录：代码路径索引](#附录代码路径索引)

---

## 1. 系统概述与目标

AI-Script Backend 是一个面向企业级场景的 AI 视频生成平台后端，支持从原始剧本到完整成片的全链路自动化生产。系统核心目标包括：

1. **多模态 AI 编排**：统一对接 text / image / video / audio 四类模型，通过适配器屏蔽底层供应商差异。
2. **流水线化生产**：剧本 -> 分集 -> 提示词 -> 分镜 -> 图片 -> 短视频 -> 完整视频，支持自定义 DAG 编排。
3. **异步高吞吐**：LLM 调用与媒体合成均为长耗时操作，采用任务队列解耦，支持水平扩展 Worker。
4. **实时进度推送**：WebSocket + Redis Pub/Sub 跨进程桥接，确保用户在前端实时看到生成进度。
5. **企业级权限**：RBAC 细粒度权限控制，支持用户、部门、角色、项目成员多级数据隔离。
6. **可观测与审计**：统一错误码、Zap 结构化日志、模型调用日志与计费统计。

---

## 2. 技术架构图

### 2.1 整体架构

```
+---------------------+        +---------------------+
|   Client (Web/App)  |        |   Admin Dashboard   |
+----------+----------+        +----------+----------+
           |                              |
           |  HTTP / WS                   |  HTTP
           v                              v
+------------------------------------------------------+
|                    AI-Script Server                   |
|  (cmd/server/main.go)                                |
|  +-----------------------------------------------+   |
|  |  Gin Router  +  Middleware                    |   |
|  |  (JWT / RBAC / CORS / RequestID / AccessLog)  |   |
|  +-----------------------------------------------+   |
|  |  Handler Layer  (REST API)                    |   |
|  +-----------------------------------------------+   |
|  |  Service Layer  (Business Logic)              |   |
|  +-----------------------------------------------+   |
|  |  Repo Layer  (GORM + MySQL)                   |   |
|  +-----------------------------------------------+   |
|  |  WS Hub  (Gorilla + Redis Pub/Sub Bridge)     |   |
|  +-----------------------------------------------+   |
|  |  Task Client  (Asynq enqueue)                 |   |
|  +-----------------------------------------------+   |
+------------------------------------------------------+
           |                              |
           |  SQL                         |  Redis
           v                              v
+------------------------------------------------------+
|  MySQL 8.x          |  Redis 6.x+        |  Object    |
|  (业务数据 / 事务)   |  (缓存 / 队列 / WS) |  Storage   |
+------------------------------------------------------+
           ^                              |
           |  Redis Pub/Sub               |  enqueue
           |                              v
+------------------------------------------------------+
|                    AI-Script Worker                   |
|  (cmd/worker/main.go)                                |
|  +-----------------------------------------------+   |
|  |  Asynq Server  (task consumer)                |   |
|  |  Queues: critical / default / low             |   |
|  +-----------------------------------------------+   |
|  |  DAG Runner  (in-process node execution)      |   |
|  |  - NodeHandlerRegistry                        |   |
|  +-----------------------------------------------+   |
|  |  Service Layer  (reuse server logic)          |   |
|  +-----------------------------------------------+   |
|  |  Adapter Layer  (call LiteLLM / upstream)     |   |
|  +-----------------------------------------------+   |
|  |  FFmpeg  (video concat / mix / burn subs)     |   |
|  +-----------------------------------------------+   |
|  |  WS Hub  (publish progress via Redis)         |   |
|  +-----------------------------------------------+   |
+------------------------------------------------------+
           |
           |  HTTP / SSE
           v
+------------------------------------------------------+
|  Upstream AI Gateway  (LiteLLM / One-API / Custom)   |
|  - Text:  GPT-4 / Claude / Qwen / DeepSeek           |
|  - Image: DALL-E / Stable Diffusion / Midjourney     |
|  - Video: Runway / Pika / CogVideo                   |
|  - Audio: TTS (Azure / ElevenLabs)                   |
+------------------------------------------------------+
```

### 2.2 进程划分

| 进程 | 入口 | 职责 |
|------|------|------|
| `server` | `cmd/server/main.go` | HTTP API、WebSocket 连接管理、任务投递、静态资源 |
| `worker` | `cmd/worker/main.go` | Asynq 任务消费、DAG 节点执行、模型调用、FFmpeg 合成 |
| `genkey` | `cmd/genkey/main.go` | 辅助工具：生成加密密钥 |
| `genpass`| `cmd/genpass/main.go`| 辅助工具：生成密码哈希 |

---

## 3. 模块职责说明

### 3.1 分层架构

```
+----------------------------------+
|  cmd/                            |  可执行入口
|  ├── server/main.go              |
|  ├── worker/main.go              |
|  └── genkey|genpass/main.go      |
+----------------------------------+
|  internal/handler/               |  HTTP 请求入口
|  - Gin Handler，参数校验，调用 Service
+----------------------------------+
|  internal/service/               |  业务逻辑层
|  - 编排业务规则，管理事务边界
|  - 异步任务封装（Enqueue -> Asynq）
+----------------------------------+
|  internal/repo/                  |  数据仓库层
|  - GORM CRUD，查询对象构造
|  - 屏蔽数据库差异
+----------------------------------+
|  internal/model/                 |  领域实体 / DTO
|  - GORM Model 定义，JSON 类型辅助
+----------------------------------+
|  internal/pipeline/              |  DAG 流水线引擎
|  - Runner：拓扑排序 + 分层并行执行
|  - NodeHandlerRegistry：节点处理器注册
|  - Asynq Handler：pipeline.run 任务桥接
+----------------------------------+
|  internal/adapter/               |  AI 模型适配器
|  - Adapter 接口：Generate / Healthcheck
|  - Registry：运行时加载 enabled 模型
+----------------------------------+
|  pkg/                            |  通用基础设施
|  ├── queue/      Asynq 客户端/服务端
|  ├── ws/         WebSocket Hub + Redis 桥接
|  ├── storage/    对象存储抽象（S3/MinIO/本地）
|  ├── jwt/        JWT 签发与校验
|  ├── casbin/     RBAC 权限策略管理
|  ├── crypto/     AES 加密（模型密钥等）
|  ├── ffmpeg/     媒体处理封装
|  ├── subtitle/   SRT 字幕生成
|  ├── logger/     Zap 日志
|  ├── errcode/    统一错误码
|  └── response/   统一 HTTP 响应封装
+----------------------------------+
```

### 3.2 各层详细职责

#### Handler 层 (`internal/handler/`)

- 负责 HTTP 请求参数绑定、基础校验、调用 Service、构造响应。
- 所有 Handler 聚合在 `Handlers` 结构体中，由 `NewHandlers` 统一注入依赖。
- 路由注册在 `internal/server/server.go` 的 `newRouter` 中，按 RESTful 风格组织。
- 关键路由分组：
  - `/api/v1/auth`：登录、刷新、登出（公开）
  - `/api/v1/users`、`/depts`、`/roles`：组织权限管理
  - `/api/v1/projects`：项目管理（含成员）
  - `/api/v1/models`：AI 模型注册与健康检查
  - `/api/v1/scripts`、`/episodes`、`/prompts`、`/storyboards`、`/images`、`/short_videos`：视频生成流水线各阶段
  - `/api/v1/pipelines/:id/run`：DAG 流水线触发
  - `/ws/progress`：WebSocket 进度订阅

#### Service 层 (`internal/service/`)

- 核心业务逻辑封装，每个域一个 Service：
  - `AuthService`：登录鉴权、密码修改、Token 刷新
  - `UserService`、`DeptService`、`RoleService`：组织与权限
  - `ProjectService`：项目 CRUD、成员管理、数据范围过滤
  - `ModelService`：模型注册、API 密钥加密存储、Adapter 动态加载、健康检查
  - `ScriptService`、`PromptService`、`StoryboardService`：剧本 -> 分集 -> 提示词 -> 分镜
  - `ImageService`、`ShortVideoService`：图片与短视频异步生成
  - `FullVideoService`：完整视频时间线编排、FFmpeg 合成任务投递
  - `PipelineService`：DAG 流水线定义 CRUD、运行触发
  - `InvocationService`：模型调用日志记录与统计
- 异步生成接口（如 `Script.Split`、`Image.Generate`）仅做参数校验 + Asynq 入队，立即返回 `task_id`。
- Worker 端通过 `HandleXxxTask` 方法消费任务，复用同一套 Service 逻辑。

#### Repo 层 (`internal/repo/`)

- `Repositories` 结构体聚合所有域的 Repo，统一注入 `*gorm.DB` 和 `*redis.Client`。
- 每个 Repo 封装对应模型的 CRUD 与列表查询，支持分页、过滤、排序。
- 不跨域调用，复杂业务组合由 Service 层完成。

#### Pipeline 引擎 (`internal/pipeline/`)

- **DAG 定义**：`DAG` 结构体包含 `Nodes`（节点列表）和 `Edges`（边与字段映射）。
- **Runner**：
  - 使用 Kahn 算法进行拓扑排序，检测环。
  - 按层执行，同层节点并行（`sync.WaitGroup` + goroutine）。
  - 节点输入通过 `edge.Mapping` 从上游输出映射，支持默认全量透传。
  - 每个节点写 `step_runs` 记录，支持重试与审计。
  - 任一节点失败即标记整个 `pipeline_runs` 为 `failed`。
- **NodeHandlerRegistry**：内存中的节点处理器注册表，`worker` 进程启动时注册所有内置节点。
- **Asynq 桥接**：`pipeline.run` 作为 Asynq 任务类型，由 `NewAsynqHandler` 解析负载后调用 `Runner.Execute`。

#### Adapter 层 (`internal/adapter/`)

- `Adapter` 接口定义统一方法：`Code()`、`Type()`、`Generate(ctx, *Request) (*Response, error)`、`Healthcheck(ctx) error`。
- `Request` / `Response` 为多模态通用结构，支持 prompt、neg_prompt、inputs（图片/音频 URL）、params。
- `Registry` 运行时管理所有 enabled 模型的 Adapter 实例，启动时从数据库加载并初始化。
- 实际实现推荐通过 LiteLLM / One-API 等统一网关中转，减少适配器数量。

#### WebSocket 实时推送 (`pkg/ws/`)

- `Hub` 管理 topic -> clients 的多路复用，支持多客户端订阅同一 topic。
- `BindRedis` 启用跨进程桥接：
  - `Publish` 时若绑定了 Redis，事件被序列化后发布到 Redis 通道。
  - `Run` 中启动 `subscribeRedis` 协程，接收跨进程事件并路由到本地 broadcast 通道。
- 客户端通过 `/ws/progress?token=xxx&topic=image:42` 建立连接，纯 Server Push 模式。
- 心跳：每 30 秒发送 WebSocket Ping 帧。

#### 对象存储 (`pkg/storage/`)

- `Storage` 接口：`Put`、`Get`、`Delete`、`SignURL`。
- 当前实现：本地文件存储（`local`），生产环境可扩展 S3 / MinIO / OSS / COS。
- 本地存储通过 Gin `Static` 暴露上传文件访问路径。

#### 任务队列 (`pkg/queue/`)

- `Client`：封装 Asynq 客户端，提供 `Enqueue` / `EnqueueIn`。
- `Server`：封装 Asynq 服务端，默认并发 16，队列优先级：`critical:6`、`default:3`、`low:1`。
- `HandlerRegistry` 接口用于 worker 端注册任务处理器。

---

## 4. 数据流

### 4.1 同步请求流（以用户查询项目列表为例）

```
Client
  -> Nginx / LB
    -> server (Gin)
      -> middleware: RequestID -> AccessLog -> JWTAuth -> RBAC
        -> handler.Project.List
          -> service.Project.List
            -> repo.Project.List (GORM + MySQL)
          -> response.Page{List, Total}
        -> JSON 200
```

### 4.2 异步生成任务流（以图片生成为例）

```
Client POST /api/v1/images/generate
  -> server (Gin)
    -> handler.Image.Generate
      -> service.Image.Generate
        -> 校验参数、构造 payload
        -> queue.Client.Enqueue("image.generate", payload)
        <- 返回 {task_id, topic: "image:<storyboard_id>"}

Client 建立 WS /ws/progress?topic=image:42
  -> server Hub 注册 client 到 topic "image:42"

Worker (Asynq Server)
  -> 消费 "image.generate" 任务
    -> service.Image.HandleGenerateTask
      -> 加载模型 Adapter
      -> 调用上游 Image 模型
      -> 保存结果到 images 表
      -> hub.Publish("image:42", Event{percent: 0.3, ...})
        -> Redis Publish (跨进程桥接)
          -> server Hub subscribeRedis
            -> broadcast 到本地 topic "image:42" 的 clients
              -> Client 收到进度事件
```

### 4.3 DAG 流水线执行流（以自定义 Pipeline 为例）

```
Client POST /api/v1/pipelines/1/run
  -> server
    -> PipelineService.Run
      -> queue.Client.Enqueue("pipeline.run", {pipeline_id:1, input:{...}})
      <- 返回 {run_id, status: "queued"}

Worker
  -> Asynq 消费 "pipeline.run"
    -> pipeline.NewAsynqHandler
      -> 创建 pipeline_runs 记录 (status=running)
      -> Runner.Execute(dagJSON, input, runID)
        -> 拓扑排序分层
        -> 逐层并行执行 NodeHandler
          -> 每节点：创建 step_runs -> 执行 -> 更新 step_runs
          -> 进度通过 hub.Publish("pipeline:<runID>", ...) 推送
        -> 全部成功 -> pipeline_runs.status = succeeded
        -> 任一失败 -> pipeline_runs.status = failed
```

### 4.4 完整视频合成流（FFmpeg）

```
Client POST /api/v1/full_videos/:id/render
  -> FullVideoService.Render
    -> Enqueue "video.compose"

Worker
  -> HandleComposeTask
    -> 加载 timeline (clips + TTS + BGM + subtitles)
    -> ffmpeg.ConcatVideos 拼接短视频片段
    -> ffmpeg.MixAudio 叠加 BGM / TTS 音轨
    -> ffmpeg.BurnSubtitles 烧录 SRT 字幕
    -> ffmpeg.ExtractThumb 抽取封面
    -> storage.Put 上传最终视频、封面、字幕
    -> 更新 full_videos 记录 (status=succeeded, output_url=...)
    -> hub.Publish("full:<id>", done)
```

---

## 5. 核心设计决策

### 5.1 为什么分 server / worker 两个进程？

| 维度 | Server | Worker |
|------|--------|--------|
| 职责 | HTTP API、WS 连接、任务投递 | 任务消费、模型调用、媒体合成 |
| 资源特征 | 轻量、低延迟、高并发连接 | 重计算、长耗时、大内存（视频） |
| 扩展方式 | 水平扩展（无状态） | 水平扩展（按队列长度自动伸缩） |
| 故障隔离 | API 不受耗时任务影响 | 单个任务崩溃不影响 API |
| 部署灵活性 | 可独立升级、灰度 | 可按 GPU/CPU 节点分组部署 |

Server 与 Worker 通过 Redis（Asynq 队列 + Pub/Sub）解耦，支持独立扩缩容。

### 5.2 为什么采用 DAG 流水线引擎？

- 视频生成涉及多阶段（剧本拆分、提示词生成、分镜、图片、视频、合成），阶段间存在依赖关系。
- DAG 支持可视化编排：用户可通过前端拖拽定义 `Nodes` 和 `Edges`。
- 同层并行：无依赖的节点（如多个图片生成）可并发执行，缩短总耗时。
- 字段映射：`Edge.Mapping` 支持上游输出字段映射到下游输入字段，实现灵活数据流。
- 可扩展：新增节点类型只需实现 `NodeHandler` 并注册到 `NodeHandlerRegistry`。

### 5.3 为什么 WebSocket 采用 Redis Pub/Sub 桥接？

- Server 进程维护 WS 客户端连接，Worker 进程执行任务。
- Worker 无法直接推送消息到 Server 进程的本地 Hub。
- Redis Pub/Sub 作为轻量级消息总线，实现跨进程事件广播，无需引入额外消息队列。
- 实现简单：`Hub.Publish` 判断若绑定了 Redis 则 `rdb.Publish`，否则直接本地 `broadcast`。

### 5.4 为什么对象存储使用抽象接口？

- 开发阶段使用本地文件存储，零依赖。
- 生产环境可无缝切换至 S3、MinIO、阿里云 OSS、腾讯云 COS。
- `Storage` 接口统一了 `Put/Get/Delete/SignURL`，业务代码无感知。

### 5.5 为什么模型密钥使用 AES 加密存储？

- 模型 API Key 属于敏感凭证，直接明文存储存在泄露风险。
- `crypto.Cipher` 提供 AES 加密，配置 `crypto.key` 作为主密钥。
- 数据库中存储 `api_key_encrypted`（`[]byte`），运行时解密后注入 Adapter。

### 5.6 为什么采用 Casbin + JWT 双层权限？

- JWT 负责身份认证（谁），携带用户 ID、部门 ID、角色列表。
- Casbin 负责授权（能做什么），基于 `rbac_model.conf` 定义策略。
- 路由元数据标记资源与动作（`rbac("project", "read")`），中间件统一鉴权，避免在每个 Handler 中写权限逻辑。

---



### 5.7 API 通用约定

#### 1.1 统一响应包络

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... },
  "request_id": "01HXxxxx"
}
```

- `code = 0` 成功;非 0 为业务错误
- 错误时 `data` 可为 `null`,详细信息在 `message` 和可选字段 `details`

#### 1.2 错误码段

| 段 | 含义 | 示例 |
|---|---|---|
| 0 | 成功 | 0 |
| 40000-40099 | 参数/校验 | 40001 参数缺失 |
| 40100-40199 | 鉴权 | 40101 未登录、40103 token 过期 |
| 40300-40399 | 授权 | 40301 无权限 |
| 40400 | 资源不存在 | |
| 40900-40999 | 业务冲突 | 40901 重复、40920 状态不允许 |
| 40290 | 额度耗尽 | 40291 部门额度不足 |
| 42900 | 限流 | |
| 50000-50099 | 系统错误 | 50001 内部错误、50002 下游模型错误 |
| 50300-50399 | 上游不可用 | 模型超时/网关不可用 |

#### 1.3 分页约定

请求:`?page=1&page_size=20&sort=created_at:desc`

响应:
```json
{
  "list": [...],
  "page": 1,
  "page_size": 20,
  "total": 123
}
```

#### 1.4 长任务规范

生成类接口立即返回 `task_id` + `run_id`,客户端通过两种方式获取结果:
- 轮询 `GET /tasks/{task_id}` 或 `GET /pipeline-runs/{run_id}`
- WebSocket 订阅 `/ws/runs/{run_id}` 获取实时进度

#### 1.5 通用响应头
- `X-Request-Id`:链路追踪
- `X-RateLimit-Remaining`:剩余配额

---



## 6. 数据库实体关系概述

### 6.1 核心实体清单

| 实体 | 说明 | 关键字段 |
|------|------|----------|
| `users` | 系统用户 | username, password_hash, dept_id, status |
| `departments` | 部门树 | name, parent_id, path |
| `roles` | 角色定义 | code, name, data_scope |
| `permissions` | 权限点 | code, resource, action |
| `user_roles` / `role_permissions` | 关联表 | 多对多 |
| `projects` | 项目 | code, name, owner_id, dept_id, default_pipeline_id |
| `project_members` | 项目成员 | project_id, user_id, role_in_project |
| `models` | AI 模型注册 | code, type, endpoint, api_key_encrypted, enabled |
| `scripts` | 剧本 | project_id, name, raw_text, current_version, status |
| `script_versions` | 剧本版本 | script_id, version_no, content, diff |
| `episodes` | 分集 | script_id, ep_no, title, summary, raw_segment |
| `episode_prompts` | 提示词版本 | episode_id, is_current, content, model_id |
| `storyboards` | 分镜 | episode_id, shot_no, shot_type, scene_desc, duration_sec |
| `styles` | 风格库 | project_id, art_style, color_tone, lighting, lora_id |
| `images` | 生成的图片 | storyboard_id, url, prompt, model_id, status |
| `short_videos` | 短视频片段 | storyboard_id, video_url, duration_ms, model_id, status |
| `full_videos` | 完整视频 | project_id, name, timeline, output_url, status, render_progress |
| `pipelines` | DAG 流水线定义 | project_id, name, dag (JSON), is_template |
| `pipeline_runs` | 流水线运行记录 | pipeline_id, status, input, output, started_at, ended_at |
| `step_runs` | 单步运行记录 | run_id, node_id, node_type, status, input, output, error_msg |
| `model_invocations` | 模型调用日志 | model_id, user_id, project_id, biz_type, duration_ms, status |
| `billing_quotas` / `billing_daily` | 计费配额与统计 | scope_type, scope_id, quota_value, used_value |
| `audit_logs` | 审计日志 | user_id, action, resource_type, before, after |

### 6.2 实体关系简图

```
users 1--* user_roles *--1 roles
users 1--* project_members *--1 projects
users *--1 departments

projects 1--* scripts 1--* episodes 1--* episode_prompts
                              |
                              +--* storyboards 1--* images
                              |           |
                              |           +--* short_videos
                              |           |
                              |           +--* styles (via storyboard_styles)
                              |
                              +--* full_videos
                              |
                              +--* pipelines 1--* pipeline_runs 1--* step_runs

models 1--* model_invocations
models 1--* model_pricing
```

### 6.3 关键设计点

- **软删除**：所有核心表使用 GORM `DeletedAt` 实现软删除。
- **审计字段**：`created_by`、`updated_by` 记录操作人。
- **JSON 字段**：`dag`、`timeline`、`params`、`meta` 等使用 `model.JSON`（`json.RawMessage` 别名）灵活存储结构化数据。
- **状态机**：
  - Script: `1=uploaded -> 2=parsed -> 3=episode_split`
  - ShortVideo / FullVideo: `draft -> queued -> running -> succeeded / failed`
  - PipelineRun: `running -> succeeded / failed`

---



### 6.4 详细表结构

#### 2.1 用户域

#### `departments` 部门
| 列 | 类型 | 说明 |
|---|---|---|
| id | BIGINT UNSIGNED PK | |
| name | VARCHAR(64) NOT NULL | |
| parent_id | BIGINT UNSIGNED | 上级部门 0=根 |
| path | VARCHAR(255) | 用 `/1/3/8` 缓存层级,做 ancestor 查询 |
| sort | INT | 排序 |
| status | TINYINT | 1=启用 |
| 通用字段 | | |
- 索引:`idx_parent_id(parent_id)`、`idx_path(path)`

#### `users` 用户
| 列 | 类型 | 说明 |
|---|---|---|
| id | BIGINT UNSIGNED PK | |
| username | VARCHAR(64) UNIQUE NOT NULL | 登录用 |
| password_hash | VARCHAR(128) NOT NULL | bcrypt |
| nickname | VARCHAR(64) | |
| email | VARCHAR(128) UNIQUE | |
| phone | VARCHAR(20) | |
| avatar_url | VARCHAR(512) | |
| dept_id | BIGINT UNSIGNED | |
| status | TINYINT | 1=正常 2=禁用 |
| last_login_at | DATETIME | |
| last_login_ip | VARCHAR(64) | |
| 通用字段 | | |
- 索引:`uk_username`, `uk_email`, `idx_dept_id`

#### `user_api_tokens` 个人 API Token
| 列 | 说明 |
|---|---|
| id, user_id, name, token_hash, scopes(JSON), expires_at, last_used_at, status |

#### 2.2 鉴权域

#### `roles`
- id, code(UNIQUE), name, description, data_scope(`self/dept/all`), is_system(TINYINT), status

#### `permissions` 权限点
- id, code(`project:read`, `model:manage`...), name, resource, action, description

#### `role_permissions`
- role_id, permission_id(联合主键)

#### `user_roles`
- user_id, role_id(联合主键)

#### 设计要点
- 用 Casbin 模型:`p, role, resource, action` + `g, user, role`
- 数据权限通过 `data_scope` 在业务层注入 SQL WHERE 实现

#### 2.3 项目域

#### `projects`
| 列 | 类型 | 说明 |
|---|---|---|
| id | BIGINT UNSIGNED PK | |
| code | VARCHAR(64) UNIQUE | 短码,如 `PRJ-2026-001` |
| name | VARCHAR(128) | |
| description | TEXT | |
| owner_id | BIGINT UNSIGNED | 负责人 |
| dept_id | BIGINT UNSIGNED | |
| status | TINYINT | 1=draft 2=in_production 3=in_review 4=published 5=archived |
| default_pipeline_id | BIGINT UNSIGNED | |
| cover_url | VARCHAR(512) | |
| tags | JSON | |
| 通用字段 | | |
- 索引:`uk_code`, `idx_owner_id`, `idx_dept_id`, `idx_status`

#### `project_members`
- id, project_id, user_id, role_in_project(producer/editor/reviewer/viewer), 通用字段
- 唯一:`uk_project_user(project_id, user_id)`

#### 2.4 剧本域

#### `scripts`
| 列 | 说明 |
|---|---|
| id, project_id, name, source_url(原文件), raw_text(MEDIUMTEXT), current_version, status(uploaded/parsed/episode_split), meta(JSON: 题材/标签/目标时长) |
- 索引:`idx_project_id`,FULLTEXT(`raw_text`) WITH PARSER ngram

#### `script_versions`
- id, script_id, version_no, content(MEDIUMTEXT), diff(JSON), commit_msg, created_by, created_at

#### `episodes`
- id, script_id, ep_no, title, summary, raw_segment(MEDIUMTEXT), char_begin, char_end, status
- 索引:`idx_script_ep(script_id, ep_no)` UNIQUE

#### 2.5 提示词域

#### `episode_prompts`
| 列 | 说明 |
|---|---|
| id, episode_id, version, is_current(TINYINT), content(JSON), model_id, generated_by, generation_params(JSON), status |
- 索引:`idx_episode_version(episode_id, version)` UNIQUE,`idx_episode_current(episode_id, is_current)`

#### 2.6 分镜 / 风格域

#### `storyboards`
| 列 | 说明 |
|---|---|
| id, episode_id, prompt_id, shot_no, shot_type(`wide/medium/close/extreme_close/establishing`), camera_motion(`pan/zoom/dolly/track/static`), scene_desc, characters(JSON), action, dialogue, duration_sec, notes, status |
- 索引:`idx_episode_shot(episode_id, shot_no)` UNIQUE

#### `styles`
| 列 | 说明 |
|---|---|
| id, project_id(NULL=公共), name, art_style, color_tone, lighting, reference_images(JSON urls), lora_id, description, status |

#### `storyboard_styles` (M:N)
- storyboard_id, style_id, applied_at, applied_by(联合主键 `storyboard_id+style_id`)

#### 2.7 图片域

#### `images`
| 列 | 说明 |
|---|---|
| id, project_id, storyboard_id(NULLABLE), src_type(`generated/uploaded`), url, thumb_url, width, height, prompt, neg_prompt, model_id, params(JSON), status, generated_in_run_id(关联 step_run) |
- 索引:`idx_project_id`, `idx_storyboard_id`, `idx_status`

#### 2.8 短视频域

#### `short_videos`
| 列 | 说明 |
|---|---|
| id, project_id, storyboard_id(NULLABLE), src_type(`generated/uploaded`), prompt, source_image_ids(JSON), video_url, thumb_url, duration_ms, width, height, audio_url, subtitle_url, model_id, params(JSON), status(`queued/generating/succeeded/failed/cancelled`), error_msg, retry_count, generated_in_run_id |
- 索引:`idx_project_id`, `idx_storyboard_id`, `idx_status`, `idx_model_id`

#### 2.9 完整视频域

#### `full_videos`
| 列 | 说明 |
|---|---|
| id, project_id, name, version, timeline(JSON: tracks/clips/transitions/captions/bgm/voiceover), output_url, thumb_url, cover_url, duration_ms, status(`draft/rendering/rendered/in_review/published/off`), render_progress(0-100), error_msg |
- 索引:`idx_project_id`, `idx_status`
- `timeline` 示例:
  ```json
  {
    "video_tracks": [
      {"clips": [{"short_video_id": 12, "in_ms": 0, "out_ms": 12000, "transition_out": "fade"}]}
    ],
    "audio_tracks": [{"clips": [{"audio_url":"...", "in_ms":0, "out_ms":60000, "volume":0.5}]}],
    "captions": [{"text":"...", "start_ms":0, "end_ms":2000, "style":"default"}],
    "watermark": {"url":"...","position":"br"}
  }
  ```

#### 2.10 审核 / 发布域

#### `review_flows` 审核流定义
- id, name, description, target_type(`full_video`), nodes(JSON 节点序列), enabled, is_default

#### `review_nodes` (内嵌于 review_flows.nodes JSON 时可省略,这里给出关系版)
- id, flow_id, step_no, name, approver_type(`user/role`), approver_value, allow_timeout_pass(0/1), timeout_hours

#### `review_records` 审核记录
| 列 | 说明 |
|---|---|
| id, target_type(`full_video`), target_id, flow_id, current_step, status(`pending/approved/rejected/withdrawn`), submitted_by, finished_at |
- 索引:`idx_target(target_type, target_id)`, `idx_status`

#### `review_node_records` 节点级记录
- id, review_record_id, step_no, approver_id, action(`approve/reject_back/reject_final/timeout_pass`), comment, acted_at

#### `publishes`
- id, full_video_id, published_by, published_at, status(`on/off`), watermark_config(JSON), download_count, play_count

#### 2.11 模型域

#### `models`
| 列 | 说明 |
|---|---|
| id, code(UNIQUE), name, type(`text/image/video/audio`), provider, endpoint, api_key_encrypted, default_params(JSON), capability_tags(JSON), enabled, priority, max_qps, health_check_url, last_health_at, last_health_status |
- 索引:`uk_code`, `idx_type_enabled`

#### `model_invocations` 单次调用流水(高频)
| 列 | 说明 |
|---|---|
| id, model_id, user_id, dept_id, project_id, biz_type(`text/image/video/audio`), biz_ref(`step_run_id` 或 `short_video_id` 等), input_tokens, output_tokens, units, duration_ms, cost, status, error_code, started_at, ended_at |
- 索引:`idx_model_started(model_id, started_at)`, `idx_user_started(user_id, started_at)`, `idx_dept_started(dept_id, started_at)`
- 建议按 `started_at` 月分区(PARTITION BY RANGE COLUMNS),或定期归档到归档表

#### 2.12 计费域

#### `billing_quotas` 额度
| 列 | 说明 |
|---|---|
| id, scope_type(`dept/user`), scope_id, model_id(NULL=全局), period(`monthly/total`), metric(`tokens/calls/video_seconds`), quota_value, used_value, reset_at, enabled |
- 索引:唯一 `uk_quota_scope_model_period(scope_type, scope_id, model_id, period, metric)`

#### `billing_daily` 每日聚合
| 列 | 说明 |
|---|---|
| id, stat_date(DATE), model_id, dept_id, user_id, calls, input_tokens, output_tokens, units, cost |
- 索引:`uk_stat(stat_date, model_id, dept_id, user_id)`

#### `model_pricing` 单价配置
- id, model_id, metric, price_per_unit, effective_from, effective_to, currency(默认 CNY)

#### 2.13 流水线域

#### `pipelines`
| 列 | 说明 |
|---|---|
| id, project_id(NULL=全局模板), name, description, dag(JSON: nodes+edges), is_template, enabled |

#### `pipeline_runs`
| 列 | 说明 |
|---|---|
| id, pipeline_id, project_id, triggered_by, trigger_type(`manual/template/api`), input(JSON), output(JSON), status(`queued/running/succeeded/failed/cancelled`), started_at, ended_at, error_msg |

#### `step_runs`
| 列 | 说明 |
|---|---|
| id, run_id, node_id, node_type, model_id, input(JSON), output(JSON), status, attempt, started_at, ended_at, error_msg |
- 索引:`idx_run(run_id, started_at)`

#### 2.14 系统域

#### `audit_logs`
- id, user_id, action, resource_type, resource_id, before(JSON), after(JSON), ip, ua, request_id, created_at
- 按月分区或定期归档

#### `async_tasks`(可选,如不用 RabbitMQ-only 模式)
- id, type, payload(JSON), priority, status, retry, max_retry, available_at, locked_by, locked_at, error_msg, created_at

#### `sys_dicts` 字典
- id, type, code, name, value, sort, status



### 6.5 命名规约

| 类型 | 规约 | 示例 |
|---|---|---|
| 表名 | 小写复数 | `short_videos` |
| 主键 | `id` BIGINT UNSIGNED | |
| 外键 | `<table_singular>_id` | `project_id` |
| 时间 | 小写蛇形 _at 结尾 | `created_at` |
| 布尔 | `is_xxx` / `enabled` TINYINT | `is_default` |
| 枚举 | TINYINT + 业务层常量(避免 ENUM) | `status` |
| JSON | 字段名以 `_json` 后缀或使用 JSON 类型 | `dag` JSON |
| 唯一索引 | `uk_xxx` | `uk_code` |
| 普通索引 | `idx_xxx` | `idx_project_status` |

### 6.6 容量估算

| 表 | 月增 | 一年量 | 备注 |
|---|---|---|---|
| projects | < 100 | 1k | |
| scripts | < 100 | 1k | |
| episodes | < 5k | 60k | 1 剧本 ~50 集 |
| storyboards | < 50k | 600k | 1 集 ~10 镜 |
| images | < 100k | 1.2M | 1 镜 2-3 张 |
| short_videos | < 30k | 360k | |
| full_videos | < 1.5k | 18k | 日产 < 50 |
| model_invocations | < 200k | 2.4M | 含重试 |
| audit_logs | < 200k | 2.4M | |

数据量在单库压力可控范围内,不需要分库分表;高频写表分区即可。



## 7. 部署拓扑建议

### 7.1 开发环境（单机）

```
[Single Host]
  ├─ Go server  (:8080)
  ├─ Go worker  (同机, 1~2 实例)
  ├─ MySQL 8.x  (:3306)
  ├─ Redis 6.x+ (:6379)
  └─ Local Storage (./var/uploads)
```

- 配置文件 `configs/config.yaml` 使用环境变量覆盖，开发时可直接修改 YAML。
- Worker 与 Server 可分别通过 `go run cmd/server/main.go` 和 `go run cmd/worker/main.go` 启动。

### 7.2 生产环境（容器化 / K8s）

```
[Ingress / Nginx]
       |
   +---+---+
   |       |
[Server Pod xN]   [Worker Pod xM]
   |                |
[MySQL Primary]  [Redis Cluster]
[MySQL Replica]    |
[MinIO / OSS] <---+
```

| 组件 | 建议配置 | 说明 |
|------|----------|------|
| Server | Deployment, 2+ replicas, HPA CPU>70% | 无状态，可快速水平扩展 |
| Worker | Deployment, 3+ replicas, 或 KEDA 按队列长度伸缩 | 长耗时任务，资源密集型 |
| MySQL | 主从 + 自动备份 | 业务数据持久化 |
| Redis | 哨兵或 Cluster 模式 | 队列 + 缓存 + WS 桥接，高可用 |
| Storage | MinIO（私有）或阿里云 OSS | 大文件存储，CDN 加速 |
| FFmpeg | Worker 镜像内置静态编译版 | 避免宿主机依赖 |
| 监控 | Prometheus + Grafana + Alertmanager | 采集 Go runtime、Asynq、MySQL、Redis 指标 |
| 日志 | ELK / Loki | 结构化 JSON 日志收集 |

### 7.3 配置管理

- 基础配置：`configs/config.yaml`（提交仓库，不含密钥）。
- 敏感配置：通过环境变量注入（`JWT_SECRET`、`CRYPTO_KEY`、`MYSQL_DSN`、`OSS_SECRET_KEY`）。
- Viper 支持 `${VAR:default}` 语法，便于本地默认值 + 生产环境变量覆盖。

### 7.4 启动顺序与依赖检查

1. MySQL、Redis 就绪。
2. Server 启动：加载配置 -> 连接 DB/Redis -> 初始化 Casbin -> 加载模型 Adapter -> 启动 HTTP Server -> 启动 WS Hub。
3. Worker 启动：加载配置 -> 连接 DB/Redis -> 初始化 Casbin -> 加载模型 Adapter -> 注册 DAG NodeHandler -> 注册 Asynq Handler -> 启动 Asynq Server。
4. 健康检查：`GET /healthz` 返回 `{"status":"ok"}`，用于 K8s liveness/readiness probe。

---

## 附录：代码路径索引

| 模块 | 路径 |
|------|------|
| Server 入口 | `cmd/server/main.go` |
| Worker 入口 | `cmd/worker/main.go` |
| HTTP 服务器与路由 | `internal/server/server.go` |
| 配置定义 | `internal/conf/conf.go` |
| 核心模型 | `internal/model/model.go`、`internal/model/extra.go` |
| Handler 聚合 | `internal/handler/handlers.go`、`internal/handler/handler.go` |
| Service 聚合 | `internal/service/service.go` |
| 异步生成与任务处理 | `internal/service/generation.go` |
| 完整视频合成 | `internal/service/full_video.go` |
| Pipeline CRUD | `internal/service/pipeline_service.go` |
| DAG Runner | `internal/pipeline/runner.go` |
| DAG 节点处理器 | `internal/pipeline/node_handlers.go` |
| Asynq 桥接 | `internal/pipeline/asynq_handler.go` |
| 节点与任务注册表 | `internal/pipeline/registry.go` |
| WebSocket Hub | `pkg/ws/hub.go` |
| 任务队列 | `pkg/queue/queue.go` |
| 对象存储 | `pkg/storage/storage.go` |
| 模型适配器接口 | `internal/adapter/adapter.go` |
| 数据仓库 | `internal/repo/repo.go` |
| 中间件 | `internal/middleware/middleware.go` |
| 错误码 | `pkg/errcode/errcode.go` |
| 配置文件示例 | `configs/config.yaml` |
