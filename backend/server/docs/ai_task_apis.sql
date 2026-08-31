-- =====================================================
-- AI 执行任务（AI执行记录）API 数据
-- 表结构 addon_quant_ai_task 已由后端 AutoMigrate 自动创建，无需手动建表
-- 执行本脚本前请先确认已重启后端服务（完成自动迁移）
-- =====================================================

-- 1. 注册 API 到权限表（必须执行，否则接口无法被访问）
INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`) VALUES
('/quant/aiTask/getAiTask', '根据ID获取AI执行任务', 'AI执行记录', 'GET'),
('/quant/aiTask/getAiTaskList', '分页获取AI执行任务列表', 'AI执行记录', 'GET'),
('/quant/aiTask/stopAiTask', '停止运行中的AI执行任务', 'AI执行记录', 'POST'),
('/quant/aiTask/restartAiTask', '重启已取消的AI执行任务', 'AI执行记录', 'POST');

-- 1.1 将新注册的 API 授权给角色（示例：超级管理员 authority_id=1）
--     其他角色请在系统"API管理"→"API分配"中为对应角色勾选这两个接口
-- INSERT INTO `sys_authority_apis` (`sys_authority_authority_id`, `sys_api_id`)
-- SELECT 1, id FROM `sys_apis` WHERE `api_group` = 'AI执行记录';

-- 查询验证
SELECT * FROM `sys_apis` WHERE `api_group` = 'AI执行记录';

-- =====================================================
-- 2. 新增侧边栏菜单"AI执行记录"（参考，请按实际菜单结构调整）
--    菜单父级建议挂在"量化管理"或"题材选股"菜单下，
--    以下示例使用子查询按名称定位父菜单，若未找到父菜单将不会插入。
--    component 对应前端页面 web/src/view/quant/aiTask/index.vue
-- =====================================================

-- 2.1 在父菜单"题材选股"下新增"AI执行记录"子菜单
-- INSERT INTO `sys_base_menus`
--   (`parent_id`, `path`, `name`, `hidden`, `component`, `sort`, `menu_level`,
--    `title`, `icon`, `keep_alive`, `default_menu`, `close_tab`, `active_name`)
-- SELECT
--   m.id, 'aiTask', 'AiTask', 0, 'view/quant/aiTask/index.vue', 5, 0,
--   'AI执行记录', 'monitor', 0, 0, 0, ''
-- FROM `sys_base_menus` m
-- WHERE m.name = 'themeStock' AND m.parent_id <> 0
-- LIMIT 1;

-- 2.2 若上面未插入（找不到父菜单），请先在系统"菜单管理"页面手动添加：
--     菜单名称：AI执行记录
--     路由 path：aiTask
--     组件路径：view/quant/aiTask/index.vue
--     父级：题材选股（或量化管理）
--     图标：monitor

-- 2.3 将菜单授权给角色（示例：授权给超级管理员 authority_id=1）
-- INSERT INTO `sys_authority_menus` (`sys_authority_authority_id`, `sys_base_menu_id`)
-- SELECT 1, id FROM `sys_base_menus` WHERE name = 'AiTask';
