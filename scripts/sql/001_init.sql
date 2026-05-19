-- =============================================================
-- AI 短剧视频生成平台 · 初始化 DDL  v1.0
-- MySQL 8.0+ , InnoDB, utf8mb4
-- =============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;
SET time_zone = '+08:00';

-- -------------------------------------------------------------
-- 1. 用户域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `departments` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT             COMMENT '主键 ID',
  `name`       VARCHAR(64)     NOT NULL                            COMMENT '部门名称',
  `parent_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0                  COMMENT '父部门 ID,0=顶级',
  `path`       VARCHAR(255)    NOT NULL DEFAULT ''                 COMMENT '层级路径,如 /1/3/7/,便于按子树查询',
  `sort`       INT             NOT NULL DEFAULT 0                  COMMENT '同级排序,越小越靠前',
  `status`     TINYINT         NOT NULL DEFAULT 1                  COMMENT '状态:1=启用 2=禁用',
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at` DATETIME(3)     NULL                                COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_path`      (`path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='部门';

CREATE TABLE IF NOT EXISTS `users` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT         COMMENT '主键 ID',
  `username`       VARCHAR(64)     NOT NULL                        COMMENT '登录账号,全局唯一',
  `password_hash`  VARCHAR(128)    NOT NULL                        COMMENT '密码哈希(bcrypt 等),不存明文',
  `nickname`       VARCHAR(64)     NOT NULL DEFAULT ''             COMMENT '昵称/显示名',
  `email`          VARCHAR(128)    NOT NULL DEFAULT ''             COMMENT '邮箱',
  `phone`          VARCHAR(20)     NOT NULL DEFAULT ''             COMMENT '手机号',
  `avatar_url`     VARCHAR(512)    NOT NULL DEFAULT ''             COMMENT '头像 URL',
  `dept_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0              COMMENT '所属部门 ID,0=未分配',
  `status`         TINYINT         NOT NULL DEFAULT 1              COMMENT '状态:1=正常 2=禁用',
  `last_login_at`  DATETIME(3)     NULL                            COMMENT '最近一次登录时间',
  `last_login_ip`  VARCHAR(64)     NOT NULL DEFAULT ''             COMMENT '最近一次登录 IP',
  `created_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`     DATETIME(3)     NULL                            COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_email`   (`email`),
  KEY `idx_dept_id` (`dept_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户';

CREATE TABLE IF NOT EXISTS `user_api_tokens` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT          COMMENT '主键 ID',
  `user_id`       BIGINT UNSIGNED NOT NULL                         COMMENT '归属用户 ID',
  `name`          VARCHAR(64)     NOT NULL                         COMMENT 'Token 名称/用途备注',
  `token_hash`    VARCHAR(128)    NOT NULL                         COMMENT 'Token 哈希值,唯一,不存明文',
  `scopes`        JSON            NULL                             COMMENT '授权范围 JSON 数组,如 ["project:read","model:invoke"]',
  `expires_at`    DATETIME(3)     NULL                             COMMENT '过期时间,NULL=永不过期',
  `last_used_at`  DATETIME(3)     NULL                             COMMENT '最近一次使用时间',
  `status`        TINYINT         NOT NULL DEFAULT 1               COMMENT '状态:1=启用 2=禁用',
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`    DATETIME(3)     NULL                             COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_token_hash` (`token_hash`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户 API Token';

-- -------------------------------------------------------------
-- 2. 鉴权域 (RBAC)
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `roles` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT            COMMENT '主键 ID',
  `code`        VARCHAR(64)     NOT NULL                           COMMENT '角色编码,全局唯一,如 admin/editor',
  `name`        VARCHAR(64)     NOT NULL                           COMMENT '角色名称',
  `description` VARCHAR(255)    NOT NULL DEFAULT ''                COMMENT '描述',
  `data_scope`  VARCHAR(16)     NOT NULL DEFAULT 'self'            COMMENT '数据范围:self=仅本人 dept=本部门 all=全部',
  `is_system`   TINYINT         NOT NULL DEFAULT 0                 COMMENT '是否系统内置角色:0=否 1=是(不可删除)',
  `status`      TINYINT         NOT NULL DEFAULT 1                 COMMENT '状态:1=启用 2=禁用',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`  DATETIME(3)     NULL                               COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色';

CREATE TABLE IF NOT EXISTS `permissions` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT            COMMENT '主键 ID',
  `code`        VARCHAR(128)    NOT NULL                           COMMENT '权限编码,格式 resource:action,如 project:read',
  `name`        VARCHAR(128)    NOT NULL                           COMMENT '权限名称',
  `resource`    VARCHAR(64)     NOT NULL                           COMMENT '资源标识,如 project/script/model',
  `action`      VARCHAR(32)     NOT NULL                           COMMENT '动作标识,如 read/write/delete',
  `description` VARCHAR(255)    NOT NULL DEFAULT ''                COMMENT '描述',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_resource_action` (`resource`,`action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='权限点';

CREATE TABLE IF NOT EXISTS `role_permissions` (
  `role_id`       BIGINT UNSIGNED NOT NULL                         COMMENT '角色 ID',
  `permission_id` BIGINT UNSIGNED NOT NULL                         COMMENT '权限点 ID',
  PRIMARY KEY (`role_id`,`permission_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色-权限 关联';

CREATE TABLE IF NOT EXISTS `user_roles` (
  `user_id` BIGINT UNSIGNED NOT NULL                               COMMENT '用户 ID',
  `role_id` BIGINT UNSIGNED NOT NULL                               COMMENT '角色 ID',
  PRIMARY KEY (`user_id`,`role_id`),
  KEY `idx_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户-角色 关联';

CREATE TABLE IF NOT EXISTS `casbin_rule` (
  `id`    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT                  COMMENT '主键 ID',
  `ptype` VARCHAR(100)    NOT NULL DEFAULT ''                      COMMENT '策略类型:p=权限策略 g=角色继承',
  `v0`    VARCHAR(100)    NOT NULL DEFAULT ''                      COMMENT '策略字段 0,通常为 subject(用户/角色)',
  `v1`    VARCHAR(100)    NOT NULL DEFAULT ''                      COMMENT '策略字段 1,通常为 object(资源)',
  `v2`    VARCHAR(100)    NOT NULL DEFAULT ''                      COMMENT '策略字段 2,通常为 action',
  `v3`    VARCHAR(100)    NOT NULL DEFAULT ''                      COMMENT '策略字段 3,扩展位',
  `v4`    VARCHAR(100)    NOT NULL DEFAULT ''                      COMMENT '策略字段 4,扩展位',
  `v5`    VARCHAR(100)    NOT NULL DEFAULT ''                      COMMENT '策略字段 5,扩展位',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_rule` (`ptype`,`v0`,`v1`,`v2`,`v3`,`v4`,`v5`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Casbin 规则表';

-- -------------------------------------------------------------
-- 3. 项目域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `projects` (
  `id`                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT    COMMENT '主键 ID',
  `code`                VARCHAR(64)     NOT NULL                   COMMENT '项目编码,全局唯一',
  `name`                VARCHAR(128)    NOT NULL                   COMMENT '项目名称',
  `description`         TEXT                                       COMMENT '项目描述',
  `owner_id`            BIGINT UNSIGNED NOT NULL                   COMMENT '项目负责人(用户 ID)',
  `dept_id`             BIGINT UNSIGNED NOT NULL DEFAULT 0         COMMENT '归属部门 ID,0=未分配',
  `status`              TINYINT         NOT NULL DEFAULT 1         COMMENT '项目状态:1=draft草稿 2=in_production制作中 3=in_review审核中 4=published已发布 5=archived已归档',
  `default_pipeline_id` BIGINT UNSIGNED NOT NULL DEFAULT 0         COMMENT '默认流水线 ID,0=未设置',
  `cover_url`           VARCHAR(512)    NOT NULL DEFAULT ''        COMMENT '封面图 URL',
  `tags`                JSON            NULL                       COMMENT '标签 JSON 数组,如 ["古装","悬疑"]',
  `created_by`          BIGINT UNSIGNED NOT NULL DEFAULT 0         COMMENT '创建人(用户 ID)',
  `updated_by`          BIGINT UNSIGNED NOT NULL DEFAULT 0         COMMENT '最近更新人(用户 ID)',
  `created_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`          DATETIME(3)     NULL                       COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_owner_id`    (`owner_id`),
  KEY `idx_dept_status` (`dept_id`,`status`),
  KEY `idx_status_created` (`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目';

CREATE TABLE IF NOT EXISTS `project_members` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT        COMMENT '主键 ID',
  `project_id`      BIGINT UNSIGNED NOT NULL                       COMMENT '项目 ID',
  `user_id`         BIGINT UNSIGNED NOT NULL                       COMMENT '成员用户 ID',
  `role_in_project` VARCHAR(32)     NOT NULL DEFAULT 'editor'      COMMENT '项目内角色:producer=制片 editor=编辑 reviewer=审核 viewer=只读',
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '加入时间',
  `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_user` (`project_id`,`user_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目成员';

-- -------------------------------------------------------------
-- 4. 剧本域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `scripts` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT        COMMENT '主键 ID',
  `project_id`      BIGINT UNSIGNED NOT NULL                       COMMENT '归属项目 ID',
  `name`            VARCHAR(128)    NOT NULL                       COMMENT '剧本名称',
  `source_url`      VARCHAR(512)    NOT NULL DEFAULT ''            COMMENT '原始文件 URL(如 docx/txt)',
  `raw_text`        MEDIUMTEXT                                     COMMENT '原始文本内容,支持全文检索',
  `current_version` INT             NOT NULL DEFAULT 1             COMMENT '当前版本号',
  `status`          TINYINT         NOT NULL DEFAULT 1             COMMENT '剧本状态:1=uploaded已上传 2=parsed已解析 3=episode_split已分集',
  `meta`            JSON            NULL                           COMMENT '元数据 JSON,如 {字数,语言,流派}',
  `created_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0             COMMENT '创建人(用户 ID)',
  `updated_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0             COMMENT '最近更新人(用户 ID)',
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`      DATETIME(3)     NULL                           COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`),
  FULLTEXT KEY `ft_raw_text` (`raw_text`) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='剧本';

CREATE TABLE IF NOT EXISTS `script_versions` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT             COMMENT '主键 ID',
  `script_id`  BIGINT UNSIGNED NOT NULL                            COMMENT '所属剧本 ID',
  `version_no` INT             NOT NULL                            COMMENT '版本号,从 1 起递增',
  `content`    MEDIUMTEXT                                          COMMENT '该版本完整内容',
  `diff`       JSON            NULL                                COMMENT '与上一版本的差异 JSON',
  `commit_msg` VARCHAR(255)    NOT NULL DEFAULT ''                 COMMENT '提交说明',
  `created_by` BIGINT UNSIGNED NOT NULL                            COMMENT '创建人(用户 ID)',
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_script_version` (`script_id`,`version_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='剧本版本';

CREATE TABLE IF NOT EXISTS `episodes` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT           COMMENT '主键 ID',
  `script_id`    BIGINT UNSIGNED NOT NULL                          COMMENT '所属剧本 ID',
  `ep_no`        INT             NOT NULL                          COMMENT '分集序号,从 1 起',
  `title`        VARCHAR(255)    NOT NULL DEFAULT ''               COMMENT '分集标题',
  `summary`      TEXT                                              COMMENT '分集概要',
  `raw_segment`  MEDIUMTEXT                                        COMMENT '原文片段(切分后的文本)',
  `char_begin`   INT             NOT NULL DEFAULT 0                COMMENT '在原文中的起始字符位置',
  `char_end`     INT             NOT NULL DEFAULT 0                COMMENT '在原文中的结束字符位置',
  `status`       TINYINT         NOT NULL DEFAULT 1                COMMENT '分集状态:1=正常',
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`   DATETIME(3)     NULL                              COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_script_ep` (`script_id`,`ep_no`),
  KEY `idx_script_id` (`script_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分集';

-- -------------------------------------------------------------
-- 5. 提示词域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `episode_prompts` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT     COMMENT '主键 ID',
  `episode_id`         BIGINT UNSIGNED NOT NULL                    COMMENT '所属分集 ID',
  `version`            INT             NOT NULL DEFAULT 1          COMMENT '提示词版本号,从 1 起递增',
  `is_current`         TINYINT         NOT NULL DEFAULT 0          COMMENT '是否当前生效版本:0=否 1=是(同一 episode 仅一条为 1)',
  `content`            JSON            NULL                        COMMENT '提示词内容 JSON',
  `model_id`           BIGINT UNSIGNED NOT NULL DEFAULT 0          COMMENT '生成此提示词所用模型 ID,0=未关联',
  `generation_params`  JSON            NULL                        COMMENT '生成参数 JSON,如 {temperature,top_p}',
  `status`             TINYINT         NOT NULL DEFAULT 1          COMMENT '状态:1=有效',
  `generated_by`       BIGINT UNSIGNED NOT NULL DEFAULT 0          COMMENT '生成发起人(用户 ID),0=系统',
  `created_at`         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_episode_version` (`episode_id`,`version`),
  KEY `idx_episode_current` (`episode_id`,`is_current`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分集提示词';

-- -------------------------------------------------------------
-- 6. 分镜 / 风格域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `storyboards` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT          COMMENT '主键 ID',
  `episode_id`    BIGINT UNSIGNED NOT NULL                         COMMENT '所属分集 ID',
  `prompt_id`     BIGINT UNSIGNED NOT NULL DEFAULT 0               COMMENT '依据的提示词 ID,0=未关联',
  `shot_no`       INT             NOT NULL                         COMMENT '镜头序号,从 1 起,在同一 episode 内唯一',
  `shot_type`     VARCHAR(32)     NOT NULL DEFAULT 'medium'        COMMENT '景别:wide=远景 medium=中景 close=近景 extreme_close=特写 establishing=定场',
  `camera_motion` VARCHAR(32)     NOT NULL DEFAULT 'static'        COMMENT '运镜方式:pan=摇 zoom=变焦 dolly=推拉 track=跟拍 static=固定',
  `scene_desc`    TEXT                                             COMMENT '场景描述',
  `characters`    JSON                                             COMMENT '出场角色 JSON 数组',
  `action`        TEXT                                             COMMENT '动作/事件描述',
  `dialogue`      TEXT                                             COMMENT '台词内容',
  `duration_sec` INT             NOT NULL DEFAULT 15               COMMENT '镜头预计时长(秒)',
  `notes`         TEXT                                             COMMENT '备注',
  `status`        TINYINT         NOT NULL DEFAULT 1               COMMENT '状态:1=有效',
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`    DATETIME(3)     NULL                             COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_episode_shot` (`episode_id`,`shot_no`),
  KEY `idx_prompt_id` (`prompt_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分镜';

CREATE TABLE IF NOT EXISTS `styles` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT       COMMENT '主键 ID',
  `project_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0            COMMENT '归属项目 ID,0=公共风格',
  `name`             VARCHAR(64)     NOT NULL                      COMMENT '风格名称',
  `art_style`        VARCHAR(64)     NOT NULL DEFAULT ''           COMMENT '美术风格,如 写实/动漫/水彩',
  `color_tone`       VARCHAR(64)     NOT NULL DEFAULT ''           COMMENT '色调,如 暖色/冷色/低饱和',
  `lighting`         VARCHAR(64)     NOT NULL DEFAULT ''           COMMENT '光照,如 自然光/影棚/逆光',
  `reference_images` JSON            NULL                          COMMENT '参考图 URL 列表 JSON',
  `lora_id`          VARCHAR(128)    NOT NULL DEFAULT ''           COMMENT '关联 LoRA 模型标识',
  `description`      TEXT                                          COMMENT '风格描述',
  `status`           TINYINT         NOT NULL DEFAULT 1            COMMENT '状态:1=有效',
  `created_by`       BIGINT UNSIGNED NOT NULL DEFAULT 0            COMMENT '创建人(用户 ID)',
  `created_at`       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`       DATETIME(3)     NULL                          COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='风格';

CREATE TABLE IF NOT EXISTS `storyboard_styles` (
  `storyboard_id` BIGINT UNSIGNED NOT NULL                         COMMENT '分镜 ID',
  `style_id`      BIGINT UNSIGNED NOT NULL                         COMMENT '风格 ID',
  `applied_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '应用时间',
  `applied_by`    BIGINT UNSIGNED NOT NULL DEFAULT 0               COMMENT '应用操作人(用户 ID)',
  PRIMARY KEY (`storyboard_id`,`style_id`),
  KEY `idx_style_id` (`style_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分镜-风格 多对多关联';

-- -------------------------------------------------------------
-- 7. 图片域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `images` (
  `id`                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT   COMMENT '主键 ID',
  `project_id`           BIGINT UNSIGNED NOT NULL                  COMMENT '归属项目 ID',
  `storyboard_id`        BIGINT UNSIGNED NULL                      COMMENT '关联分镜 ID,NULL=未关联分镜',
  `src_type`             VARCHAR(16)     NOT NULL DEFAULT 'generated' COMMENT '来源类型:generated=AI 生成 uploaded=用户上传',
  `url`                  VARCHAR(512)    NOT NULL                  COMMENT '图片 URL',
  `thumb_url`            VARCHAR(512)    NOT NULL DEFAULT ''       COMMENT '缩略图 URL',
  `width`                INT             NOT NULL DEFAULT 0        COMMENT '宽(px)',
  `height`               INT             NOT NULL DEFAULT 0        COMMENT '高(px)',
  `prompt`               TEXT                                      COMMENT '正向提示词',
  `neg_prompt`           TEXT                                      COMMENT '负向提示词',
  `model_id`             BIGINT UNSIGNED NOT NULL DEFAULT 0        COMMENT '生成所用模型 ID,0=非生成',
  `params`               JSON            NULL                      COMMENT '生成参数 JSON,如 {seed,steps,cfg_scale}',
  `status`               TINYINT         NOT NULL DEFAULT 1        COMMENT '状态:1=ok 2=failed',
  `generated_in_run_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0        COMMENT '生成所在流水线运行 ID,0=非流水线触发',
  `created_by`           BIGINT UNSIGNED NOT NULL DEFAULT 0        COMMENT '创建人(用户 ID),0=系统',
  `created_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`           DATETIME(3)     NULL                      COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_project_id`     (`project_id`),
  KEY `idx_storyboard_id`  (`storyboard_id`),
  KEY `idx_status_created` (`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='图片';

-- -------------------------------------------------------------
-- 8. 短视频域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `short_videos` (
  `id`                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT   COMMENT '主键 ID',
  `project_id`           BIGINT UNSIGNED NOT NULL                  COMMENT '归属项目 ID',
  `storyboard_id`        BIGINT UNSIGNED NULL                      COMMENT '关联分镜 ID,NULL=未关联分镜',
  `src_type`             VARCHAR(16)     NOT NULL DEFAULT 'generated' COMMENT '来源类型:generated=AI 生成 uploaded=用户上传',
  `prompt`               TEXT                                      COMMENT '生成提示词',
  `source_image_ids`     JSON            NULL                      COMMENT '依据的源图片 ID 列表 JSON',
  `video_url`            VARCHAR(512)    NOT NULL DEFAULT ''       COMMENT '视频文件 URL',
  `thumb_url`            VARCHAR(512)    NOT NULL DEFAULT ''       COMMENT '缩略图 URL',
  `duration_ms`          INT             NOT NULL DEFAULT 0        COMMENT '时长(毫秒)',
  `width`                INT             NOT NULL DEFAULT 0        COMMENT '宽(px)',
  `height`               INT             NOT NULL DEFAULT 0        COMMENT '高(px)',
  `audio_url`            VARCHAR(512)    NOT NULL DEFAULT ''       COMMENT '音频文件 URL',
  `subtitle_url`         VARCHAR(512)    NOT NULL DEFAULT ''       COMMENT '字幕文件 URL',
  `model_id`             BIGINT UNSIGNED NOT NULL DEFAULT 0        COMMENT '生成所用模型 ID,0=非生成',
  `params`               JSON            NULL                      COMMENT '生成参数 JSON',
  `status`               VARCHAR(16)     NOT NULL DEFAULT 'queued' COMMENT '状态:queued=排队中 running=生成中 succeeded=成功 failed=失败 cancelled=已取消',
  `error_msg`            VARCHAR(512)    NOT NULL DEFAULT ''       COMMENT '失败错误信息',
  `retry_count`          INT             NOT NULL DEFAULT 0        COMMENT '已重试次数',
  `generated_in_run_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0        COMMENT '生成所在流水线运行 ID,0=非流水线触发',
  `created_by`           BIGINT UNSIGNED NOT NULL DEFAULT 0        COMMENT '创建人(用户 ID),0=系统',
  `created_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`           DATETIME(3)     NULL                      COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_project_id`     (`project_id`),
  KEY `idx_storyboard_id`  (`storyboard_id`),
  KEY `idx_status_created` (`status`,`created_at`),
  KEY `idx_model_status`   (`model_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短视频';

-- -------------------------------------------------------------
-- 9. 完整视频域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `full_videos` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT        COMMENT '主键 ID',
  `project_id`      BIGINT UNSIGNED NOT NULL                       COMMENT '归属项目 ID',
  `name`            VARCHAR(128)    NOT NULL                       COMMENT '完整视频名称',
  `version`         INT             NOT NULL DEFAULT 1             COMMENT '版本号,从 1 起递增',
  `timeline`        JSON            NULL                           COMMENT '时间线 JSON,包含各片段的引用与剪辑参数',
  `output_url`      VARCHAR(512)    NOT NULL DEFAULT ''            COMMENT '最终成片 URL',
  `thumb_url`       VARCHAR(512)    NOT NULL DEFAULT ''            COMMENT '缩略图 URL',
  `cover_url`       VARCHAR(512)    NOT NULL DEFAULT ''            COMMENT '封面图 URL',
  `duration_ms`     INT             NOT NULL DEFAULT 0             COMMENT '成片时长(毫秒)',
  `status`          VARCHAR(16)     NOT NULL DEFAULT 'draft'       COMMENT '状态:draft=草稿 queued=排队中 running=合成中 succeeded=成功 failed=失败',
  `render_progress` INT             NOT NULL DEFAULT 0             COMMENT '合成进度 0-100',
  `error_msg`       VARCHAR(512)    NOT NULL DEFAULT ''            COMMENT '失败错误信息',
  `created_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0             COMMENT '创建人(用户 ID)',
  `updated_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0             COMMENT '最近更新人(用户 ID)',
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`      DATETIME(3)     NULL                           COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_project_id`     (`project_id`),
  KEY `idx_status_created` (`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='完整视频';

-- -------------------------------------------------------------
-- 10. 审核 / 发布
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `review_flows` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT            COMMENT '主键 ID',
  `name`        VARCHAR(64)     NOT NULL                           COMMENT '审核流名称',
  `description` VARCHAR(255)    NOT NULL DEFAULT ''                COMMENT '描述',
  `target_type` VARCHAR(32)     NOT NULL DEFAULT 'full_video'      COMMENT '审核对象类型:full_video=完整视频(目前仅支持该类型)',
  `enabled`     TINYINT         NOT NULL DEFAULT 1                 COMMENT '是否启用:0=禁用 1=启用',
  `is_default`  TINYINT         NOT NULL DEFAULT 0                 COMMENT '是否为该 target_type 的默认流:0=否 1=是',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`  DATETIME(3)     NULL                               COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_target_default` (`target_type`,`is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核流';

CREATE TABLE IF NOT EXISTS `review_nodes` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT     COMMENT '主键 ID',
  `flow_id`            BIGINT UNSIGNED NOT NULL                    COMMENT '所属审核流 ID',
  `step_no`            INT             NOT NULL                    COMMENT '步骤号,从 1 起,在同一 flow 内唯一',
  `name`               VARCHAR(64)     NOT NULL                    COMMENT '节点名称',
  `approver_type`      VARCHAR(16)     NOT NULL                    COMMENT '审批人类型:user=指定用户 role=指定角色',
  `approver_value`     VARCHAR(64)     NOT NULL                    COMMENT '审批人取值,user 时填用户 ID,role 时填角色 code',
  `allow_timeout_pass` TINYINT         NOT NULL DEFAULT 0          COMMENT '超时是否自动通过:0=否 1=是',
  `timeout_hours`      INT             NOT NULL DEFAULT 0          COMMENT '超时阈值(小时),0=不超时',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_flow_step` (`flow_id`,`step_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核节点';

CREATE TABLE IF NOT EXISTS `review_records` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT           COMMENT '主键 ID',
  `target_type`  VARCHAR(32)     NOT NULL DEFAULT 'full_video'     COMMENT '审核对象类型:full_video=完整视频',
  `target_id`    BIGINT UNSIGNED NOT NULL                          COMMENT '审核对象 ID(由 target_type 决定来源表)',
  `flow_id`      BIGINT UNSIGNED NOT NULL                          COMMENT '使用的审核流 ID',
  `current_step` INT             NOT NULL DEFAULT 1                COMMENT '当前所处步骤号',
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'pending'        COMMENT '整体状态:pending=审核中 approved=通过 rejected=驳回 cancelled=已撤回',
  `submitted_by` BIGINT UNSIGNED NOT NULL                          COMMENT '提交审核人(用户 ID)',
  `finished_at`  DATETIME(3)     NULL                              COMMENT '审核结束时间,NULL=未结束',
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_target` (`target_type`,`target_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核记录';

CREATE TABLE IF NOT EXISTS `review_node_records` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT      COMMENT '主键 ID',
  `review_record_id`  BIGINT UNSIGNED NOT NULL                     COMMENT '所属审核记录 ID',
  `step_no`           INT             NOT NULL                     COMMENT '步骤号',
  `approver_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0           COMMENT '实际审批人(用户 ID),0=系统(如超时自动通过)',
  `action`            VARCHAR(32)     NOT NULL                     COMMENT '审批动作:approve=通过 reject=驳回 timeout_pass=超时自动通过',
  `comment`           VARCHAR(1024)   NOT NULL DEFAULT ''          COMMENT '审批意见',
  `acted_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '审批时间',
  PRIMARY KEY (`id`),
  KEY `idx_review_record_id` (`review_record_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核节点记录';

CREATE TABLE IF NOT EXISTS `publishes` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT      COMMENT '主键 ID',
  `full_video_id`     BIGINT UNSIGNED NOT NULL                     COMMENT '关联完整视频 ID,唯一(同一视频仅一条发布记录)',
  `published_by`      BIGINT UNSIGNED NOT NULL                     COMMENT '发布人(用户 ID)',
  `published_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '首次发布时间',
  `status`            VARCHAR(8)      NOT NULL DEFAULT 'on'        COMMENT '发布状态:on=上架 off=下架',
  `watermark_config`  JSON            NULL                         COMMENT '水印配置 JSON',
  `download_count`    INT             NOT NULL DEFAULT 0           COMMENT '下载次数',
  `play_count`        INT             NOT NULL DEFAULT 0           COMMENT '播放次数',
  `updated_at`        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_full_video` (`full_video_id`),
  KEY `idx_status_published_at` (`status`,`published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发布';

-- -------------------------------------------------------------
-- 11. 模型域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `models` (
  `id`                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT    COMMENT '主键 ID',
  `code`                VARCHAR(64)     NOT NULL                   COMMENT '模型编码,全局唯一',
  `name`                VARCHAR(128)    NOT NULL                   COMMENT '模型名称',
  `type`                VARCHAR(16)     NOT NULL                   COMMENT '模型类型:text=文本 image=图片 video=视频 audio=音频',
  `provider`            VARCHAR(32)     NOT NULL                   COMMENT '服务提供商,如 openai/anthropic/aliyun',
  `endpoint`            VARCHAR(512)    NOT NULL DEFAULT ''        COMMENT '调用端点 URL',
  `api_key_encrypted`   VARBINARY(1024) NULL                       COMMENT 'API Key 密文(应用层加密)',
  `default_params`      JSON            NULL                       COMMENT '默认调用参数 JSON',
  `capability_tags`     JSON            NULL                       COMMENT '能力标签 JSON,如 ["vision","tool_use"]',
  `enabled`             TINYINT         NOT NULL DEFAULT 1         COMMENT '是否启用:0=禁用 1=启用',
  `priority`            INT             NOT NULL DEFAULT 0         COMMENT '优先级,同类型下越大越优先',
  `max_qps`             INT             NOT NULL DEFAULT 0         COMMENT '最大 QPS 限制,0=不限',
  `health_check_url`    VARCHAR(512)    NOT NULL DEFAULT ''        COMMENT '探活 URL,空=使用 endpoint',
  `last_health_at`      DATETIME(3)     NULL                       COMMENT '最近一次探活时间',
  `last_health_status`  TINYINT         NOT NULL DEFAULT 0         COMMENT '最近一次探活结果:0=unknown未探测 1=ok健康 2=fail异常',
  `created_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`          DATETIME(3)     NULL                       COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_type_enabled` (`type`,`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模型';

CREATE TABLE IF NOT EXISTS `model_pricing` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT        COMMENT '主键 ID',
  `model_id`        BIGINT UNSIGNED NOT NULL                       COMMENT '关联模型 ID',
  `metric`          VARCHAR(32)     NOT NULL                       COMMENT '计价维度:input_tokens=输入 token output_tokens=输出 token calls=按次调用 video_seconds=视频秒数 image=按图片张数',
  `price_per_unit`  DECIMAL(20,8)   NOT NULL DEFAULT 0             COMMENT '单价(每单位),保留 8 位小数',
  `currency`        VARCHAR(8)      NOT NULL DEFAULT 'CNY'         COMMENT '币种,如 CNY/USD',
  `effective_from`  DATETIME(3)     NOT NULL                       COMMENT '生效起始时间',
  `effective_to`    DATETIME(3)     NULL                           COMMENT '生效结束时间,NULL=至今仍有效',
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_model_metric_from` (`model_id`,`metric`,`effective_from`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模型计价';

CREATE TABLE IF NOT EXISTS `model_invocations` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT         COMMENT '主键 ID',
  `model_id`       BIGINT UNSIGNED NOT NULL                        COMMENT '调用的模型 ID',
  `user_id`        BIGINT UNSIGNED NOT NULL                        COMMENT '发起调用的用户 ID',
  `dept_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0              COMMENT '用户所属部门 ID,冗余便于统计',
  `project_id`     BIGINT UNSIGNED NOT NULL DEFAULT 0              COMMENT '关联项目 ID,0=非项目调用',
  `biz_type`       VARCHAR(16)     NOT NULL                        COMMENT '业务类型:text_gen=文本生成 image_gen=图片生成 video_gen=视频生成 video_compose=视频合成',
  `biz_ref`       VARCHAR(64)     NOT NULL DEFAULT ''              COMMENT '业务关联引用,如 short_video:123/image:456',
  `input_tokens`   INT             NOT NULL DEFAULT 0              COMMENT '输入 token 数',
  `output_tokens`  INT             NOT NULL DEFAULT 0              COMMENT '输出 token 数',
  `units`          INT             NOT NULL DEFAULT 0              COMMENT '通用计量单位(如视频秒数/图片张数)',
  `duration_ms`    INT             NOT NULL DEFAULT 0              COMMENT '调用耗时(毫秒)',
  `cost`           DECIMAL(20,8)   NOT NULL DEFAULT 0              COMMENT '本次调用成本,根据 model_pricing 计算',
  `status`         VARCHAR(16)     NOT NULL DEFAULT 'succeeded'    COMMENT '调用状态:succeeded=成功 failed=失败',
  `error_code`     VARCHAR(32)     NOT NULL DEFAULT ''             COMMENT '错误码,如 adapter_unavailable/model_call_failed,成功为空',
  `started_at`     DATETIME(3)     NOT NULL                        COMMENT '调用开始时间,分区键',
  `ended_at`       DATETIME(3)     NULL                            COMMENT '调用结束时间',
  PRIMARY KEY (`id`,`started_at`),
  KEY `idx_model_started` (`model_id`,`started_at`),
  KEY `idx_user_started`  (`user_id`,`started_at`),
  KEY `idx_dept_started`  (`dept_id`,`started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模型调用流水'
PARTITION BY RANGE COLUMNS(`started_at`) (
  PARTITION p202601 VALUES LESS THAN ('2026-02-01'),
  PARTITION p202602 VALUES LESS THAN ('2026-03-01'),
  PARTITION p202603 VALUES LESS THAN ('2026-04-01'),
  PARTITION p202604 VALUES LESS THAN ('2026-05-01'),
  PARTITION p202605 VALUES LESS THAN ('2026-06-01'),
  PARTITION p202606 VALUES LESS THAN ('2026-07-01'),
  PARTITION p202607 VALUES LESS THAN ('2026-08-01'),
  PARTITION p202608 VALUES LESS THAN ('2026-09-01'),
  PARTITION p202609 VALUES LESS THAN ('2026-10-01'),
  PARTITION p202610 VALUES LESS THAN ('2026-11-01'),
  PARTITION p202611 VALUES LESS THAN ('2026-12-01'),
  PARTITION p202612 VALUES LESS THAN ('2027-01-01'),
  PARTITION pMax    VALUES LESS THAN (MAXVALUE)
);

CREATE TABLE IF NOT EXISTS `billing_quotas` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT            COMMENT '主键 ID',
  `scope_type`  VARCHAR(8)      NOT NULL                           COMMENT '配额作用范围:user=按用户 dept=按部门',
  `scope_id`    BIGINT UNSIGNED NOT NULL                           COMMENT '范围 ID,由 scope_type 决定(用户 ID 或部门 ID)',
  `model_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0                 COMMENT '限制的模型 ID,0=全部模型',
  `period`      VARCHAR(16)     NOT NULL DEFAULT 'monthly'         COMMENT '周期:monthly=月度 daily=日 total=总额',
  `metric`      VARCHAR(32)     NOT NULL                           COMMENT '配额计量维度:calls=调用次数 cost=金额 tokens=token 数 invocations=调用条数',
  `quota_value` DECIMAL(20,4)   NOT NULL DEFAULT 0                 COMMENT '配额上限值',
  `used_value`  DECIMAL(20,4)   NOT NULL DEFAULT 0                 COMMENT '当前周期已用值',
  `reset_at`    DATETIME(3)     NULL                               COMMENT '下次重置时间,NULL=不自动重置',
  `enabled`     TINYINT         NOT NULL DEFAULT 1                 COMMENT '是否启用:0=禁用 1=启用',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_quota` (`scope_type`,`scope_id`,`model_id`,`period`,`metric`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='额度配置';

CREATE TABLE IF NOT EXISTS `billing_daily` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT         COMMENT '主键 ID',
  `stat_date`      DATE            NOT NULL                        COMMENT '统计日期',
  `model_id`       BIGINT UNSIGNED NOT NULL                        COMMENT '模型 ID',
  `dept_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0              COMMENT '部门 ID,0=未聚合到部门维度',
  `user_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0              COMMENT '用户 ID,0=未聚合到用户维度',
  `calls`          INT             NOT NULL DEFAULT 0              COMMENT '调用次数',
  `input_tokens`   BIGINT          NOT NULL DEFAULT 0              COMMENT '累计输入 token',
  `output_tokens`  BIGINT          NOT NULL DEFAULT 0              COMMENT '累计输出 token',
  `units`          BIGINT          NOT NULL DEFAULT 0              COMMENT '累计通用计量单位',
  `cost`           DECIMAL(20,8)   NOT NULL DEFAULT 0              COMMENT '累计成本',
  `created_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_stat` (`stat_date`,`model_id`,`dept_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每日计费聚合';

-- -------------------------------------------------------------
-- 12. 流水线域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `pipelines` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT           COMMENT '主键 ID',
  `project_id`   BIGINT UNSIGNED NOT NULL DEFAULT 0                COMMENT '归属项目 ID,0=全局模板',
  `name`         VARCHAR(128)    NOT NULL                          COMMENT '流水线名称',
  `description`  VARCHAR(512)    NOT NULL DEFAULT ''               COMMENT '描述',
  `dag`          JSON            NOT NULL                          COMMENT 'DAG 定义 JSON,描述节点与依赖关系',
  `is_template`  TINYINT         NOT NULL DEFAULT 0                COMMENT '是否模板:0=否 1=是(可被复制)',
  `enabled`      TINYINT         NOT NULL DEFAULT 1                COMMENT '是否启用:0=禁用 1=启用',
  `created_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0                COMMENT '创建人(用户 ID)',
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `deleted_at`   DATETIME(3)     NULL                              COMMENT '软删除时间,NULL=未删除',
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_is_template_enabled` (`is_template`,`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流水线';

CREATE TABLE IF NOT EXISTS `pipeline_runs` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT         COMMENT '主键 ID',
  `pipeline_id`    BIGINT UNSIGNED NOT NULL                        COMMENT '所属流水线 ID',
  `project_id`    BIGINT UNSIGNED NOT NULL                         COMMENT '归属项目 ID',
  `triggered_by`   BIGINT UNSIGNED NOT NULL                        COMMENT '触发人(用户 ID)',
  `trigger_type`   VARCHAR(16)     NOT NULL DEFAULT 'manual'       COMMENT '触发方式:manual=手动 scheduled=定时(预留) webhook=回调(预留)',
  `input`          JSON            NULL                            COMMENT '运行入参 JSON',
  `output`         JSON            NULL                            COMMENT '运行输出 JSON',
  `status`         VARCHAR(16)     NOT NULL DEFAULT 'queued'       COMMENT '状态:queued=排队中 pending=等待 running=运行中 succeeded=成功 failed=失败 cancelled=已取消',
  `started_at`     DATETIME(3)     NULL                            COMMENT '开始时间,NULL=尚未开始',
  `ended_at`       DATETIME(3)     NULL                            COMMENT '结束时间,NULL=尚未结束',
  `error_msg`      VARCHAR(1024)   NOT NULL DEFAULT ''             COMMENT '失败错误信息',
  `created_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_pipeline_id` (`pipeline_id`),
  KEY `idx_project_id`  (`project_id`),
  KEY `idx_status_started` (`status`,`started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流水线运行';

CREATE TABLE IF NOT EXISTS `step_runs` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT            COMMENT '主键 ID',
  `run_id`      BIGINT UNSIGNED NOT NULL                           COMMENT '所属流水线运行 ID',
  `node_id`     VARCHAR(64)     NOT NULL                           COMMENT 'DAG 中的节点 ID',
  `node_type`   VARCHAR(64)     NOT NULL                           COMMENT '节点类型,如 llm/image/video/compose/review',
  `model_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0                 COMMENT '本步骤使用的模型 ID,0=不涉及模型',
  `input`       JSON            NULL                               COMMENT '步骤入参 JSON',
  `output`      JSON            NULL                               COMMENT '步骤输出 JSON',
  `status`      VARCHAR(16)     NOT NULL DEFAULT 'queued'          COMMENT '状态:queued=排队中 pending=等待 running=运行中 succeeded=成功 failed=失败',
  `attempt`     INT             NOT NULL DEFAULT 0                 COMMENT '已尝试次数(含重试)',
  `started_at`  DATETIME(3)     NULL                               COMMENT '开始时间,NULL=尚未开始',
  `ended_at`    DATETIME(3)     NULL                               COMMENT '结束时间,NULL=尚未结束',
  `error_msg`   VARCHAR(1024)   NOT NULL DEFAULT ''                COMMENT '失败错误信息',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_run_started` (`run_id`,`started_at`),
  KEY `idx_status`      (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点运行';

-- -------------------------------------------------------------
-- 13. 系统域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `audit_logs` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT          COMMENT '主键 ID',
  `user_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0               COMMENT '操作人(用户 ID),0=系统/匿名',
  `action`        VARCHAR(64)     NOT NULL                         COMMENT '动作名称,如 create/update/delete/publish 等',
  `resource_type` VARCHAR(64)     NOT NULL                         COMMENT '资源类型,如 project/script/full_video',
  `resource_id`   VARCHAR(64)     NOT NULL DEFAULT ''              COMMENT '资源 ID(字符串以兼容复合主键)',
  `before`        JSON            NULL                             COMMENT '变更前快照 JSON',
  `after`         JSON            NULL                             COMMENT '变更后快照 JSON',
  `ip`            VARCHAR(64)     NOT NULL DEFAULT ''              COMMENT '操作来源 IP',
  `ua`            VARCHAR(255)    NOT NULL DEFAULT ''              COMMENT 'User-Agent',
  `request_id`    VARCHAR(64)     NOT NULL DEFAULT ''              COMMENT '请求 trace ID,便于链路追踪',
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '记录时间,分区键',
  PRIMARY KEY (`id`,`created_at`),
  KEY `idx_user_created`     (`user_id`,`created_at`),
  KEY `idx_resource_created` (`resource_type`,`resource_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审计日志'
PARTITION BY RANGE COLUMNS(`created_at`) (
  PARTITION p202601 VALUES LESS THAN ('2026-02-01'),
  PARTITION p202602 VALUES LESS THAN ('2026-03-01'),
  PARTITION p202603 VALUES LESS THAN ('2026-04-01'),
  PARTITION p202604 VALUES LESS THAN ('2026-05-01'),
  PARTITION p202605 VALUES LESS THAN ('2026-06-01'),
  PARTITION p202606 VALUES LESS THAN ('2026-07-01'),
  PARTITION p202607 VALUES LESS THAN ('2026-08-01'),
  PARTITION p202608 VALUES LESS THAN ('2026-09-01'),
  PARTITION p202609 VALUES LESS THAN ('2026-10-01'),
  PARTITION p202610 VALUES LESS THAN ('2026-11-01'),
  PARTITION p202611 VALUES LESS THAN ('2026-12-01'),
  PARTITION p202612 VALUES LESS THAN ('2027-01-01'),
  PARTITION pMax    VALUES LESS THAN (MAXVALUE)
);

CREATE TABLE IF NOT EXISTS `feature_flags` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT            COMMENT '主键 ID',
  `key`         VARCHAR(64)     NOT NULL                           COMMENT '特性开关键,全局唯一',
  `description` VARCHAR(255)    NOT NULL DEFAULT ''                COMMENT '描述',
  `enabled`     TINYINT         NOT NULL DEFAULT 0                 COMMENT '是否启用:0=禁用 1=启用',
  `rollout`     INT             NOT NULL DEFAULT 0                 COMMENT '灰度百分比 0-100',
  `rules`       JSON            NULL                               COMMENT '定向命中规则 JSON,如 {users:[1,2],depts:[10],projects:[5]}',
  `created_by`  BIGINT UNSIGNED NOT NULL DEFAULT 0                 COMMENT '创建人(用户 ID)',
  `updated_by`  BIGINT UNSIGNED NOT NULL DEFAULT 0                 COMMENT '最近更新人(用户 ID)',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_key` (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='灰度/特性开关';

CREATE TABLE IF NOT EXISTS `sys_dicts` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT             COMMENT '主键 ID',
  `type`       VARCHAR(64)     NOT NULL                            COMMENT '字典分组类型,如 video_status/shot_type',
  `code`       VARCHAR(64)     NOT NULL                            COMMENT '字典项编码,在 type 内唯一',
  `name`       VARCHAR(128)    NOT NULL                            COMMENT '展示名称',
  `value`      VARCHAR(255)    NOT NULL DEFAULT ''                 COMMENT '字典值',
  `sort`       INT             NOT NULL DEFAULT 0                  COMMENT '同分组排序,越小越靠前',
  `status`     TINYINT         NOT NULL DEFAULT 1                  COMMENT '状态:1=启用 2=禁用',
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_type_code` (`type`,`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='字典';

SET FOREIGN_KEY_CHECKS = 1;
