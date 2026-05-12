-- =============================================================
-- AI 短剧视频生成平台 · 种子数据  v1.0
-- 执行顺序在 001_init.sql 之后
-- =============================================================

SET NAMES utf8mb4;

-- -------------------------------------------------------------
-- 部门
-- -------------------------------------------------------------
INSERT INTO `departments` (`id`,`name`,`parent_id`,`path`,`sort`,`status`) VALUES
  (1, '总部',    0, '/1',     1, 1),
  (2, '内容中心', 1, '/1/2',   2, 1),
  (3, '技术中心', 1, '/1/3',   3, 1),
  (4, '运营中心', 1, '/1/4',   4, 1);

-- -------------------------------------------------------------
-- 权限点
-- -------------------------------------------------------------
INSERT INTO `permissions` (`code`,`name`,`resource`,`action`,`description`) VALUES
  ('project:read',      '查看项目',   'project',     'read',   ''),
  ('project:write',     '编辑项目',   'project',     'write',  ''),
  ('project:delete',    '删除项目',   'project',     'delete', ''),
  ('script:read',       '查看剧本',   'script',      'read',   ''),
  ('script:write',      '编辑剧本',   'script',      'write',  ''),
  ('prompt:read',       '查看提示词', 'prompt',      'read',   ''),
  ('prompt:write',      '编辑提示词', 'prompt',      'write',  ''),
  ('storyboard:read',   '查看分镜',   'storyboard',  'read',   ''),
  ('storyboard:write',  '编辑分镜',   'storyboard',  'write',  ''),
  ('style:read',        '查看风格',   'style',       'read',   ''),
  ('style:write',       '编辑风格',   'style',       'write',  ''),
  ('image:read',        '查看图片',   'image',       'read',   ''),
  ('image:write',       '生成图片',   'image',       'write',  ''),
  ('short_video:read',  '查看短视频', 'short_video', 'read',   ''),
  ('short_video:write', '生成短视频', 'short_video', 'write',  ''),
  ('full_video:read',   '查看完整视频','full_video', 'read',   ''),
  ('full_video:write',  '编辑完整视频','full_video', 'write',  ''),
  ('review:read',       '查看审核',   'review',      'read',   ''),
  ('review:write',      '执行审核',   'review',      'write',  ''),
  ('publish:write',     '发布视频',   'publish',     'write',  ''),
  ('user:read',         '查看用户',   'user',        'read',   ''),
  ('user:manage',       '管理用户',   'user',        'manage', ''),
  ('role:manage',       '管理角色',   'role',        'manage', ''),
  ('model:read',        '查看模型',   'model',       'read',   ''),
  ('model:manage',      '管理模型',   'model',       'manage', ''),
  ('billing:read',      '查看计费',   'billing',     'read',   ''),
  ('billing:manage',    '管理额度',   'billing',     'manage', ''),
  ('pipeline:read',     '查看流水线', 'pipeline',    'read',   ''),
  ('pipeline:write',    '编辑流水线', 'pipeline',    'write',  ''),
  ('pipeline:run',      '执行流水线', 'pipeline',    'run',    '');

-- -------------------------------------------------------------
-- 角色
-- -------------------------------------------------------------
INSERT INTO `roles` (`id`,`code`,`name`,`description`,`data_scope`,`is_system`) VALUES
  (1, 'super_admin',  '超级管理员', '所有权限',         'all',  1),
  (2, 'dept_admin',   '部门管理员', '部门级管理',       'dept', 1),
  (3, 'producer',     '制作人',     '创建/制作内容',    'self', 1),
  (4, 'editor',       '编辑/分镜师','编辑剧本/分镜',    'self', 1),
  (5, 'reviewer',     '审核员',     '审核内容',         'all',  1),
  (6, 'operator',     '运营',       '发布/下架',        'all',  1),
  (7, 'viewer',       '访客',       '只读已发布',       'all',  1);

-- 角色-权限(此处简化:超管 拥有全部;其余按角色矩阵选定)
INSERT INTO `role_permissions` (`role_id`,`permission_id`)
  SELECT 1, id FROM `permissions`;

-- 制作人:项目/剧本/提示词/分镜/风格/图片/短视频/完整视频/审核(提交)/流水线
INSERT INTO `role_permissions` (`role_id`,`permission_id`)
  SELECT 3, id FROM `permissions` WHERE `code` IN
    ('project:read','project:write',
     'script:read','script:write',
     'prompt:read','prompt:write',
     'storyboard:read','storyboard:write',
     'style:read','style:write',
     'image:read','image:write',
     'short_video:read','short_video:write',
     'full_video:read','full_video:write',
     'review:read',
     'model:read',
     'pipeline:read','pipeline:run');

-- 编辑/分镜师
INSERT INTO `role_permissions` (`role_id`,`permission_id`)
  SELECT 4, id FROM `permissions` WHERE `code` IN
    ('project:read',
     'script:read','script:write',
     'prompt:read','prompt:write',
     'storyboard:read','storyboard:write',
     'style:read','style:write',
     'image:read','image:write',
     'short_video:read','full_video:read');

-- 审核员
INSERT INTO `role_permissions` (`role_id`,`permission_id`)
  SELECT 5, id FROM `permissions` WHERE `code` IN
    ('project:read','script:read','prompt:read','storyboard:read','style:read',
     'image:read','short_video:read','full_video:read',
     'review:read','review:write');

-- 运营
INSERT INTO `role_permissions` (`role_id`,`permission_id`)
  SELECT 6, id FROM `permissions` WHERE `code` IN
    ('project:read','full_video:read','publish:write');

-- 部门管理员:基本读 + 用户/计费
INSERT INTO `role_permissions` (`role_id`,`permission_id`)
  SELECT 2, id FROM `permissions` WHERE `code` IN
    ('project:read','script:read','full_video:read',
     'user:read','user:manage',
     'billing:read','billing:manage');

-- 访客
INSERT INTO `role_permissions` (`role_id`,`permission_id`)
  SELECT 7, id FROM `permissions` WHERE `code` IN
    ('project:read','full_video:read');

-- -------------------------------------------------------------
-- 内置超管账户(用户名:admin / 密码:Admin@123,bcrypt cost=10)
-- 上线后请用 reset_password 接口立即修改
-- -------------------------------------------------------------
INSERT INTO `users`(`id`,`username`,`password_hash`,`nickname`,`email`,`dept_id`,`status`)
VALUES
  (1, 'admin',
      '$2a$10$9cxa7tVUgiWADsKLo5paFOyhiM0UjwY72DBPA2Eyi/9G5.iiTSbaC',
      '超级管理员','admin@example.com', 1, 1);

INSERT INTO `user_roles`(`user_id`,`role_id`) VALUES (1, 1);

-- -------------------------------------------------------------
-- 字典:状态、模型类型、分镜类型等
-- -------------------------------------------------------------
INSERT INTO `sys_dicts`(`type`,`code`,`name`,`value`,`sort`) VALUES
  ('project_status','1','草稿','draft',1),
  ('project_status','2','制作中','in_production',2),
  ('project_status','3','审核中','in_review',3),
  ('project_status','4','已发布','published',4),
  ('project_status','5','已归档','archived',5),
  ('model_type','text','文本模型','text',1),
  ('model_type','image','图像模型','image',2),
  ('model_type','video','视频模型','video',3),
  ('model_type','audio','音频模型','audio',4),
  ('shot_type','wide','远景','wide',1),
  ('shot_type','medium','中景','medium',2),
  ('shot_type','close','近景','close',3),
  ('shot_type','extreme_close','特写','extreme_close',4),
  ('shot_type','establishing','大远景','establishing',5),
  ('camera_motion','static','固定','static',1),
  ('camera_motion','pan','摇','pan',2),
  ('camera_motion','zoom','推/拉','zoom',3),
  ('camera_motion','dolly','移','dolly',4),
  ('camera_motion','track','跟','track',5);

-- -------------------------------------------------------------
-- 默认审核流(单级:制作人提交 -> 主管审核)
-- -------------------------------------------------------------
INSERT INTO `review_flows`(`id`,`name`,`description`,`target_type`,`enabled`,`is_default`)
VALUES (1,'默认审核流','单级,主管审核','full_video',1,1);

INSERT INTO `review_nodes`(`flow_id`,`step_no`,`name`,`approver_type`,`approver_value`)
VALUES (1,1,'主管审核','role','reviewer');

-- -------------------------------------------------------------
-- 预置流水线模板
-- -------------------------------------------------------------
INSERT INTO `pipelines`(`project_id`,`name`,`description`,`dag`,`is_template`,`enabled`,`created_by`)
VALUES
  (0, '全自动版',
   '剧本→拆分集→提示词→分镜→生图→生短视频',
   '{"nodes":[{"id":"n1","type":"script.split"},{"id":"n2","type":"prompt.generate"},{"id":"n3","type":"storyboard.generate"},{"id":"n4","type":"image.generate"},{"id":"n5","type":"video.generate"}],"edges":[{"from":"n1","to":"n2"},{"from":"n2","to":"n3"},{"from":"n3","to":"n4"},{"from":"n4","to":"n5"}]}',
   1, 1, 1),
  (0, '半自动到分镜',
   '剧本→提示词→分镜(人工继续)',
   '{"nodes":[{"id":"n1","type":"script.split"},{"id":"n2","type":"prompt.generate"},{"id":"n3","type":"storyboard.generate"}],"edges":[{"from":"n1","to":"n2"},{"from":"n2","to":"n3"}]}',
   1, 1, 1),
  (0, '仅生短视频',
   '直接图生视频',
   '{"nodes":[{"id":"n1","type":"video.generate"}],"edges":[]}',
   1, 1, 1);
