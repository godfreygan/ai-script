-- =============================================================
-- AI 短剧视频生成平台 · 初始化 DDL  v1.0
-- MySQL 8.0+ , InnoDB, utf8mb4
-- =============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;
SET time_zone = '+00:00';

-- -------------------------------------------------------------
-- 1. 用户域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `departments` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`       VARCHAR(64)     NOT NULL,
  `parent_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `path`       VARCHAR(255)    NOT NULL DEFAULT '',
  `sort`       INT             NOT NULL DEFAULT 0,
  `status`     TINYINT         NOT NULL DEFAULT 1 COMMENT '1=启用 2=禁用',
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_path`      (`path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='部门';

CREATE TABLE IF NOT EXISTS `users` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username`       VARCHAR(64)     NOT NULL,
  `password_hash`  VARCHAR(128)    NOT NULL,
  `nickname`       VARCHAR(64)     NOT NULL DEFAULT '',
  `email`          VARCHAR(128)    NOT NULL DEFAULT '',
  `phone`          VARCHAR(20)     NOT NULL DEFAULT '',
  `avatar_url`     VARCHAR(512)    NOT NULL DEFAULT '',
  `dept_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `status`         TINYINT         NOT NULL DEFAULT 1 COMMENT '1=正常 2=禁用',
  `last_login_at`  DATETIME(3)     NULL,
  `last_login_ip`  VARCHAR(64)     NOT NULL DEFAULT '',
  `created_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`     DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_email`   (`email`),
  KEY `idx_dept_id` (`dept_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户';

CREATE TABLE IF NOT EXISTS `user_api_tokens` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`       BIGINT UNSIGNED NOT NULL,
  `name`          VARCHAR(64)     NOT NULL,
  `token_hash`    VARCHAR(128)    NOT NULL,
  `scopes`        JSON            NULL,
  `expires_at`    DATETIME(3)     NULL,
  `last_used_at`  DATETIME(3)     NULL,
  `status`        TINYINT         NOT NULL DEFAULT 1,
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`    DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_token_hash` (`token_hash`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户 API Token';

-- -------------------------------------------------------------
-- 2. 鉴权域 (RBAC)
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `roles` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code`        VARCHAR(64)     NOT NULL,
  `name`        VARCHAR(64)     NOT NULL,
  `description` VARCHAR(255)    NOT NULL DEFAULT '',
  `data_scope`  VARCHAR(16)     NOT NULL DEFAULT 'self' COMMENT 'self/dept/all',
  `is_system`   TINYINT         NOT NULL DEFAULT 0,
  `status`      TINYINT         NOT NULL DEFAULT 1,
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`  DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色';

CREATE TABLE IF NOT EXISTS `permissions` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code`        VARCHAR(128)    NOT NULL COMMENT '如 project:read',
  `name`        VARCHAR(128)    NOT NULL,
  `resource`    VARCHAR(64)     NOT NULL,
  `action`      VARCHAR(32)     NOT NULL,
  `description` VARCHAR(255)    NOT NULL DEFAULT '',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_resource_action` (`resource`,`action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='权限点';

CREATE TABLE IF NOT EXISTS `role_permissions` (
  `role_id`       BIGINT UNSIGNED NOT NULL,
  `permission_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`role_id`,`permission_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色-权限';

CREATE TABLE IF NOT EXISTS `user_roles` (
  `user_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`user_id`,`role_id`),
  KEY `idx_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户-角色';

CREATE TABLE IF NOT EXISTS `casbin_rule` (
  `id`    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ptype` VARCHAR(100)    NOT NULL DEFAULT '',
  `v0`    VARCHAR(100)    NOT NULL DEFAULT '',
  `v1`    VARCHAR(100)    NOT NULL DEFAULT '',
  `v2`    VARCHAR(100)    NOT NULL DEFAULT '',
  `v3`    VARCHAR(100)    NOT NULL DEFAULT '',
  `v4`    VARCHAR(100)    NOT NULL DEFAULT '',
  `v5`    VARCHAR(100)    NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_rule` (`ptype`,`v0`,`v1`,`v2`,`v3`,`v4`,`v5`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Casbin 规则表';

-- -------------------------------------------------------------
-- 3. 项目域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `projects` (
  `id`                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code`                VARCHAR(64)     NOT NULL,
  `name`                VARCHAR(128)    NOT NULL,
  `description`         TEXT,
  `owner_id`            BIGINT UNSIGNED NOT NULL,
  `dept_id`             BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `status`              TINYINT         NOT NULL DEFAULT 1 COMMENT '1=draft 2=in_production 3=in_review 4=published 5=archived',
  `default_pipeline_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `cover_url`           VARCHAR(512)    NOT NULL DEFAULT '',
  `tags`                JSON            NULL,
  `created_by`          BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by`          BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`          DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_owner_id`    (`owner_id`),
  KEY `idx_dept_status` (`dept_id`,`status`),
  KEY `idx_status_created` (`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目';

CREATE TABLE IF NOT EXISTS `project_members` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`      BIGINT UNSIGNED NOT NULL,
  `user_id`         BIGINT UNSIGNED NOT NULL,
  `role_in_project` VARCHAR(32)     NOT NULL DEFAULT 'editor' COMMENT 'producer/editor/reviewer/viewer',
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_user` (`project_id`,`user_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目成员';

-- -------------------------------------------------------------
-- 4. 剧本域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `scripts` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`      BIGINT UNSIGNED NOT NULL,
  `name`            VARCHAR(128)    NOT NULL,
  `source_url`      VARCHAR(512)    NOT NULL DEFAULT '',
  `raw_text`        MEDIUMTEXT,
  `current_version` INT             NOT NULL DEFAULT 1,
  `status`          TINYINT         NOT NULL DEFAULT 1 COMMENT '1=uploaded 2=parsed 3=episode_split',
  `meta`            JSON            NULL,
  `created_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`      DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`),
  FULLTEXT KEY `ft_raw_text` (`raw_text`) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='剧本';

CREATE TABLE IF NOT EXISTS `script_versions` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `script_id`  BIGINT UNSIGNED NOT NULL,
  `version_no` INT             NOT NULL,
  `content`    MEDIUMTEXT,
  `diff`       JSON            NULL,
  `commit_msg` VARCHAR(255)    NOT NULL DEFAULT '',
  `created_by` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_script_version` (`script_id`,`version_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='剧本版本';

CREATE TABLE IF NOT EXISTS `episodes` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `script_id`    BIGINT UNSIGNED NOT NULL,
  `ep_no`        INT             NOT NULL,
  `title`        VARCHAR(255)    NOT NULL DEFAULT '',
  `summary`      TEXT,
  `raw_segment`  MEDIUMTEXT,
  `char_begin`   INT             NOT NULL DEFAULT 0,
  `char_end`     INT             NOT NULL DEFAULT 0,
  `status`       TINYINT         NOT NULL DEFAULT 1,
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`   DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_script_ep` (`script_id`,`ep_no`),
  KEY `idx_script_id` (`script_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分集';

-- -------------------------------------------------------------
-- 5. 提示词域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `episode_prompts` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `episode_id`         BIGINT UNSIGNED NOT NULL,
  `version`            INT             NOT NULL DEFAULT 1,
  `is_current`         TINYINT         NOT NULL DEFAULT 0,
  `content`            JSON            NULL,
  `model_id`           BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `generation_params`  JSON            NULL,
  `status`             TINYINT         NOT NULL DEFAULT 1,
  `generated_by`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_episode_version` (`episode_id`,`version`),
  KEY `idx_episode_current` (`episode_id`,`is_current`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分集提示词';

-- -------------------------------------------------------------
-- 6. 分镜 / 风格域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `storyboards` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `episode_id`    BIGINT UNSIGNED NOT NULL,
  `prompt_id`     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `shot_no`       INT             NOT NULL,
  `shot_type`     VARCHAR(32)     NOT NULL DEFAULT 'medium' COMMENT 'wide/medium/close/extreme_close/establishing',
  `camera_motion` VARCHAR(32)     NOT NULL DEFAULT 'static' COMMENT 'pan/zoom/dolly/track/static',
  `scene_desc`    TEXT,
  `characters`    JSON,
  `action`        TEXT,
  `dialogue`      TEXT,
  `duration_sec` INT             NOT NULL DEFAULT 15,
  `notes`         TEXT,
  `status`        TINYINT         NOT NULL DEFAULT 1,
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`    DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_episode_shot` (`episode_id`,`shot_no`),
  KEY `idx_prompt_id` (`prompt_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分镜';

CREATE TABLE IF NOT EXISTS `styles` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=公共',
  `name`             VARCHAR(64)     NOT NULL,
  `art_style`        VARCHAR(64)     NOT NULL DEFAULT '',
  `color_tone`       VARCHAR(64)     NOT NULL DEFAULT '',
  `lighting`         VARCHAR(64)     NOT NULL DEFAULT '',
  `reference_images` JSON            NULL,
  `lora_id`          VARCHAR(128)    NOT NULL DEFAULT '',
  `description`      TEXT,
  `status`           TINYINT         NOT NULL DEFAULT 1,
  `created_by`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`       DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='风格';

CREATE TABLE IF NOT EXISTS `storyboard_styles` (
  `storyboard_id` BIGINT UNSIGNED NOT NULL,
  `style_id`      BIGINT UNSIGNED NOT NULL,
  `applied_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `applied_by`    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`storyboard_id`,`style_id`),
  KEY `idx_style_id` (`style_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分镜-风格 多对多';

-- -------------------------------------------------------------
-- 7. 图片域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `images` (
  `id`                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`           BIGINT UNSIGNED NOT NULL,
  `storyboard_id`        BIGINT UNSIGNED NULL,
  `src_type`             VARCHAR(16)     NOT NULL DEFAULT 'generated' COMMENT 'generated/uploaded',
  `url`                  VARCHAR(512)    NOT NULL,
  `thumb_url`            VARCHAR(512)    NOT NULL DEFAULT '',
  `width`                INT             NOT NULL DEFAULT 0,
  `height`               INT             NOT NULL DEFAULT 0,
  `prompt`               TEXT,
  `neg_prompt`           TEXT,
  `model_id`             BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `params`               JSON            NULL,
  `status`               TINYINT         NOT NULL DEFAULT 1 COMMENT '1=ok 2=failed',
  `generated_in_run_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_by`           BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`           DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_project_id`     (`project_id`),
  KEY `idx_storyboard_id`  (`storyboard_id`),
  KEY `idx_status_created` (`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='图片';

-- -------------------------------------------------------------
-- 8. 短视频域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `short_videos` (
  `id`                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`           BIGINT UNSIGNED NOT NULL,
  `storyboard_id`        BIGINT UNSIGNED NULL,
  `src_type`             VARCHAR(16)     NOT NULL DEFAULT 'generated' COMMENT 'generated/uploaded',
  `prompt`               TEXT,
  `source_image_ids`     JSON            NULL,
  `video_url`            VARCHAR(512)    NOT NULL DEFAULT '',
  `thumb_url`            VARCHAR(512)    NOT NULL DEFAULT '',
  `duration_ms`          INT             NOT NULL DEFAULT 0,
  `width`                INT             NOT NULL DEFAULT 0,
  `height`               INT             NOT NULL DEFAULT 0,
  `audio_url`            VARCHAR(512)    NOT NULL DEFAULT '',
  `subtitle_url`         VARCHAR(512)    NOT NULL DEFAULT '',
  `model_id`             BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `params`               JSON            NULL,
  `status`               VARCHAR(16)     NOT NULL DEFAULT 'queued' COMMENT 'queued/generating/succeeded/failed/cancelled',
  `error_msg`            VARCHAR(512)    NOT NULL DEFAULT '',
  `retry_count`          INT             NOT NULL DEFAULT 0,
  `generated_in_run_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_by`           BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`           DATETIME(3)     NULL,
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
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`      BIGINT UNSIGNED NOT NULL,
  `name`            VARCHAR(128)    NOT NULL,
  `version`         INT             NOT NULL DEFAULT 1,
  `timeline`        JSON            NULL,
  `output_url`      VARCHAR(512)    NOT NULL DEFAULT '',
  `thumb_url`       VARCHAR(512)    NOT NULL DEFAULT '',
  `cover_url`       VARCHAR(512)    NOT NULL DEFAULT '',
  `duration_ms`     INT             NOT NULL DEFAULT 0,
  `status`          VARCHAR(16)     NOT NULL DEFAULT 'draft' COMMENT 'draft/rendering/rendered/in_review/published/off',
  `render_progress` INT             NOT NULL DEFAULT 0,
  `error_msg`       VARCHAR(512)    NOT NULL DEFAULT '',
  `created_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by`      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`      DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_project_id`     (`project_id`),
  KEY `idx_status_created` (`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='完整视频';

-- -------------------------------------------------------------
-- 10. 审核 / 发布
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `review_flows` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)     NOT NULL,
  `description` VARCHAR(255)    NOT NULL DEFAULT '',
  `target_type` VARCHAR(32)     NOT NULL DEFAULT 'full_video',
  `enabled`     TINYINT         NOT NULL DEFAULT 1,
  `is_default`  TINYINT         NOT NULL DEFAULT 0,
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`  DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_target_default` (`target_type`,`is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核流';

CREATE TABLE IF NOT EXISTS `review_nodes` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `flow_id`            BIGINT UNSIGNED NOT NULL,
  `step_no`            INT             NOT NULL,
  `name`               VARCHAR(64)     NOT NULL,
  `approver_type`      VARCHAR(16)     NOT NULL COMMENT 'user/role',
  `approver_value`     VARCHAR(64)     NOT NULL,
  `allow_timeout_pass` TINYINT         NOT NULL DEFAULT 0,
  `timeout_hours`      INT             NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_flow_step` (`flow_id`,`step_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核节点';

CREATE TABLE IF NOT EXISTS `review_records` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `target_type`  VARCHAR(32)     NOT NULL DEFAULT 'full_video',
  `target_id`    BIGINT UNSIGNED NOT NULL,
  `flow_id`      BIGINT UNSIGNED NOT NULL,
  `current_step` INT             NOT NULL DEFAULT 1,
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected/withdrawn',
  `submitted_by` BIGINT UNSIGNED NOT NULL,
  `finished_at`  DATETIME(3)     NULL,
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_target` (`target_type`,`target_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核记录';

CREATE TABLE IF NOT EXISTS `review_node_records` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `review_record_id`  BIGINT UNSIGNED NOT NULL,
  `step_no`           INT             NOT NULL,
  `approver_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `action`            VARCHAR(32)     NOT NULL COMMENT 'approve/reject_back/reject_final/timeout_pass',
  `comment`           VARCHAR(1024)   NOT NULL DEFAULT '',
  `acted_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_review_record_id` (`review_record_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核节点记录';

CREATE TABLE IF NOT EXISTS `publishes` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `full_video_id`     BIGINT UNSIGNED NOT NULL,
  `published_by`      BIGINT UNSIGNED NOT NULL,
  `published_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `status`            VARCHAR(8)      NOT NULL DEFAULT 'on' COMMENT 'on/off',
  `watermark_config`  JSON            NULL,
  `download_count`    INT             NOT NULL DEFAULT 0,
  `play_count`        INT             NOT NULL DEFAULT 0,
  `updated_at`        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_full_video` (`full_video_id`),
  KEY `idx_status_published_at` (`status`,`published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发布';

-- -------------------------------------------------------------
-- 11. 模型域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `models` (
  `id`                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code`                VARCHAR(64)     NOT NULL,
  `name`                VARCHAR(128)    NOT NULL,
  `type`                VARCHAR(16)     NOT NULL COMMENT 'text/image/video/audio',
  `provider`            VARCHAR(32)     NOT NULL,
  `endpoint`            VARCHAR(512)    NOT NULL DEFAULT '',
  `api_key_encrypted`   VARBINARY(1024) NULL,
  `default_params`      JSON            NULL,
  `capability_tags`     JSON            NULL,
  `enabled`             TINYINT         NOT NULL DEFAULT 1,
  `priority`            INT             NOT NULL DEFAULT 0,
  `max_qps`             INT             NOT NULL DEFAULT 0,
  `health_check_url`    VARCHAR(512)    NOT NULL DEFAULT '',
  `last_health_at`      DATETIME(3)     NULL,
  `last_health_status`  TINYINT         NOT NULL DEFAULT 0 COMMENT '0=unknown 1=ok 2=fail',
  `created_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`          DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_type_enabled` (`type`,`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模型';

CREATE TABLE IF NOT EXISTS `model_pricing` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `model_id`        BIGINT UNSIGNED NOT NULL,
  `metric`          VARCHAR(32)     NOT NULL COMMENT 'input_tokens/output_tokens/calls/video_seconds/image',
  `price_per_unit`  DECIMAL(20,8)   NOT NULL DEFAULT 0,
  `currency`        VARCHAR(8)      NOT NULL DEFAULT 'CNY',
  `effective_from`  DATETIME(3)     NOT NULL,
  `effective_to`    DATETIME(3)     NULL,
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_model_metric_from` (`model_id`,`metric`,`effective_from`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模型计价';

CREATE TABLE IF NOT EXISTS `model_invocations` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `model_id`       BIGINT UNSIGNED NOT NULL,
  `user_id`        BIGINT UNSIGNED NOT NULL,
  `dept_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `project_id`     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `biz_type`       VARCHAR(16)     NOT NULL,
  `biz_ref`        VARCHAR(64)     NOT NULL DEFAULT '',
  `input_tokens`   INT             NOT NULL DEFAULT 0,
  `output_tokens`  INT             NOT NULL DEFAULT 0,
  `units`          INT             NOT NULL DEFAULT 0,
  `duration_ms`    INT             NOT NULL DEFAULT 0,
  `cost`           DECIMAL(20,8)   NOT NULL DEFAULT 0,
  `status`         VARCHAR(16)     NOT NULL DEFAULT 'succeeded',
  `error_code`     VARCHAR(32)     NOT NULL DEFAULT '',
  `started_at`     DATETIME(3)     NOT NULL,
  `ended_at`       DATETIME(3)     NULL,
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
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `scope_type`  VARCHAR(8)      NOT NULL COMMENT 'dept/user',
  `scope_id`    BIGINT UNSIGNED NOT NULL,
  `model_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `period`      VARCHAR(16)     NOT NULL DEFAULT 'monthly' COMMENT 'monthly/total',
  `metric`      VARCHAR(32)     NOT NULL,
  `quota_value` DECIMAL(20,4)   NOT NULL DEFAULT 0,
  `used_value`  DECIMAL(20,4)   NOT NULL DEFAULT 0,
  `reset_at`    DATETIME(3)     NULL,
  `enabled`     TINYINT         NOT NULL DEFAULT 1,
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_quota` (`scope_type`,`scope_id`,`model_id`,`period`,`metric`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='额度配置';

CREATE TABLE IF NOT EXISTS `billing_daily` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `stat_date`      DATE            NOT NULL,
  `model_id`       BIGINT UNSIGNED NOT NULL,
  `dept_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `user_id`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `calls`          INT             NOT NULL DEFAULT 0,
  `input_tokens`   BIGINT          NOT NULL DEFAULT 0,
  `output_tokens`  BIGINT          NOT NULL DEFAULT 0,
  `units`          BIGINT          NOT NULL DEFAULT 0,
  `cost`           DECIMAL(20,8)   NOT NULL DEFAULT 0,
  `created_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_stat` (`stat_date`,`model_id`,`dept_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每日计费聚合';

-- -------------------------------------------------------------
-- 12. 流水线域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `pipelines` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=全局模板',
  `name`         VARCHAR(128)    NOT NULL,
  `description`  VARCHAR(512)    NOT NULL DEFAULT '',
  `dag`          JSON            NOT NULL,
  `is_template`  TINYINT         NOT NULL DEFAULT 0,
  `enabled`      TINYINT         NOT NULL DEFAULT 1,
  `created_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`   DATETIME(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_is_template_enabled` (`is_template`,`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流水线';

CREATE TABLE IF NOT EXISTS `pipeline_runs` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `pipeline_id`    BIGINT UNSIGNED NOT NULL,
  `project_id`    BIGINT UNSIGNED NOT NULL,
  `triggered_by`   BIGINT UNSIGNED NOT NULL,
  `trigger_type`   VARCHAR(16)     NOT NULL DEFAULT 'manual',
  `input`          JSON            NULL,
  `output`         JSON            NULL,
  `status`         VARCHAR(16)     NOT NULL DEFAULT 'queued',
  `started_at`     DATETIME(3)     NULL,
  `ended_at`       DATETIME(3)     NULL,
  `error_msg`      VARCHAR(1024)   NOT NULL DEFAULT '',
  `created_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_pipeline_id` (`pipeline_id`),
  KEY `idx_project_id`  (`project_id`),
  KEY `idx_status_started` (`status`,`started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流水线运行';

CREATE TABLE IF NOT EXISTS `step_runs` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id`      BIGINT UNSIGNED NOT NULL,
  `node_id`     VARCHAR(64)     NOT NULL,
  `node_type`   VARCHAR(64)     NOT NULL,
  `model_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `input`       JSON            NULL,
  `output`      JSON            NULL,
  `status`      VARCHAR(16)     NOT NULL DEFAULT 'queued',
  `attempt`     INT             NOT NULL DEFAULT 0,
  `started_at`  DATETIME(3)     NULL,
  `ended_at`    DATETIME(3)     NULL,
  `error_msg`   VARCHAR(1024)   NOT NULL DEFAULT '',
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_run_started` (`run_id`,`started_at`),
  KEY `idx_status`      (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点运行';

-- -------------------------------------------------------------
-- 13. 系统域
-- -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `audit_logs` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `action`        VARCHAR(64)     NOT NULL,
  `resource_type` VARCHAR(64)     NOT NULL,
  `resource_id`   VARCHAR(64)     NOT NULL DEFAULT '',
  `before`        JSON            NULL,
  `after`         JSON            NULL,
  `ip`            VARCHAR(64)     NOT NULL DEFAULT '',
  `ua`            VARCHAR(255)    NOT NULL DEFAULT '',
  `request_id`    VARCHAR(64)     NOT NULL DEFAULT '',
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
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
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `key`         VARCHAR(64)     NOT NULL,
  `description` VARCHAR(255)    NOT NULL DEFAULT '',
  `enabled`     TINYINT         NOT NULL DEFAULT 0,
  `rollout`     INT             NOT NULL DEFAULT 0 COMMENT '0-100,百分比灰度',
  `rules`       JSON            NULL COMMENT '{users:[1,2],depts:[10],projects:[5]}',
  `created_by`  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by`  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_key` (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='灰度/特性开关';

CREATE TABLE IF NOT EXISTS `sys_dicts` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `type`       VARCHAR(64)     NOT NULL,
  `code`       VARCHAR(64)     NOT NULL,
  `name`       VARCHAR(128)    NOT NULL,
  `value`      VARCHAR(255)    NOT NULL DEFAULT '',
  `sort`       INT             NOT NULL DEFAULT 0,
  `status`     TINYINT         NOT NULL DEFAULT 1,
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_type_code` (`type`,`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='字典';

SET FOREIGN_KEY_CHECKS = 1;
