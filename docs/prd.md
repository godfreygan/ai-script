# AI 短剧视频生成平台 · 需求规格说明书 v1.0

> 文档状态:Draft v1.0  
> 最近更新:2026-05-12  
> 负责人:待指定  
> 说明：本文档由 `requirements.md` + `mvp-plan.md` 合并而成。
> 适用范围:公司内部研发、产品、UI、QA、运维

---

## 1. 项目背景

公司需要把"剧本 → 完整短剧视频"的制作链路 AI 化、流水线化,以提升内部短剧 / 营销视频 / 教学视频的产出效率。本平台面向公司内部使用,统一管理从剧本到成品视频的全生命周期,同时让"原子能力"(如只生图、只生短视频)也能独立使用。

## 2. 业务目标与非目标

### 2.1 目标
- 一站式管理剧本、提示词、分镜、风格、图片、短视频、完整视频。
- 支持自定义流水线编排,每一步可独立调用、可选模型。
- 多级可配置审核,发布到站内播放/下载。
- 模型集中管理、按部门/个人额度控制。
- 单机型公有云部署即可上线,< 100 人使用,日产视频 < 50 部。

### 2.2 非目标(明确不做)
- 对外多租户 SaaS、注册付费、跨公司协作。
- 抖音/YouTube/B 站等第三方平台直接发布(导出本地 / 站内播放即可)。
- 大规模 GPU 集群、分布式渲染。
- 真实计费交易(仅做额度统计与拦截)。

## 3. 角色与权限矩阵

| 角色 | 项目 | 剧本 | 提示词 | 分镜/风格 | 图片 | 短视频 | 完整视频 | 审核 | 发布 | 用户/角色 | 模型/计费 |
|------|------|------|--------|-----------|------|--------|----------|------|------|-----------|-----------|
| 超级管理员 | RWX | RWX | RWX | RWX | RWX | RWX | RWX | RWX | RWX | RWX | RWX |
| 部门管理员 | RW(本部门) | R | R | R | R | R | R | - | - | RW(本部门) | R |
| 制作人 | RWX(本人) | RWX | RWX | RWX | RWX | RWX | RWX | 提交 | - | - | R |
| 编辑/分镜师 | R | RW | RW | RW | RW | R | R | - | - | - | R |
| 审核员 | R | R | R | R | R | R | R | RWX | - | - | - |
| 运营 | R | R | R | R | R | R | R | - | RWX | - | - |
| 访客 | R(已发布) | - | - | - | - | - | R(已发布) | - | - | - | - |

> R 读 / W 写 / X 删 / RWX 全部

数据权限:`本人`、`本部门`、`全部` 三级,可在角色定义里指定。

## 4. 核心业务流程

```
[项目]
   │  创建项目(选模板/选默认流水线)
   ▼
[剧本上传] ──► AI 自动拆分集 ──► 人工微调分集边界
   │
   ▼
[分集提示词] ──► AI 生成 ──► 人工编辑/多版本
   │
   ▼
[分镜] ──► AI 生成(基于提示词) ──► 人工微调
   │     ┌──► [风格]:可对一个或多个分镜套同一风格
   │     │
   ▼     ▼
[图片生成] ──► 文/图生图(基于分镜+风格)
   │   或 用户手动上传图
   ▼
[短视频生成 ~15s] ──► 图生视频 / 文生视频
   │   或 用户手动上传素材
   ▼
[完整视频] ──► 轻量在线剪辑器(时间轴+转场+字幕+BGM+配音)
   │
   ▼
[审核] ──► 可配置多级审核(通过 / 退回 / 驳回)
   │
   ▼
[发布] ──► 站内播放 / 下载
```

**关键性质**:
- 每个粗体步骤都是独立原子能力,有独立菜单入口、独立 API,可绕过上一步直接使用。
- 用户可在"流水线管理"中拖拽 DAG,把这些原子能力按需串起来。
- 每个节点可选择一个已注册的模型(同类型可换)。

## 5. 功能模块详述

### 5.1 项目管理
- **实体**:Project(id, code, name, description, owner_id, dept_id, status, default_pipeline_id, cover_url, tags, created_at...)
- **状态机**:`draft → in_production → in_review → published → archived`
- **操作**:
  - 项目 CRUD、归档、克隆(连同剧本/分镜等深拷贝可选)
  - 成员管理(项目级 ACL 覆盖角色权限)
  - 项目主页:剧本数、分镜数、视频数、近 7 日产量
  - 资源树视图:剧本 → 分集 → 分镜 → 图片/视频

### 5.2 剧本管理
- **实体**:Script(id, project_id, name, source_url, raw_text, version, status, meta_json)
- **支持格式**:txt / docx / pdf / markdown(后端用 `gopdf` / `docx` 解析)
- **能力**:
  - 上传 + 解析正文
  - **AI 自动拆分集**:调用文本模型,输出 `[{ep: 1, title: "...", begin: 0, end: 1200, summary: "..."}]`,人工可拖拽边界修订
  - 版本管理:每次保存生成 ScriptVersion(`diff` 保留)、可比较、可回滚
  - 元数据:题材标签、风格倾向、目标时长、目标观众、备注
  - 全文检索(MySQL 5.7+ ngram fulltext,或后期接 ES)

### 5.3 分集提示词管理
- **实体**:Episode(id, script_id, ep_no, title, summary, raw_segment) + EpisodePrompt(id, episode_id, version, content_json, model_id, status)
- **content_json 字段**:
  ```json
  {
    "scene_brief": "...",
    "key_scenes": ["...", "..."],
    "characters": [{"name": "...", "description": "..."}],
    "shot_suggestions": ["远景:...","近景:..."]
  }
  ```
- **能力**:
  - 一键生成(选文本模型 + 提示词模板)
  - 多版本(每次重生为新版本)、版本对比、设为当前
  - 人工编辑,支持 Markdown

### 5.4 分镜 / 风格管理
- **Storyboard 字段**:shot_no, shot_type(远/中/近/特/大特), camera_motion(推/拉/摇/移/跟/固定), scene_desc, characters, action, dialogue, duration_sec, notes
- **StoryboardStyle 字段**:name, art_style(写实/水墨/二次元/赛博...), color_tone, lighting, reference_images[], lora_id(可选), description
- **关系**:Storyboard ↔ Style 多对多,支持"一个风格套到多个分镜"
- **能力**:
  - 基于分集提示词一键生成 N 个分镜
  - 单独编辑分镜
  - 风格库(项目级 + 公共),可复用、可一键替换
  - 批量套用风格(选多个分镜 + 选风格 = 一键应用)

### 5.5 短视频管理(单段约 15s)
- **来源**:
  1. 分镜 + 风格 → 文/图生图 → 图生视频(完整链)
  2. 分镜 + 风格 → 直接文/图生视频(跳过单独生图)
  3. 上传图片 + 提示词 → 图生视频
  4. 上传视频 → 纯素材入库
- **字段**:src_type(generated/uploaded), storyboard_id?, prompt, generated_images[], video_url, duration, audio_url, subtitle_url, model_id, params, status, retries
- **状态机**:`queued → generating → succeeded / failed / cancelled`
- **能力**:
  - 重新生成、取消、收藏、打标(角色/场景标签)
  - 缩略图自动抽帧
  - 留痕调用参数(便于复盘)

### 5.6 完整视频(轻量在线剪辑器)
- **时间轴**:1 ~ N 个视频轨、N 个音频轨、1 个字幕轨
- **可操作**:
  - 拖拽短视频排序、裁切、设置出/入点
  - 转场:预设(淡入淡出、滑动、缩放)
  - 字幕:从短视频字幕导入,或对完整片重做 ASR;支持样式
  - BGM:本地上传或库内素材
  - 配音:支持 TTS 模型;可对完整剧本一次性合成
  - 封面图选定
- **渲染**:前端只做预览(ffmpeg.wasm + canvas),最终交后端 FFmpeg 容器渲染(异步,推送进度)
- **多版本草稿**:同一个项目下可保存多个剪辑草稿

### 5.7 用户管理
- **登录**:系统自有账号(账号/密码),密码 bcrypt 加盐
- **个人中心**:头像、邮箱、手机、所在部门
- **API Token**:个人可生成长期 token 给脚本/SDK 用
- **批量导入**:CSV 导入用户、设默认密码

### 5.8 角色权限管理
- **RBAC**:Role ↔ Permission(resource + action)
- **资源**:project, script, episode, prompt, storyboard, style, image, short_video, full_video, review, publish, user, role, model, billing
- **action**:read, write, delete, manage(管理者动作)
- **数据权限**:`self / dept / all` 三档,通过角色绑定
- **审核流配置**:管理员后台配置多级审核节点,指定审核人/审核角色

### 5.9 模型管理
- **类型**:text / image / video / audio
- **字段**:name, provider(openai/anthropic/azure/aliyun/volcengine/...), endpoint, api_key(加密), default_params(json), capability_tags(支持中文/支持图生视频/支持参考图/...), enabled, priority, health_check_url, max_qps
- **抽象**:Adapter 接口
  ```go
  type Adapter interface {
      Name() string
      Type() ModelType
      Generate(ctx, request) (Response, error)
      Healthcheck() error
      CalcCost(usage) float64
  }
  ```
- **能力**:
  - 模型注册、启停、健康检查、试调(测试一次)
  - 推荐通过 [LiteLLM / One-API](https://github.com/songquanpeng/one-api) 做统一网关(减少自研 Adapter 代码量)

### 5.10 模型计费 / 额度管理
- **额度维度**:tokens / calls / video_seconds 三选一,按 模型 + 部门/个人 设置
- **结算规则**:单价(每千 token、每张图、每秒视频),用量 × 单价 = 费用
- **统计**:每次模型调用落 ModelInvocation 表;按日聚合 BillingDaily(部门/个人/模型)
- **拦截**:超额时返回 4029 错误码,允许超管临时放行
- **报表**:折线图(近 30 日)、表格(按部门导出 Excel)

## 6. 流水线管理(可视化 DAG 编排)

### 6.1 节点(原子能力清单)
| 节点编码 | 名称 | 输入 | 输出 | 默认模型类型 |
|---|---|---|---|---|
| `script.split` | 剧本拆分集 | script_id | episodes[] | text |
| `prompt.generate` | 生成分集提示词 | episode_id | prompt_id | text |
| `storyboard.generate` | 生成分镜 | prompt_id | storyboards[] | text |
| `style.apply` | 套用风格 | storyboards[], style_id | storyboards_with_style | - |
| `image.generate` | 生成图片 | storyboard_id (+style) | image_id | image |
| `image.upload` | 上传图片 | file | image_id | - |
| `video.generate` | 生成短视频 | image_id / prompt | short_video_id | video |
| `audio.tts` | 文字转语音 | text | audio_id | audio |
| `video.compose` | 合成完整视频 | short_videos[], timeline | full_video_id | - |
| `review.submit` | 提交审核 | full_video_id | review_id | - |
| `human.approve` | 人工节点 | any | any | - |

### 6.2 DAG 模型
```json
{
  "id": "pipeline_xxx",
  "name": "短剧全自动版",
  "nodes": [
    {"id": "n1", "type": "script.split",       "model_id": "m_qwen_max"},
    {"id": "n2", "type": "prompt.generate",    "model_id": "m_gpt4o"},
    {"id": "n3", "type": "storyboard.generate","model_id": "m_gpt4o"},
    {"id": "n4", "type": "image.generate",     "model_id": "m_jimeng",   "params":{"size":"1024x576"}},
    {"id": "n5", "type": "video.generate",     "model_id": "m_kling",    "params":{"duration":15}}
  ],
  "edges": [
    {"from":"n1","to":"n2","mapping":{"episode_id":"$.episodes[*].id"}},
    {"from":"n2","to":"n3","mapping":{"prompt_id":"$.prompt_id"}},
    {"from":"n3","to":"n4","mapping":{"storyboard_id":"$.storyboards[*].id"}},
    {"from":"n4","to":"n5","mapping":{"image_id":"$.image_id"}}
  ]
}
```

### 6.3 运行模型
- `PipelineRun`:一次执行实例,串联多个 `StepRun`
- 长任务:异步队列(RabbitMQ)+ worker,生成类节点单独 queue
- 失败重试:节点级 3 次重试 + 退避
- 可视化进度:前端用 ReactFlow 渲染 DAG + 实时 WebSocket 推送状态
- 人工节点:阻塞直到指定用户在前端 confirm

### 6.4 预置模板(MVP 内置)
- `全自动版`:script.split → prompt → storyboard → image → video → compose
- `半自动版-到分镜`:script.split → prompt → storyboard(人工节点终止)
- `仅生图`:image.generate(单步)
- `仅生短视频`:video.generate(单步)
- `成品剪辑`:video.compose(从素材池)

## 7. 审核与发布

### 7.1 多级审核
- 管理员可在 `审核流配置` 中定义节点序列:`[节点1=编辑组长, 节点2=合规, 节点3=部门主管]`
- 每个节点动作:**通过 / 退回到上节点 / 驳回(回作者)**
- 节点可选自动超时(超时默认通过 or 升级)

### 7.2 发布
- 通过最末节点 → 视频进入"发布池"
- 运营点 `发布` → 站内列表可见,可点播/下载
- 发布后可下架(状态回到 published_off)
- 支持简单水印(项目 logo / 内部标识)

## 8. 非功能性需求

| 维度 | 目标 |
|---|---|
| 可用性 | 99.5%(内部工具) |
| 业务接口 P99 | < 300ms |
| 生成类接口 | 立即返回 task_id,平均生成时间通过队列管控 |
| 单用户并发 | 同时 ≤ 5 个生成任务 |
| 系统并发 | 同时 ≤ 30 个生成任务(< 50/日的常态下完全够) |
| 数据安全 | 模型 API Key 加密(AES-GCM + KMS)、密码 bcrypt、操作审计 |
| 备份 | MySQL 每日全备 + binlog,OSS 默认多副本 |
| 监控 | Prometheus + Grafana,关键指标:QPS、生成任务积压、错误率 |
| 日志 | 结构化 JSON,接 Loki / 阿里云 SLS |

## 9. 兼容性与限制
- **浏览器**:Chrome / Edge 最新两个版本
- **剧本大小**:单文件 ≤ 5 MB,文本字数 ≤ 30 万
- **图片**:单张 ≤ 10 MB,分辨率 ≤ 2K
- **视频**:单段 ≤ 100 MB,完整片 ≤ 1 GB
- **生成时长**:单短视频 ≤ 30s(默认 ~15s)

## 10. 未决项 / 待与业务方再确认
- [ ] **TTS 优先供应商**:Azure / 火山 / 阿里云 / 讯飞,先支持 1 家,其余后期。**默认建议:火山引擎(中文质量好)**
- [ ] **剧本版权水印**:是否要在导出视频和图片上加内部水印 / 隐式水印。**默认建议:加可关闭的项目级 logo 水印**
- [ ] **是否对外开放 OpenAPI**:给其他业务系统调用原子能力。**默认建议:第二期开放**
- [ ] **第一期 MVP 范围**:见下文 MVP 实施计划，需产品确认

## 11. 术语表
- **分集**:剧本被拆分后的逻辑章节,一集对应一组提示词、一组分镜、N 个短视频
- **分镜**:一集中的一个具体镜头,对应一张图 / 一段短视频
- **风格**:画面美术风格,可跨分镜复用
- **流水线 (Pipeline)**:有向无环图(DAG),把多个原子能力串/并联起来
- **额度**:某部门/个人在某模型上可消费的预算上限

---

# AI 短剧视频生产平台 · MVP 实施计划

> 版本：v1.0  
> 适用：内部团队工具，<100 用户，日生成视频 < 50 部  
> 周期：5 个 Sprint × 2 周 = 10 周（约 2.5 个月）

---

## 1. 目标与范围

### 1.1 MVP 必须达成
1. 走通端到端流程：登录 → 项目 → 剧本 → 提示词 → 分镜 → 图片 → 短视频 → 完整视频 → 审核 → 站内发布。
2. 每一步均可独立调用（HTTP API + 前端单页面入口）。
3. 提供可视化流水线编排器，支持自定义 DAG 串联以上能力。
4. 模型管理：可在系统内注册任意 OpenAI 兼容模型（含文本/图像/视频/音频），并在每个节点指定具体模型。
5. RBAC：7 个内置角色 × 30+ 权限点，部门级数据隔离。
6. 单机 docker-compose 部署可用；MySQL + Redis 单实例。

### 1.2 MVP 不做（推迟）
- 多租户（仅本团队使用）
- 模型自助调参 / Fine-tune 入口
- 复杂剪辑（多轨道时间线、转场、滤镜）→ 仅做剪入剪出 + 拼接 + 配音 + 字幕
- 自动伸缩与多节点（单机即可满足 < 50 视频/日）
- 外部 OpenAPI（v1.1 再开放）
- 二级缓存、读写分离、分库分表
- 完整 i18n（中文优先）

---

## 2. 团队配置建议

| 角色 | 人数 | 投入 | 主要职责 |
|------|------|------|----------|
| 产品 / 项目经理 | 1 | 50% | 需求澄清、Sprint 评审、上线节奏 |
| 后端工程师 | 2 | 100% | API / 调度 / 模型适配 / 流水线引擎 / 数据库 |
| 前端工程师 | 1~2 | 100% | 15 个页面 + ReactFlow 编排器 |
| 测试工程师 | 1 | 50% | 用例、回归、压测 |
| UI / 视觉 | 1 | 25% | 主题色、关键页交互细节 |
| 运维 | 1 | 20% | 服务器、对象存储、模型网关、日志告警 |

合计：约 **4 ~ 5 FTE × 10 周 ≈ 40 ~ 50 人周**。

---

## 3. Sprint 规划

> 命名约定：S1~S5 为正式 Sprint；S0 为已完成的项目骨架（即当前仓库交付的 D 阶段产物）。

### S0 · 项目骨架（已完成）
- ✅ 仓库结构、Go module、前端脚手架
- ✅ DDL（27 张表 + 种子）
- ✅ HTTP 框架 / 中间件 / 鉴权骨架
- ✅ Asynq worker 骨架 + 模型 Adapter 接口
- ✅ React 路由 / 布局 / API client / 流水线编辑器原型
- ✅ docker-compose（mysql + redis + backend + worker + frontend）

---

### Sprint 1 · 账号 + 项目 + 模型（第 1~2 周）

**目标**：基础设施就绪，可登录、可建项目、可注册模型。

| # | 任务 | 负责 | 工作量 |
|---|------|------|--------|
| 1.1 | 用户 / 部门 CRUD + 密码重置 | 后端 A | 3d |
| 1.2 | 角色 / 权限矩阵 + Casbin 接入 | 后端 A | 3d |
| 1.3 | 登录 / 登出 / 刷新 token / 鉴权中间件 | 后端 A | 2d |
| 1.4 | 项目 CRUD + 成员 + 软删除 | 后端 B | 3d |
| 1.5 | 模型管理 CRUD + api_key AES 加密 + 健康检测按钮 | 后端 B | 4d |
| 1.6 | 模型 Adapter 注册中心 + LiteLLM 实现 | 后端 B | 2d |
| 1.7 | 前端：登录、布局、用户/角色/项目/模型 5 个页面 | 前端 | 8d |
| 1.8 | docker-compose 联调 + 部署文档 | 运维 | 2d |
| 1.9 | 单元测试 + Postman 集合 | QA | 3d |

**验收**
- ✅ 浏览器登录 → 进入工作台
- ✅ 管理员可创建用户、分配角色、限定数据范围
- ✅ 创建一个项目并加入成员
- ✅ 注册一个 OpenAI 兼容文本模型并通过健康检测
- ✅ `docker-compose up` 一键拉起

---

### Sprint 2 · 剧本 + 提示词（第 3~4 周）

**目标**：上传剧本 → AI 拆分分集 → 生成提示词。

| # | 任务 | 负责 | 工作量 |
|---|------|------|--------|
| 2.1 | 剧本上传 (.txt/.md/.docx) + 解析存储 | 后端 A | 3d |
| 2.2 | 剧本版本管理 (script_versions) | 后端 A | 2d |
| 2.3 | `POST /scripts/{id}:split-episodes` 异步任务实现 | 后端 A | 3d |
| 2.4 | 分集 episode CRUD + 人工修订 | 后端 A | 2d |
| 2.5 | `POST /episodes/{id}/prompts:generate` 异步任务 | 后端 B | 3d |
| 2.6 | episode_prompts 增删改 + 锁定 | 后端 B | 2d |
| 2.7 | Asynq 任务结果回写 + step_runs 状态机 | 后端 B | 3d |
| 2.8 | WebSocket `/ws/pipeline-runs/{id}` 推送骨架 | 后端 B | 2d |
| 2.9 | 前端：剧本管理、提示词编辑（含富文本对照） | 前端 | 8d |
| 2.10 | 集成测试（端到端跑通文本侧） | QA | 3d |

**验收**
- ✅ 上传剧本 → 一键 AI 拆分为 N 集 → 列表展示
- ✅ 选中分集 → 一键生成提示词 → 人工编辑保存
- ✅ 异步任务进度通过 WebSocket 实时刷新
- ✅ 任意中间状态可手动调整后继续后续步骤

---

### Sprint 3 · 分镜 + 图片 + 短视频（第 5~6 周）

**目标**：从提示词生成分镜，从分镜生成图片，从图片生成短视频。

| # | 任务 | 负责 | 工作量 |
|---|------|------|--------|
| 3.1 | 分镜 storyboard 列表 / 详情 / CRUD | 后端 A | 3d |
| 3.2 | 风格预设 styles + storyboard_styles 关系表 | 后端 A | 2d |
| 3.3 | `POST /storyboards:generate` + `:apply-style` | 后端 A | 3d |
| 3.4 | 图像模型 Adapter (Stable Diffusion / DALL·E / 通义万相) | 后端 B | 3d |
| 3.5 | `POST /images:generate` + 上传 OSS/COS + images 表 | 后端 B | 3d |
| 3.6 | 视频模型 Adapter (Sora / Kling / Runway / 即梦) | 后端 B | 4d |
| 3.7 | `POST /short-videos:generate` (图生视频 + 文生视频) | 后端 B | 3d |
| 3.8 | 对象存储 Storage 接口实现 (阿里 OSS / 腾讯 COS) | 后端 B | 2d |
| 3.9 | 前端：分镜、风格、图片、短视频 4 个页面 + 预览组件 | 前端 | 10d |
| 3.10 | 模型调用日志 model_invocations 写入 + 用量统计 | 后端 A | 2d |
| 3.11 | 回归测试 + 模型切换测试 | QA | 3d |

**验收**
- ✅ 从一条提示词 → 生成 8~12 个分镜
- ✅ 套用「赛博朋克」风格 → 分镜风格字段批量更新
- ✅ 每个分镜可生成 1~4 张图片，支持手动上传替换
- ✅ 选定图片 → 生成 15s 短视频，在线预览
- ✅ 在同一个步骤切换不同模型（如 Kling 与 Sora），可正常运行

---

### Sprint 4 · 完整视频 + 流水线引擎（第 7~8 周）

**目标**：把多个短视频合并为完整剧集；流水线 DAG 引擎正式可用。

| # | 任务 | 负责 | 工作量 |
|---|------|------|--------|
| 4.1 | FFmpeg worker 容器化 (cgo / 子进程) | 后端 A | 3d |
| 4.2 | 短视频合并 + 转场 + 标准化输出 | 后端 A | 3d |
| 4.3 | TTS 适配 + 字幕生成 + 时间轴对齐 | 后端 A | 4d |
| 4.4 | 轻量在线剪辑（剪入剪出 + 顺序拖拽） | 前端 | 6d |
| 4.5 | 流水线 DAG 拓扑排序 + 参数映射引擎 | 后端 B | 4d |
| 4.6 | 节点失败重试 + 中断恢复 + 人工节点暂停/继续 | 后端 B | 4d |
| 4.7 | 流水线模板：剧本→提示词→分镜→图→片 | 后端 B | 2d |
| 4.8 | `POST /pipelines/{id}:run` + run/step 详情查询 | 后端 B | 2d |
| 4.9 | 前端：完整视频页 + ReactFlow 编辑器联调 | 前端 | 8d |
| 4.10 | 流水线 e2e 测试（成功 / 失败 / 取消 / 重试） | QA | 4d |

**验收**
- ✅ 把若干 15s 短视频拼为 3 分钟成片，含 TTS 配音 + 字幕
- ✅ 在编排器中拖拽节点，保存后点击运行，可在「运行历史」看到逐步状态
- ✅ 故意让某节点失败，可在前端重试该节点续跑
- ✅ 流水线节点可单独选择模型（如「图片生成」节点切到 SD）

---

### Sprint 5 · 审核 + 发布 + 计费 + 上线（第 9~10 周）

**目标**：合规闭环，准备首批用户上线。

| # | 任务 | 负责 | 工作量 |
|---|------|------|--------|
| 5.1 | review_flows / review_nodes 配置 | 后端 A | 2d |
| 5.2 | `POST /reviews` 提交 + 审核记录 + 驳回回滚 | 后端 A | 3d |
| 5.3 | 发布站内分发：播放页 + 下载 + 签名 URL | 后端 A | 3d |
| 5.4 | 计费 quota 校验中间件 + 超额拦截 | 后端 B | 2d |
| 5.5 | 计费日统计 billing_daily + 调用明细 | 后端 B | 2d |
| 5.6 | 操作审计 audit_logs 全量接入 | 后端 B | 2d |
| 5.7 | Prometheus 指标 + 健康检查 + Zap 结构化日志 | 后端 B | 2d |
| 5.8 | 前端：审核、发布、计费 3 个页面 + 工作台数据真实化 | 前端 | 8d |
| 5.9 | 压测：50 并发 + 每分钟 5 个生成任务 | QA | 3d |
| 5.10 | 用户手册 + 上线 Runbook + 灾难恢复演练 | PM | 3d |
| 5.11 | 灰度 → 全量上线 | 全员 | 2d |

**验收**
- ✅ 一个完整视频走完二级审核后可发布
- ✅ 普通用户点击播放页可正常观看 / 下载
- ✅ 操作员超出额度时被拦截并返回 40290
- ✅ 工作台数据为真实统计
- ✅ Grafana 看板可观察 QPS / P95 / 失败率 / 队列堆积
- ✅ 首日生产环境跑通 5 部短剧

---

## 4. 演进路线（MVP 之后）

| 版本 | 目标 | 预估 |
|------|------|------|
| v1.1 | 外部 OpenAPI + Webhook 回调 + 配额体系 | 2 周 |
| v1.2 | 多轨道剪辑（视频 / 音频 / 字幕轨）+ 转场库 | 4 周 |
| v1.3 | 模型自助调参 / Prompt 模板市场 | 3 周 |
| v1.4 | 多节点部署 + 任务优先级队列 + S3 兼容存储 | 3 周 |
| v2.0 | 多租户改造 + 计费账单 + 角色细分到资源级 | 6 周 |

---

## 5. 风险登记册

| # | 风险 | 概率 | 影响 | 缓解措施 |
|---|------|------|------|----------|
| R1 | 视频模型（Sora/Kling）调用配额 / 速率被卡 | 高 | 高 | 多模型冗余 + 限速排队 + 监控配额预警 |
| R2 | 模型生成结果不稳定 / 质量参差 | 高 | 中 | 重试机制 + 多次生成挑选 + 人工节点兜底 |
| R3 | FFmpeg 在 worker 容器内性能不足 | 中 | 中 | 单独的 video-worker pool；必要时 GPU 加速 |
| R4 | OSS/COS 流量费用超预算 | 中 | 中 | 私有桶 + 签名 URL + 7 天生命周期 + 用量告警 |
| R5 | RBAC 数据范围漏判导致越权 | 低 | 高 | 中间件兜底 + 每个 List 查询强制注入 scope 条件 + e2e 越权用例 |
| R6 | 异步任务丢失（Redis 重启） | 低 | 中 | Redis AOF 持久化 + Asynq retention=72h + 死信队列告警 |
| R7 | api_key 泄露 | 低 | 高 | AES-256-GCM + 操作审计 + 角色隔离 + 定期轮换 |

---

## 6. 关键依赖

| 依赖 | 提供方 | 何时需要 | 准备状态 |
|------|--------|---------|---------|
| MySQL 8.0 实例（≥ 16 GB） | 运维 | S1 第 1 天 | ⬜ |
| Redis 7（≥ 4 GB） | 运维 | S1 第 1 天 | ⬜ |
| 阿里云 OSS / 腾讯 COS 桶 + AK | 运维 | S3 第 1 天 | ⬜ |
| LiteLLM 网关或直连模型供应商 | 后端 + 运维 | S1 第 5 天 | ⬜ |
| 视频模型试用配额（Sora / Kling / 即梦） | PM 对接 | S3 第 1 天 | ⬜ |
| 域名 + HTTPS 证书 | 运维 | S5 第 5 天 | ⬜ |
| 灰度用户清单（5~10 人） | PM | S5 第 8 天 | ⬜ |

---

## 7. 验收里程碑

- **M1（S1 末）**：账号体系 + 项目 + 模型联通，「基础平台可用」
- **M2（S2 末）**：剧本到提示词链路打通，「文本侧 MVP」
- **M3（S3 末）**：图像与短视频生成可用，「多模态 MVP」
- **M4（S4 末）**：流水线编排可用，「自定义生产线 MVP」
- **M5（S5 末）**：审核发布闭环 + 计费 + 观测，「正式上线」

---

## 8. 待用户确认项

> 以下 4 项默认值已落在需求文档与代码里，开发期间任何时间确认即可，不影响 Sprint 1~3 推进，最迟在 Sprint 4 前敲定。

1. **TTS 供应商优先级**（默认：阿里云 / 腾讯云 / OpenAI TTS）
2. **是否给完整视频打默认水印**（默认：否，可在项目级开启）
3. **是否开放外部 OpenAPI 给其他系统**（默认：MVP 不开放，v1.1 再做）
4. **首版灰度用户范围**（默认：内容部 5 人 + 产品 2 人）
