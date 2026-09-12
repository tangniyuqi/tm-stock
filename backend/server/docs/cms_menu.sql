-- =============================================================================
-- CMS 内容管理模块子菜单补数据脚本（幂等，可安全重复执行）
-- 适用场景：新增 ad / page / article / feedback 四个功能对应的菜单，
--           父级分组已存在（sys_base_menus.id = 118），本脚本只在其下挂 4 个子菜单。
-- 菜单字段风格与 server/source/system/menu.go 及 menu_supplement_gva30.sql 保持一致。
--
-- 层级设计：
--   内容管理(cms)  [父级分组，id=118]
--     ├── 广告管理(ad)
--     ├── 单页管理(page)
--     ├── 文章管理(article)
--     └── 反馈管理(feedback)
--
-- 说明：
--   1. 只做“补齐缺失”的增量操作，不重复插入已存在菜单，也不改动已有菜单数据。
--   2. 子菜单 component 均对应真实前端页面（已在前端 web/src 下验证存在）。
--   3. 角色关联：默认给 888（超级管理员）全量。如需其他角色，参照末尾注释补充。
--   4. 菜单图标为空心线框风格（Element Plus 图标小写串，同 menu.go 写法）。
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 二、子菜单（父级固定为 id=118，幂等）
-- -----------------------------------------------------------------------------

-- 广告管理 / ad
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT 118, 1, 'ad', 'ad', 'view/cms/ad/ad.vue', 1, 0, '广告管理', 'megaphone', 0, NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'ad' AND `parent_id` = 118 AND `deleted_at` IS NULL);

-- 单页管理 / page
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT 118, 1, 'page', 'page', 'view/cms/page/page.vue', 2, 0, '单页管理', 'document', 0, NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'page' AND `parent_id` = 118 AND `deleted_at` IS NULL);

-- 文章管理 / article
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT 118, 1, 'article', 'article', 'view/cms/article/article.vue', 3, 0, '文章管理', 'notebook', 0, NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'article' AND `parent_id` = 118 AND `deleted_at` IS NULL);

-- 反馈管理 / feedback
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT 118, 1, 'feedback', 'feedback', 'view/cms/feedback/feedback.vue', 4, 0, '反馈管理', 'chat-dot-round', 0, NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'feedback' AND `parent_id` = 118 AND `deleted_at` IS NULL);

-- -----------------------------------------------------------------------------
-- 三、角色-菜单关联（sys_authority_menus）
-- -----------------------------------------------------------------------------

-- 888（超级管理员）：补齐父级分组(118)及其全部子菜单关联（幂等）
INSERT INTO sys_authority_menus (`sys_base_menu_id`, `sys_authority_authority_id`)
SELECT m.`id`, '888' FROM sys_base_menus m
WHERE m.`deleted_at` IS NULL
  AND (m.`id` = 118 OR m.`parent_id` = 118)
  AND NOT EXISTS (SELECT 1 FROM sys_authority_menus a WHERE a.`sys_base_menu_id` = m.`id` AND a.`sys_authority_authority_id` = '888');

-- 如需给其他角色（如 9528）同样授权，可复制上一条并把权限 id 改成对应角色；
-- 也可在「角色管理」中为目标角色勾选 cms 分组及子菜单来补齐。

-- =============================================================================
-- 执行完成后的注意项：
--   1. 执行后 888 登录刷新即可看到「内容管理」下的四个子菜单。
--   2. 新增菜单对应 API 与 Casbin 策略需另行同步：前端「菜单管理 → api管理」点
--      「同步Api」从后端代码补齐；再在「角色管理」编辑 888 用「一键授权」补齐策略。
-- =============================================================================
