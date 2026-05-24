-- AI Script seed data aligned with backend/internal/repo/seed.go
-- Safe for fresh initialization and repeat execution on local environments.

SET NAMES utf8mb4;

SET @admin_password_hash = '$2a$10$9cxa7tVUgiWADsKLo5paFOyhiM0UjwY72DBPA2Eyi/9G5.iiTSbaC';

INSERT INTO `roles` (`code`,`name`,`description`,`data_scope`,`is_system`,`status`)
VALUES
  ('super_admin', 'super admin', 'all permissions', 'all', 1, 1),
  ('viewer', 'viewer', 'read only', 'all', 1, 1)
ON DUPLICATE KEY UPDATE
  `name` = VALUES(`name`),
  `description` = VALUES(`description`),
  `data_scope` = VALUES(`data_scope`),
  `is_system` = VALUES(`is_system`),
  `status` = VALUES(`status`);

INSERT INTO `permissions` (`code`,`name`,`resource`,`action`,`description`)
SELECT
  CONCAT(r.resource, ':', a.action) AS code,
  CONCAT(r.resource, ':', a.action) AS name,
  r.resource,
  a.action,
  CONCAT(a.action, ' ', r.resource) AS description
FROM (
  SELECT 'user' AS resource UNION ALL
  SELECT 'dept' UNION ALL
  SELECT 'role' UNION ALL
  SELECT 'project' UNION ALL
  SELECT 'model' UNION ALL
  SELECT 'script' UNION ALL
  SELECT 'storyboard' UNION ALL
  SELECT 'style' UNION ALL
  SELECT 'image' UNION ALL
  SELECT 'short_video' UNION ALL
  SELECT 'full_video' UNION ALL
  SELECT 'pipeline' UNION ALL
  SELECT 'upload' UNION ALL
  SELECT 'invocation' UNION ALL
  SELECT 'review' UNION ALL
  SELECT 'publish' UNION ALL
  SELECT 'billing' UNION ALL
  SELECT 'audit' UNION ALL
  SELECT 'feature_flag'
) AS r
CROSS JOIN (
  SELECT 'read' AS action UNION ALL
  SELECT 'create' UNION ALL
  SELECT 'update' UNION ALL
  SELECT 'delete' UNION ALL
  SELECT 'execute'
) AS a
ON DUPLICATE KEY UPDATE
  `name` = VALUES(`name`),
  `resource` = VALUES(`resource`),
  `action` = VALUES(`action`),
  `description` = VALUES(`description`);

INSERT INTO `users` (`username`,`password_hash`,`nickname`,`email`,`status`)
VALUES ('admin', @admin_password_hash, 'admin', 'admin@example.com', 1)
ON DUPLICATE KEY UPDATE
  `password_hash` = VALUES(`password_hash`),
  `nickname` = VALUES(`nickname`),
  `email` = VALUES(`email`),
  `status` = VALUES(`status`);

INSERT IGNORE INTO `user_roles` (`user_id`,`role_id`)
SELECT u.id, r.id
FROM `users` AS u
JOIN `roles` AS r ON r.code = 'super_admin'
WHERE u.username = 'admin';

INSERT IGNORE INTO `role_permissions` (`role_id`,`permission_id`)
SELECT r.id, p.id
FROM `roles` AS r
JOIN `permissions` AS p
WHERE r.code = 'super_admin';

INSERT IGNORE INTO `role_permissions` (`role_id`,`permission_id`)
SELECT r.id, p.id
FROM `roles` AS r
JOIN `permissions` AS p
WHERE r.code = 'viewer'
  AND (
    p.action = 'read'
    OR (
      p.action IN ('create', 'update')
      AND p.resource IN (
        'project',
        'script',
        'storyboard',
        'style',
        'image',
        'short_video',
        'full_video',
        'pipeline',
        'upload'
      )
    )
  );

INSERT IGNORE INTO `casbin_rule` (`ptype`,`v0`,`v1`,`v2`,`v3`,`v4`,`v5`)
SELECT
  'p',
  'super_admin',
  r.resource,
  a.action,
  '',
  '',
  ''
FROM (
  SELECT 'user' AS resource UNION ALL
  SELECT 'dept' UNION ALL
  SELECT 'role' UNION ALL
  SELECT 'project' UNION ALL
  SELECT 'model' UNION ALL
  SELECT 'script' UNION ALL
  SELECT 'storyboard' UNION ALL
  SELECT 'style' UNION ALL
  SELECT 'image' UNION ALL
  SELECT 'short_video' UNION ALL
  SELECT 'full_video' UNION ALL
  SELECT 'pipeline' UNION ALL
  SELECT 'upload' UNION ALL
  SELECT 'invocation' UNION ALL
  SELECT 'review' UNION ALL
  SELECT 'publish' UNION ALL
  SELECT 'billing' UNION ALL
  SELECT 'audit' UNION ALL
  SELECT 'feature_flag'
) AS r
CROSS JOIN (
  SELECT 'read' AS action UNION ALL
  SELECT 'create' UNION ALL
  SELECT 'update' UNION ALL
  SELECT 'delete' UNION ALL
  SELECT 'execute'
) AS a;

INSERT IGNORE INTO `casbin_rule` (`ptype`,`v0`,`v1`,`v2`,`v3`,`v4`,`v5`)
SELECT
  'p',
  'viewer',
  r.resource,
  'read',
  '',
  '',
  ''
FROM (
  SELECT 'user' AS resource UNION ALL
  SELECT 'dept' UNION ALL
  SELECT 'role' UNION ALL
  SELECT 'project' UNION ALL
  SELECT 'model' UNION ALL
  SELECT 'script' UNION ALL
  SELECT 'storyboard' UNION ALL
  SELECT 'style' UNION ALL
  SELECT 'image' UNION ALL
  SELECT 'short_video' UNION ALL
  SELECT 'full_video' UNION ALL
  SELECT 'pipeline' UNION ALL
  SELECT 'upload' UNION ALL
  SELECT 'invocation' UNION ALL
  SELECT 'review' UNION ALL
  SELECT 'publish' UNION ALL
  SELECT 'billing' UNION ALL
  SELECT 'audit' UNION ALL
  SELECT 'feature_flag'
) AS r;

INSERT IGNORE INTO `casbin_rule` (`ptype`,`v0`,`v1`,`v2`,`v3`,`v4`,`v5`)
SELECT
  'p',
  'viewer',
  r.resource,
  a.action,
  '',
  '',
  ''
FROM (
  SELECT 'project' AS resource UNION ALL
  SELECT 'script' UNION ALL
  SELECT 'storyboard' UNION ALL
  SELECT 'style' UNION ALL
  SELECT 'image' UNION ALL
  SELECT 'short_video' UNION ALL
  SELECT 'full_video' UNION ALL
  SELECT 'pipeline' UNION ALL
  SELECT 'upload'
) AS r
CROSS JOIN (
  SELECT 'create' AS action UNION ALL
  SELECT 'update'
) AS a;

UPDATE `review_flows`
SET `is_default` = 0
WHERE `target_type` = 'full_video';

INSERT INTO `review_flows` (`name`,`description`,`target_type`,`enabled`,`is_default`)
SELECT
  'default-single-step-review',
  'single step review flow for admin',
  'full_video',
  1,
  1
WHERE NOT EXISTS (
  SELECT 1
  FROM `review_flows`
  WHERE `name` = 'default-single-step-review'
    AND `target_type` = 'full_video'
);

UPDATE `review_flows`
SET
  `description` = 'single step review flow for admin',
  `enabled` = 1,
  `is_default` = 1
WHERE `name` = 'default-single-step-review'
  AND `target_type` = 'full_video';

DELETE rn
FROM `review_nodes` AS rn
JOIN `review_flows` AS rf ON rf.id = rn.flow_id
WHERE rf.`name` = 'default-single-step-review'
  AND rf.`target_type` = 'full_video';

INSERT INTO `review_nodes`
  (`flow_id`,`step_no`,`name`,`approver_type`,`approver_value`,`allow_timeout_pass`,`timeout_hours`)
SELECT
  rf.id,
  1,
  'admin approval',
  'role',
  'super_admin',
  1,
  24
FROM `review_flows` AS rf
WHERE rf.`name` = 'default-single-step-review'
  AND rf.`target_type` = 'full_video'
LIMIT 1;

INSERT INTO `billing_quotas`
  (`scope_type`,`scope_id`,`model_id`,`period`,`metric`,`quota_value`,`enabled`)
SELECT
  'user',
  u.id,
  0,
  'monthly',
  'invocations',
  1000,
  1
FROM `users` AS u
WHERE u.`username` = 'admin'
  AND NOT EXISTS (
    SELECT 1
    FROM `billing_quotas` AS bq
    WHERE bq.`scope_type` = 'user'
      AND bq.`scope_id` = u.id
      AND bq.`model_id` = 0
      AND bq.`period` = 'monthly'
      AND bq.`metric` = 'invocations'
  );
