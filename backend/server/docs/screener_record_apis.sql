-- AI选股器查询记录 API 数据
-- 如果数据库已经初始化，可以直接执行此SQL插入API记录

INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`) VALUES
('/quant/screenerRecord/createScreenerRecord', '创建AI选股器查询记录', 'AI选股器查询记录', 'POST'),
('/quant/screenerRecord/deleteScreenerRecord', '删除AI选股器查询记录', 'AI选股器查询记录', 'DELETE'),
('/quant/screenerRecord/deleteScreenerRecordByIds', '批量删除AI选股器查询记录', 'AI选股器查询记录', 'DELETE'),
('/quant/screenerRecord/updateScreenerRecord', '更新AI选股器查询记录', 'AI选股器查询记录', 'PUT'),
('/quant/screenerRecord/findScreenerRecord', '根据ID获取AI选股器查询记录', 'AI选股器查询记录', 'GET'),
('/quant/screenerRecord/getScreenerRecordList', '获取AI选股器查询记录列表', 'AI选股器查询记录', 'GET');

-- 查询验证
SELECT * FROM `sys_apis` WHERE `api_group` = 'AI选股器查询记录';
