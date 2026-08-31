-- =============================================================================
-- GVA 3.0 菜单增量补数据脚本（幂等，可安全重复执行）
-- 适用场景：线上库 go_noooya_com 在旧版本时期初始化，遗漏 3.0 新增菜单，
--           而 server/source/system/menu.go 的 DataInserted 只检查 dashboard，
--           不会自动补齐（参见 menu.go 中 initMenu.DataInserted 逻辑）。
--
-- 说明：
--   1. 本脚本只做“补齐缺失”的增量操作，不会重复插入已存在的菜单，
--      也不会改动已存在的菜单数据，可重复执行。
--   2. 菜单字段与 server/source/system/menu.go 中的定义逐项一致。
--   3. 角色关联与 server/source/system/authorities_menus.go 保持一致：
--      - 888（超级管理员）：拥有全部菜单
--      - 9528（测试角色）：全部父级菜单 + systemTools/example 子菜单
--      - 8881（普通用户）：保持基础菜单，不在此范围
--   4. 执行前建议先备份：mysqldump -h 119.29.82.70 go_noooya_com sys_base_menus sys_authority_menus > menu_backup.sql
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 一、顶层菜单（ParentId = 0）
-- -----------------------------------------------------------------------------
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'dashboard', 'dashboard', 'view/dashboard/index.vue', 1, 0, '仪表盘', 'odometer', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'dashboard' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'permission', 'permission', 'view/routerHolder.vue', 2, 0, '权限管理', 'perm-gva', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'permission' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'org', 'org', 'view/routerHolder.vue', 3, 0, '组织管理', 'share', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'org' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'systemConfig', 'systemConfig', 'view/routerHolder.vue', 4, 0, '系统设置', 'config-gva', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'systemConfig' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'monitor', 'monitor', 'view/routerHolder.vue', 5, 0, '运维监控', 'monitor-gva', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'monitor' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'media', 'media', 'view/routerHolder.vue', 6, 0, '媒体管理', 'folder-opened', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'media' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'systemTools', 'systemTools', 'view/routerHolder.vue', 7, 0, '编程辅助', 'cpu', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'systemTools' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'ai', 'ai', 'view/routerHolder.vue', 8, 0, 'AI 工坊', 'ai-gva', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'ai' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'example', 'example', 'view/example/index.vue', 9, 0, '示例文件', 'example-gva', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'example' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'plugin', 'plugin', 'view/routerHolder.vue', 10, 0, '插件系统', 'cherry', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'plugin' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'https://www.gin-vue-admin.com', 'https://www.gin-vue-admin.com', '/', 11, 0, '官方网站', 'customer-gva', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'https://www.gin-vue-admin.com' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'about', 'about', 'view/about/index.vue', 12, 0, '关于我们', 'office-building', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'about' AND `parent_id` = 0 AND `deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `active_name`, `default_menu`, `close_tab`, `transition_type`, `created_at`, `updated_at`)
SELECT 0, 0, 'person', 'person', 'view/person/person.vue', 13, 1, '个人信息', 'postcard', 0, '', 0, 0, '', NOW(), NOW()
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE `name` = 'person' AND `parent_id` = 0 AND `deleted_at` IS NULL);

-- -----------------------------------------------------------------------------
-- 二、子菜单（ParentId 通过父级 name 子查询，幂等）
-- -----------------------------------------------------------------------------

-- 权限管理 / permission
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'authority', 'authority', 'view/superAdmin/authority/authority.vue', 1, 0, '角色管理', 'role-gva', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'permission' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'authority' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'menu', 'menu', 'view/superAdmin/menu/menu.vue', 2, 0, '菜单管理', 'tickets', 1, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'permission' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'menu' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'api', 'api', 'view/superAdmin/api/api.vue', 3, 0, 'api管理', 'api-gva', 1, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'permission' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'api' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'apiToken', 'apiToken', 'view/systemTools/apiToken/index.vue', 4, 0, 'API Token', 'key', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'permission' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'apiToken' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- 组织管理 / org
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'user', 'user', 'view/superAdmin/user/user.vue', 1, 0, '用户管理', 'user', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'org' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'user' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'department', 'department', 'view/superAdmin/department/department.vue', 2, 0, '部门管理', 'office-building', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'org' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'department' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'position', 'position', 'view/superAdmin/position/position.vue', 3, 0, '岗位管理', 'postcard', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'org' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'position' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- 系统设置 / systemConfig
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'system', 'system', 'view/systemTools/system/system.vue', 1, 0, '配置文件', 'config-file-gva', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemConfig' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'system' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'dictionary', 'dictionary', 'view/superAdmin/dictionary/sysDictionary.vue', 2, 0, '字典管理', 'notebook', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemConfig' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'dictionary' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'sysParams', 'sysParams', 'view/superAdmin/params/sysParams.vue', 3, 0, '参数管理', 'set-up', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemConfig' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'sysParams' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'security', 'security', 'view/system/security/index.vue', 4, 0, '安全配置', 'security-gva', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemConfig' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'security' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- 运维监控 / monitor
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'operation', 'operation', 'view/superAdmin/operation/sysOperationRecord.vue', 1, 0, '操作历史', 'document', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'operation' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'loginLog', 'loginLog', 'view/systemTools/loginLog/index.vue', 2, 0, '登录日志', 'clock', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'loginLog' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'sysError', 'sysError', 'view/systemTools/sysError/sysError.vue', 3, 0, '错误日志', 'error-gva', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'sysError' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'sysVersion', 'sysVersion', 'view/systemTools/version/version.vue', 4, 0, '版本管理', 'version-gva', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'sysVersion' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'state', 'state', 'view/system/state.vue', 5, 0, '服务器状态', 'server', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'state' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'dataAccessLog', 'dataAccessLog', 'view/superAdmin/dataAccessLog/dataAccessLog.vue', 6, 0, '数据权限审计', 'warning', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'dataAccessLog' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'timedTask', 'timedTask', 'view/systemTools/timedTask/index.vue', 7, 0, '定时任务', 'timer', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'timedTask' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'logViewer', 'logViewer', 'view/systemTools/logViewer/index.vue', 8, 0, '文件日志', 'document', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'monitor' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'logViewer' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- 媒体管理 / media
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'upload', 'upload', 'view/media/upload.vue', 1, 0, '媒体库（上传下载）', 'upload', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'media' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'upload' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'chunkUpload', 'chunkUpload', 'view/media/chunkUpload.vue', 2, 0, '大文件上传', 'folder-add', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'media' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'chunkUpload' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- 示例文件 / example
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'customer', 'customer', 'view/example/customer/customer.vue', 1, 0, '客户列表（资源示例）', 'service', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'example' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'customer' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- 编程辅助 / systemTools
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'autoCode', 'autoCode', 'plugin/auto/view/autoCode/index.vue', 1, 0, '代码生成器', 'magic-stick', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemTools' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'autoCode' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'autoPkg', 'autoPkg', 'plugin/auto/view/autoPkg/autoPkg.vue', 2, 0, '模板配置', 'files', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemTools' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'autoPkg' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'autoCodeAdmin', 'AutoCodeAdmin', 'plugin/auto/view/autoCodeAdmin/index.vue', 3, 0, '自动代码管理', 'file-code-2-gva', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemTools' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'AutoCodeAdmin' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'formCreate', 'formCreate', 'plugin/auto/view/formCreate/index.vue', 4, 0, '表单生成器', 'edit', 1, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemTools' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'formCreate' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'autoCodeEdit/:id', 'autoCodeEdit', 'plugin/auto/view/autoCode/index.vue', 0, 1, '自动化代码-${id}', 'magic-stick', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemTools' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'autoCodeEdit' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'exportTemplate', 'exportTemplate', 'plugin/auto/view/exportTemplate/exportTemplate.vue', 6, 0, '导出模板', 'download', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'systemTools' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'exportTemplate' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- AI 工坊 / ai
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'mcpTool', 'mcpTool', 'plugin/ai/view/mcp/mcp.vue', 1, 0, 'Mcp Tools模板', 'grid', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'ai' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'mcpTool' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'mcpTest', 'mcpTest', 'plugin/ai/view/mcp/mcpTest.vue', 2, 0, 'Mcp Tools管理', 'connection', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'ai' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'mcpTest' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'mcpApi', 'McpApi', 'plugin/ai/view/mcpApi/index.vue', 3, 0, 'AI MCP构建', 'set-up', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'ai' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'McpApi' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'skills', 'Skills', 'plugin/ai/view/skills/index.vue', 4, 0, 'Skills管理', 'edit-pen', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'ai' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'Skills' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'cli', 'Cli', 'plugin/ai/view/cli/index.vue', 5, 0, 'AI CLI管理', 'monitor', 1, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'ai' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'Cli' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'picture', 'picture', 'plugin/ai/view/picture/picture.vue', 6, 0, 'AI页面绘制', 'picture', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'ai' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'picture' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- 插件系统 / plugin
INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'https://plugin.gin-vue-admin.com/', 'https://plugin.gin-vue-admin.com/', 'https://plugin.gin-vue-admin.com/', 0, 0, '插件市场', 'shop', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'plugin' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'https://plugin.gin-vue-admin.com/' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'installPlugin', 'installPlugin', 'view/systemTools/installPlugin/index.vue', 1, 0, '插件安装', 'box', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'plugin' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'installPlugin' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'pubPlug', 'pubPlug', 'view/systemTools/pubPlug/pubPlug.vue', 3, 0, '打包插件', 'suitcase', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'plugin' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'pubPlug' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'plugin-email', 'plugin-email', 'plugin/email/view/index.vue', 4, 0, '邮件插件', 'message', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'plugin' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'plugin-email' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

INSERT INTO sys_base_menus (`parent_id`, `menu_level`, `path`, `name`, `component`, `sort`, `hidden`, `title`, `icon`, `keep_alive`, `created_at`, `updated_at`)
SELECT p.id, 1, 'anInfo', 'anInfo', 'plugin/announcement/view/info.vue', 5, 0, '公告管理[示例]', 'bell', 0, NOW(), NOW()
FROM sys_base_menus p WHERE p.`name` = 'plugin' AND p.`parent_id` = 0 AND p.`deleted_at` IS NULL
AND NOT EXISTS (SELECT 1 FROM sys_base_menus c WHERE c.`name` = 'anInfo' AND c.`parent_id` = p.id AND c.`deleted_at` IS NULL);

-- -----------------------------------------------------------------------------
-- 三、角色-菜单关联（sys_authority_menus）
-- -----------------------------------------------------------------------------

-- 888（超级管理员）：补齐全部菜单关联（幂等）
INSERT INTO sys_authority_menus (`sys_base_menu_id`, `sys_authority_authority_id`)
SELECT m.`id`, '888' FROM sys_base_menus m
WHERE m.`deleted_at` IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_authority_menus a WHERE a.`sys_base_menu_id` = m.`id` AND a.`sys_authority_authority_id` = '888');

-- 9528（测试角色）：全部父级菜单 + systemTools/example 子菜单
INSERT INTO sys_authority_menus (`sys_base_menu_id`, `sys_authority_authority_id`)
SELECT m.`id`, '9528' FROM sys_base_menus m
WHERE m.`deleted_at` IS NULL
  AND (m.`parent_id` = 0 OR EXISTS (
        SELECT 1 FROM sys_base_menus p
        WHERE p.`id` = m.`parent_id` AND p.`deleted_at` IS NULL
          AND p.`parent_id` = 0
          AND p.`name` IN ('systemTools', 'example')
      ))
  AND NOT EXISTS (SELECT 1 FROM sys_authority_menus a WHERE a.`sys_base_menu_id` = m.`id` AND a.`sys_authority_authority_id` = '9528');

-- =============================================================================
-- 执行完成后的注意项：
--   1. 执行后菜单前端即可显示（888/9528 登录后刷新即可）。
--   2. 新增菜单对应的 API 与 Casbin 策略：
--      - API 表（sys_apis）：请在前端「菜单管理 → api管理」页面点击「同步Api」按钮，
--        从后端代码自动同步补齐（需 888 且服务端已加载最新路由）。
--      - Casbin 权限（casbin_rule）：请在「角色管理」中编辑 888，使用「全部勾选/一键授权」补齐。
--        若线上库为旧版本缺失 security/timedTask/logViewer 等策略，也可参照
--        server/initialize/log_viewer_seed.go 的 EnsureLogViewerData 方式在服务端补齐。
-- =============================================================================