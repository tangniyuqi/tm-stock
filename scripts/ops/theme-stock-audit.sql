-- =============================================================================
-- addon_quant_theme_stock 只读核查（P0-1 / docs/specs/ai-analysis / ADR-0008）
--
-- 目的：在对上线库执行 server/migrations/20261002_converge_addon_quant_theme_stock.sql 之前，
--       弄清楚这张表现在是什么形态、有多少数据、旧 GVA 的 AI 留下了什么、令牌类型强制检查会波及多少会员。
--
-- 本脚本只含 SELECT / SHOW / SET @变量 / PREPARE（且被 PREPARE 的都是 SELECT 与 SHOW），
-- 不建表、不改表、不写数据；会话变量随连接结束消失。表或列不存在时如实打印"不存在"，不会报错中止。
--
-- 怎么跑（用只读账号，最好在上线库的副本上；不要加 --force）：
--   mysql --default-character-set=utf8mb4 -D <库名> < scripts/ops/theme-stock-audit.sql > audit-<日期>.txt
--
-- 输出只含计数与表结构，不含摘录正文、手机号等个人信息。结果归档进变更单，不要提交到仓库。
--
-- 本脚本回答不了、必须人工回答并记入变更单的问题，见文末。
-- =============================================================================

SET NAMES utf8mb4;

-- 没选库就中止：information_schema 的查询会静默返回空，后面的输出会误导人
SET @tm_db = DATABASE();
SET @tm_sql = IF(@tm_db IS NULL, 'SELECT * FROM `tm_audit_abort__no_database_selected`', 'DO 0');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 1. 环境 ────────────────────────────────────────────────────────────────────
SELECT @tm_db AS `1_当前库`, VERSION() AS `mysql版本`, @@global.transaction_isolation AS `事务隔离级别`,
       @@collation_database AS `库默认排序规则`, @@sql_mode AS `sql_mode`;

-- ── 2. 相关表（行数为 information_schema 的估计值，仅供参考）─────────────────────
SELECT table_name AS `2_相关表`, engine AS `引擎`, table_rows AS `行数(估计)`, create_time AS `创建时间`
  FROM information_schema.tables
 WHERE table_schema = @tm_db
   AND ( table_name IN ('addon_quant_theme_stock', 'addon_quant_theme', 'addon_quant_base_stock',
                        'addon_quant_ai_task', 'addon_member', 'addon_member_asset', 'addon_member_asset_log')
         OR table_name LIKE 'tm\_theme\_stock\_legacy\_%'
         OR table_name LIKE 'tm\_theme\_stock\_snapshot\_%' )
 ORDER BY table_name;

-- ── 3. 形态判定 ────────────────────────────────────────────────────────────────
SET @tm_exists = (SELECT COUNT(*) FROM information_schema.tables
                   WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock');
SET @tm_ev_cols = (SELECT COUNT(*) FROM information_schema.columns
                    WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                      AND column_name IN ('ts_code', 'source_type', 'source_excerpt', 'source_url', 'collected_at'));
SET @tm_audit_cols = (SELECT COUNT(*) FROM information_schema.columns
                       WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                         AND column_name IN ('audit_status', 'audit_by', 'audit_at', 'reject_reason'));
SET @tm_legacy_cols = (SELECT COUNT(*) FROM information_schema.columns
                        WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                          AND column_name IN ('reason', 'ai_reason', 'tier', 'relevance', 'in_date'));
SET @tm_chk = (SELECT COUNT(*) FROM information_schema.table_constraints
                WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                  AND constraint_type = 'CHECK' AND constraint_name = 'chk_theme_stock_evidence');
SET @tm_uk = (SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
               WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                 AND index_name = 'uk_theme_stock_alive');
SET @tm_up_exists = (SELECT COUNT(*) FROM information_schema.tables
                      WHERE table_schema = @tm_db AND table_name IN ('addon_quant_theme', 'addon_quant_base_stock'));

SELECT CASE
         WHEN @tm_exists = 0 THEN '表不存在：收敛迁移会按权威定义建表'
         WHEN @tm_ev_cols = 0 THEN '旧 GVA 形态（没有依据列）：收敛迁移会整表封存后重建，旧行不会进入新表'
         WHEN @tm_ev_cols = 5 AND @tm_audit_cols = 4 AND @tm_legacy_cols = 0 AND @tm_chk = 1 AND @tm_uk = 1
              THEN '权威形态（请再核对下面的列属性与建表语句）'
         WHEN @tm_ev_cols = 5 THEN '混合形态（权威表被旧 GVA 改写过，或缺约束）：收敛迁移会原地修复，必要时先做快照'
         ELSE '认不出的形态（只有部分依据列）：收敛迁移会主动中止，须人工处理'
       END AS `3_形态判定`,
       @tm_exists AS `表存在`, @tm_ev_cols AS `依据列数(应为5)`, @tm_audit_cols AS `审核列数(应为4)`,
       @tm_legacy_cols AS `旧GVA列数(应为0)`, @tm_chk AS `依据CHECK(应为1)`, @tm_uk AS `存活唯一键(应为1)`;

-- 列属性：被旧 GVA 改写的典型症状是 theme_id/stock_id 变成可空、theme_id 变成 int、status 失去默认值
SET @tm_sql = IF(@tm_exists = 1,
  'SELECT column_name AS `3_列`, column_type AS `类型`, is_nullable AS `可空`, column_default AS `默认值` FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ''addon_quant_theme_stock'' ORDER BY ordinal_position',
  'SELECT ''表不存在，无列可列'' AS `3_列`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

SET @tm_sql = IF(@tm_exists = 1, 'SHOW CREATE TABLE `addon_quant_theme_stock`', 'SELECT ''表不存在'' AS `3_建表语句`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 4. 数据量与分布（只统计未删除的行）──────────────────────────────────────────
SET @tm_sql = IF(@tm_exists = 1,
  'SELECT COUNT(*) AS `4_总行数`, SUM(deleted_at IS NULL) AS `未删除行数`, SUM(deleted_at IS NULL AND status IS NULL) AS `status为NULL的行(默认值被去掉的症状,C端不可见)` FROM `addon_quant_theme_stock`',
  'SELECT ''表不存在'' AS `4_总行数`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

SET @tm_sql = IF(@tm_exists = 1,
  'SELECT status AS `4_启用状态(1启用 0停用 NULL缺失)`, COUNT(*) AS `行数` FROM `addon_quant_theme_stock` WHERE deleted_at IS NULL GROUP BY status ORDER BY status',
  'SELECT ''表不存在'' AS `4_启用状态`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

SET @tm_sql = IF(@tm_exists = 1 AND @tm_audit_cols = 4,
  'SELECT audit_status AS `4_审核状态(0草稿 1待审 2已通过 3已驳回)`, COUNT(*) AS `行数` FROM `addon_quant_theme_stock` WHERE deleted_at IS NULL GROUP BY audit_status ORDER BY audit_status',
  IF(@tm_exists = 0, 'SELECT ''表不存在'' AS `4_审核状态`',
     'SELECT ''审核列不齐（旧 GVA 形态或被改坏），这些行没有可信的审核记录'' AS `4_审核状态`'));
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- 依据完整性：权威表有 CHECK 保证为 0；其他形态下才可能出现非 0
SET @tm_sql = IF(@tm_exists = 1 AND @tm_ev_cols = 5,
  'SELECT SUM(source_type IS NULL OR source_type <= 0) AS `4_缺依据类型`, SUM(source_excerpt IS NULL OR source_excerpt = '''') AS `缺摘录`, SUM(source_url IS NULL OR source_url = '''') AS `缺链接`, SUM(collected_at IS NULL) AS `缺采集时点`, SUM(CHAR_LENGTH(source_excerpt) < 8) AS `摘录不足8字(疑似占位)` FROM `addon_quant_theme_stock` WHERE deleted_at IS NULL',
  IF(@tm_exists = 0, 'SELECT ''表不存在'' AS `4_依据完整性`',
     'SELECT ''依据列不齐（旧 GVA 形态或被改坏）：这些行没有可信的依据'' AS `4_依据完整性`'));
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- 同一题材下同一股票重复挂接（旧形态没有唯一键，可能有重复）
SET @tm_sql = IF(@tm_exists = 1,
  'SELECT COUNT(*) AS `4_重复挂接的(题材,股票)组数` FROM (SELECT theme_id, stock_id FROM `addon_quant_theme_stock` WHERE deleted_at IS NULL GROUP BY theme_id, stock_id HAVING COUNT(*) > 1) d',
  'SELECT ''表不存在'' AS `4_重复挂接的(题材,股票)组数`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- 孤儿行：题材或股票已不存在（或已软删除）
SET @tm_sql = IF(@tm_exists = 1 AND @tm_up_exists = 2,
  'SELECT SUM(t.id IS NULL) AS `4_题材不存在或已删的行`, SUM(s.id IS NULL) AS `股票不存在或已删的行` FROM `addon_quant_theme_stock` ts LEFT JOIN `addon_quant_theme` t ON t.id = ts.theme_id AND t.deleted_at IS NULL LEFT JOIN `addon_quant_base_stock` s ON s.id = ts.stock_id AND s.deleted_at IS NULL WHERE ts.deleted_at IS NULL',
  'SELECT ''表或上游两表不存在，跳过孤儿检查'' AS `4_孤儿行`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 5. 收敛迁移之后 C 端能看到多少（可见规则见 ADR-0008）────────────────────────────
-- 注意：旧 GVA 形态的行没有依据，收敛后不会进入新表；这里只统计"当前就满足可见条件"的行
SET @tm_sql = IF(@tm_exists = 1 AND @tm_ev_cols = 5 AND @tm_audit_cols = 4 AND @tm_up_exists = 2,
  'SELECT COUNT(*) AS `5_当前即满足C端可见条件的关联数` FROM `addon_quant_theme_stock` ts JOIN `addon_quant_base_stock` s ON s.id = ts.stock_id AND s.deleted_at IS NULL JOIN `addon_quant_theme` t ON t.id = ts.theme_id AND t.deleted_at IS NULL WHERE ts.deleted_at IS NULL AND ts.audit_status = 2 AND ts.status = 1 AND ts.source_excerpt <> '''' AND ts.source_url <> ''''',
  'SELECT 0 AS `5_当前即满足C端可见条件的关联数`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 6. 旧 GVA 的 AI 在这张表里留下的东西（只在存在这五列时统计）──────────────────────
SET @tm_sql = IF(@tm_exists = 1 AND @tm_legacy_cols = 5,
  'SELECT SUM(tier IS NOT NULL AND tier > 0) AS `6_带梯队的行`, SUM(relevance IS NOT NULL AND relevance > 0) AS `带相关度的行`, SUM(ai_reason IS NOT NULL AND ai_reason <> '''') AS `带AI入选逻辑的行`, SUM(reason IS NOT NULL AND reason <> '''') AS `带入选逻辑的行` FROM `addon_quant_theme_stock` WHERE deleted_at IS NULL',
  'SELECT ''没有（或不全有）旧 GVA 的评价类列，无需统计'' AS `6_旧GVA评价类内容`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 7. GVA 的 AI 任务（三类题材股票任务已下线，待调度的遗留任务会被落成失败）──────────
SET @tm_ai_task = (SELECT COUNT(*) FROM information_schema.tables
                    WHERE table_schema = @tm_db AND table_name = 'addon_quant_ai_task');
SET @tm_sql = IF(@tm_ai_task = 1,
  'SELECT type AS `7_任务类型`, status AS `状态(0运行 1成功 2失败 3取消 4待调度)`, COUNT(*) AS `任务数`, MIN(created_at) AS `最早创建`, MAX(created_at) AS `最近创建` FROM `addon_quant_ai_task` WHERE deleted_at IS NULL GROUP BY type, status ORDER BY type, status',
  'SELECT ''addon_quant_ai_task 不存在'' AS `7_任务类型`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

SET @tm_sql = IF(@tm_ai_task = 1,
  'SELECT COUNT(*) AS `7_待调度的已下线类型任务数` FROM `addon_quant_ai_task` WHERE deleted_at IS NULL AND status = 4 AND type IN (''ai_add'', ''ai_update_one'', ''ai_update_batch'')',
  'SELECT 0 AS `7_待调度的已下线类型任务数`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 8. 基础股票 AI 分析的存量（requirements F15 / D16）────────────────────────────
SET @tm_bs_ai = (SELECT COUNT(*) FROM information_schema.columns
                  WHERE table_schema = @tm_db AND table_name = 'addon_quant_base_stock'
                    AND column_name IN ('ai_analyzed_at', 'fundamentals', 'financial', 'realization', 'momentum', 'risk'));
SET @tm_sql = IF(@tm_bs_ai = 6,
  'SELECT COUNT(*) AS `8_股票总数`, SUM(ai_analyzed_at IS NOT NULL) AS `已有AI分析的股票数`, SUM(fundamentals IS NOT NULL AND fundamentals <> '''') AS `有AI基本面文本`, SUM(financial IS NOT NULL AND financial <> '''') AS `有AI财务文本`, SUM(risk IS NOT NULL AND risk <> '''') AS `有AI风险文本` FROM `addon_quant_base_stock` WHERE deleted_at IS NULL',
  'SELECT ''addon_quant_base_stock 没有 AI 分析相关列（或表不存在）'' AS `8_基础股票AI分析`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 9. 会员数量：令牌类型强制检查上线后，已登录会员需重新登录一次，这里估个规模 ─────────
SET @tm_member = (SELECT COUNT(*) FROM information_schema.tables
                   WHERE table_schema = @tm_db AND table_name = 'addon_member');
SET @tm_sql = IF(@tm_member = 1,
  'SELECT COUNT(*) AS `9_会员总数`, SUM(status = 1) AS `启用中的会员数` FROM `addon_member` WHERE deleted_at IS NULL',
  'SELECT ''addon_member 不存在'' AS `9_会员总数`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- =============================================================================
-- 必须人工回答、记入变更单的问题（SQL 查不出来）
--   1. gin-vue-admin 的 BSL 商业授权是否已采购？凭证放在哪里？（ADR-0004；未确认前，GVA 后台不得处理真实业务数据）
--   2. 现在的 GVA 是否对公网开放、是否已在处理真实用户数据？（决定 BSL 开发期豁免是否还成立）
--   3. 除 tm-stock 与 GVA 外，还有哪些系统读写 addon_quant_* 这几张表？（共用表的改动要先通知它们）
--   4. 上线库的 MySQL 是 9.7.0（ADR-0007）。本脚本与收敛迁移已在 8.0.46 与 9.7.0 上实测（含多种表形态），但上线库的真实形态未知：先在目标实例的副本上演练一遍。
--   5. 执行收敛迁移前，先确认新版 GVA（已从 AutoMigrate 移除 ThemeStock）已部署；旧版 GVA 重启会再次改坏这张表。
-- =============================================================================
