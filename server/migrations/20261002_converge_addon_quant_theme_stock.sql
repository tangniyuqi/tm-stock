-- =============================================================================
-- 把 addon_quant_theme_stock 收敛到权威定义（P0-2 / AC-O1，docs/specs/ai-analysis）
--
-- 为什么需要它（2026-10-02 在 MySQL 8.0.46 + GVA 同版本 GORM v1.31.1 上实测）：
--   GVA 的 initialize/gorm_biz.go 曾把 quant.ThemeStock{} 注册进 AutoMigrate，而本表同时是
--   tm-stock 的合规命门（依据 NOT NULL + CHECK，ADR-0003）。两个系统各管一半，结果是——
--   ① GVA 先启动：表是 GVA 旧形态——没有依据列、审核列、CHECK、唯一键，无依据行、重复行都能写入；
--      之后执行 20260730 迁移会报 ERROR 1050（表已存在）。
--   ② tm-stock 先建表、旧 GVA 后启动：AutoMigrate 静默改写——theme_id/stock_id 失去 NOT NULL
--      （theme_id 还从 bigint 收窄为 int）、status 失去 DEFAULT 1，并多出 reason/ai_reason/tier/
--      relevance/in_date 五列；依据列与 CHECK 保留，旧 GVA 的写入因此全部报错（依据列无默认值）。
--   这两种形态的真实 DDL 已存为测试夹具：server/internal/repository/testdata/theme_stock_old_gva_*.sql。
--
-- 本迁移做什么（幂等，可重复执行，任何顺序、任何已知形态都收敛到同一个权威结构）：
--   形态 0  表不存在                       → 按权威定义建表。
--   形态 1  旧 GVA 空库建的表（无依据列）    → 整表改名为 tm_theme_stock_legacy_<时间戳> 封存，再建权威表。
--           旧行没有依据，不能进入"无依据禁止入库"的表；也不伪造依据填充——依据必须是真的。封存表保持原样，
--           没有任何读取方；之后由内容团队带着真实依据重新录入。【C 端会因此暂时看不到这些旧关联。】
--   形态 2  权威表，或被旧 GVA 改写过的混合表 → 原地修复：恢复 theme_id/stock_id 的 NOT NULL 与 bigint
--           unsigned、status 的 DEFAULT 1，删除五个旧 GVA 列。删列或删行之前，若有数据可能丢失，
--           先复制到 tm_theme_stock_snapshot_<时间戳>（含旧 GVA 列的取值）。
--   其他形态（只有部分依据列、缺 theme_id/stock_id、修复后不满足断言）→ 主动报错中止，不猜、不硬改。
--           中止方式是执行一条引用名为 tm_migration_abort__<原因> 的不存在表的语句，
--           报错信息里就带着原因；mysql 命令行不要加 --force，否则不会停。
--
-- 本迁移不做什么：不删除任何数据表（封存表与快照表由运维确认无需追溯后手工清理）；
--   不碰 addon_quant_theme / addon_quant_base_stock。
--
-- 执行顺序与运维要点（先读这段）：
--   1. 先部署"已从 AutoMigrate 移除 quant.ThemeStock{}"的新版 GVA，再执行本迁移；
--      仍在运行的旧版 GVA 一旦重启，会把表再次改成形态 2 的混合表——这时重跑本迁移即可（幂等）。
--   2. 本迁移【不是事务】：DDL 会隐式提交。中途失败可以放心重跑。
--   3. 同一次执行内以 @tm_state 记录走了哪个分支（created / legacy_archived / repaired /
--      already_converged），结尾会输出，请把它连同封存/快照表名写进变更记录。
--   4. 与 20260730 成对使用：20260730 已改为 CREATE TABLE IF NOT EXISTS，两者先后顺序都安全。
--      两处的建表定义必须一致——AC-O1 的集成测试会逐项比对 SHOW CREATE TABLE，漂移会让 CI 失败。
--   5. 已在 MySQL 8.0.46 与 9.7.0 上实测（集成测试与 verify-migrations 两个版本都跑过，结果一致）；
--      但上线库的真实形态未知，正式执行前仍须在目标实例的副本上演练一遍。
--   6. 末尾会重算 addon_quant_theme.stock_count（题材上的"可见股票数量"，口径 = 审核已通过且启用且未删除）；
--      封存旧表后这个数会归零，C 端的"热度"随之下降，这是预期的，不是故障。
--   7. 被旧 GVA 改坏之后写入的行，status 可能是 NULL（默认值被去掉了）。本迁移不替运维做主，保持 NULL；
--      C 端只返回 status = 1 的行，所以这些行对 C 端不可见（安全方向）。需要上线的，核对依据后手工置 1。
-- =============================================================================

SET NAMES utf8mb4;

-- ── 0. 探测当前形态 ────────────────────────────────────────────────────────────
SET @tm_db = DATABASE();
SET @tm_ts = DATE_FORMAT(NOW(6), '%Y%m%d%H%i%s%f');   -- 精确到微秒，避免同一秒内重复执行时表名冲突
SET @tm_exists = (SELECT COUNT(*) FROM information_schema.tables
                   WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock');
SET @tm_ev_cols = (SELECT COUNT(*) FROM information_schema.columns
                    WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                      AND column_name IN ('ts_code', 'source_type', 'source_excerpt', 'source_url', 'collected_at'));
SET @tm_key_cols = (SELECT COUNT(*) FROM information_schema.columns
                     WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                       AND column_name IN ('theme_id', 'stock_id'));

-- 认不出的形态：直接中止
SET @tm_sql = IF(@tm_exists = 1 AND @tm_ev_cols NOT IN (0, 5),
  'SELECT * FROM `tm_migration_abort__theme_stock_only_some_evidence_columns`', 'DO 0');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

SET @tm_sql = IF(@tm_exists = 1 AND @tm_key_cols <> 2,
  'SELECT * FROM `tm_migration_abort__theme_stock_missing_theme_id_or_stock_id`', 'DO 0');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 1. 形态 1：旧 GVA 建的表（无依据列）整表封存 ───────────────────────────────
SET @tm_sql = IF(@tm_exists = 1 AND @tm_ev_cols = 0,
  CONCAT('RENAME TABLE `addon_quant_theme_stock` TO `tm_theme_stock_legacy_', @tm_ts, '`'), 'DO 0');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 2. 形态 0 / 封存之后：按权威定义建表（表已存在则什么也不做）────────────────
-- 与 20260730_addon_quant_theme_stock.sql 的建表语句逐字一致（仅多了 IF NOT EXISTS）。
CREATE TABLE IF NOT EXISTS `addon_quant_theme_stock` (
  `id`       bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',

  -- ── 关联主体 ──
  -- theme_id 允许指向【任意层级】的节点：
  --   题材不分环节 → 挂 level=1 的题材本身
  --   题材分环节   → 挂 level=2 的产业链环节（如 光刻机 › 光源）
  -- 这样将来加不加环节都不用改表结构。
  `theme_id` bigint UNSIGNED NOT NULL COMMENT '题材/环节ID（addon_quant_theme.id，可为任意层级）',
  `stock_id` bigint UNSIGNED NOT NULL COMMENT '股票ID（addon_quant_base_stock.id，权威字段）',
  -- ts_code 冗余存一份：CSV 批量导入与人工对账看的是代码不是自增 id，
  -- 有它才能在不 JOIN 的情况下审计导入结果。但【以 stock_id 为准】。
  `ts_code`  varchar(20) NOT NULL COMMENT 'TS代码冗余（仅供排查对账，权威以 stock_id 为准）',

  -- ── 🔴 归属依据（合规命门，四项缺一不可）──
  `source_type`    tinyint       NOT NULL COMMENT '依据类型：1公告 2年报 3招股书 4官方产业目录 5互动易问答',
  `source_excerpt` varchar(1000) NOT NULL COMMENT '原文摘录。禁止填“见链接”“详见公告”之类占位',
  `source_url`     varchar(512)  NOT NULL COMMENT '原文链接',
  `collected_at`   datetime(3)   NOT NULL COMMENT '采集时点',

  -- ── 审核（仅已通过的对 C 端可见）──
  -- 字段名用 audit_status 而不是 status：题材表里的 status 是【启用/停用】，
  -- 复用同名会造成语义冲突，日后必然有人搞混。
  `audit_status`  tinyint         NOT NULL DEFAULT 0 COMMENT '审核：0草稿 1待审 2已通过 3已驳回。仅 2 对 C 端可见',
  `audit_by`      bigint UNSIGNED DEFAULT NULL COMMENT '审核人',
  `audit_at`      datetime(3)     DEFAULT NULL COMMENT '审核时间',
  `reject_reason` varchar(250)    DEFAULT NULL COMMENT '驳回原因',

  -- ── 通用字段（沿用现有两表约定）──
  `remark`     varchar(250)    DEFAULT NULL COMMENT '备注',
  `sort`       int             DEFAULT 0 COMMENT '排序',
  `status`     tinyint         DEFAULT 1 COMMENT '状态：1启用 0停用',
  `created_at` datetime(3)     DEFAULT NULL COMMENT '创建时间',
  `updated_at` datetime(3)     DEFAULT NULL COMMENT '更新时间',
  `deleted_at` datetime(3)     DEFAULT NULL COMMENT '删除时间',
  `created_by` bigint UNSIGNED DEFAULT NULL COMMENT '创建者',
  `updated_by` bigint UNSIGNED DEFAULT NULL COMMENT '更新者',
  `deleted_by` bigint UNSIGNED DEFAULT NULL COMMENT '删除者',

  -- ── 软删除 + 唯一键的正确写法（见文末「三个坑」①②）──
  -- 生成列：存活为 1，已删除为 NULL。
  -- 唯一键含它之后：存活记录 (theme,stock,1) 唯一；
  -- 已删除记录全部是 NULL，而唯一键不约束 NULL → 同一对可以被反复删除再添加。
  -- ⚠️ 不要写成 IFNULL(UNIX_TIMESTAMP(deleted_at),0)：
  --    UNIX_TIMESTAMP 依赖会话时区、属非确定性函数，生成列直接拒绝建表
  --    （ERROR 3763，已实测）。
  `alive` tinyint GENERATED ALWAYS AS (IF(`deleted_at` IS NULL, 1, NULL)) STORED
          COMMENT '软删除唯一键辅助列：存活=1，已删除=NULL。勿手工写入',

  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_theme_stock_alive` (`theme_id`, `stock_id`, `alive`),
  KEY `idx_stock`        (`stock_id`),
  KEY `idx_theme_audit`  (`theme_id`, `audit_status`, `deleted_at`),
  KEY `idx_audit_status` (`audit_status`),
  KEY `idx_addon_quant_theme_stock_deleted_at` (`deleted_at`),

  -- 依据非空：NOT NULL 拦不住空串，只有 CHECK 能。
  -- （tm-stock 侧已在 MySQL 8.0.45 实测：空串触发
  --   ERROR 3819 Check constraint ... is violated）
  CONSTRAINT `chk_theme_stock_evidence` CHECK (
    `source_type` > 0 AND `source_excerpt` <> '' AND `source_url` <> ''
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='扩展_QUANT_题材股票关联表。🔴 无依据禁止入库（ADR-0003）';

-- ── 3. 形态 2：原地修复被旧 GVA 改写过的表 ─────────────────────────────────────
SET @tm_rows = (SELECT COUNT(*) FROM `addon_quant_theme_stock`);
SET @tm_null_keys = (SELECT COUNT(*) FROM `addon_quant_theme_stock`
                      WHERE `theme_id` IS NULL OR `stock_id` IS NULL);
SET @tm_legacy_cols = (SELECT COUNT(*) FROM information_schema.columns
                        WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                          AND column_name IN ('reason', 'ai_reason', 'tier', 'relevance', 'in_date'));
SET @tm_drop_clauses = (SELECT GROUP_CONCAT(CONCAT('DROP COLUMN `', column_name, '`')
                                            ORDER BY ordinal_position SEPARATOR ', ')
                          FROM information_schema.columns
                         WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                           AND column_name IN ('reason', 'ai_reason', 'tier', 'relevance', 'in_date'));
-- 三个被旧 GVA 改动的列是否都已回到权威定义（theme_id/stock_id 为 bigint unsigned NOT NULL，status 默认 1）
SET @tm_keys_ok = (SELECT COUNT(*) FROM information_schema.columns
                    WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                      AND ((column_name IN ('theme_id', 'stock_id')
                            AND column_type = 'bigint unsigned' AND is_nullable = 'NO')
                        OR (column_name = 'status'
                            AND column_type = 'tinyint' AND is_nullable = 'YES' AND column_default = '1')));

-- 3.1 可能丢数据时先留快照：有旧 GVA 列且表非空，或存在关联键为空的废行（即将被清理）
SET @tm_need_snapshot = IF((@tm_legacy_cols > 0 AND @tm_rows > 0) OR @tm_null_keys > 0, 1, 0);
SET @tm_snapshot = CONCAT('tm_theme_stock_snapshot_', @tm_ts);
SET @tm_cols = (SELECT GROUP_CONCAT(CONCAT('`', column_name, '`') ORDER BY ordinal_position SEPARATOR ', ')
                  FROM information_schema.columns
                 WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                   AND generation_expression = '');

SET @tm_sql = IF(@tm_need_snapshot = 1,
  CONCAT('CREATE TABLE `', @tm_snapshot, '` LIKE `addon_quant_theme_stock`'), 'DO 0');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

SET @tm_sql = IF(@tm_need_snapshot = 1,
  CONCAT('INSERT INTO `', @tm_snapshot, '` (', @tm_cols, ') SELECT ', @tm_cols, ' FROM `addon_quant_theme_stock`'),
  'DO 0');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- 3.2 关联键为空的行没有任何含义（也无法满足 NOT NULL），已在快照里；从主表清掉
DELETE FROM `addon_quant_theme_stock` WHERE `theme_id` IS NULL OR `stock_id` IS NULL;

-- 3.3 一条 ALTER 完成：恢复三个列的权威定义 + 删除旧 GVA 列（没有要改的就什么也不做）
SET @tm_alter = CONCAT_WS(', ',
  IF(@tm_keys_ok < 3,
     'MODIFY COLUMN `theme_id` bigint UNSIGNED NOT NULL COMMENT ''题材/环节ID（addon_quant_theme.id，可为任意层级）''', NULL),
  IF(@tm_keys_ok < 3,
     'MODIFY COLUMN `stock_id` bigint UNSIGNED NOT NULL COMMENT ''股票ID（addon_quant_base_stock.id，权威字段）''', NULL),
  IF(@tm_keys_ok < 3,
     'MODIFY COLUMN `status` tinyint DEFAULT 1 COMMENT ''状态：1启用 0停用''', NULL),
  @tm_drop_clauses);
SET @tm_sql = IF(@tm_alter = '', 'DO 0', CONCAT('ALTER TABLE `addon_quant_theme_stock` ', @tm_alter));
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 4. 终检：合规命门必须真的在，否则中止（宁可停下，不要静默带病）──────────────
-- 4.1 依据列与审核状态均为 NOT NULL
SET @tm_sql = IF((SELECT COUNT(*) FROM information_schema.columns
                   WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                     AND column_name IN ('ts_code', 'source_type', 'source_excerpt', 'source_url',
                                         'collected_at', 'audit_status')
                     AND is_nullable = 'NO') = 6,
  'DO 0', 'SELECT * FROM `tm_migration_abort__theme_stock_evidence_column_nullable`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- 4.2 关联键为 bigint unsigned NOT NULL，且没有旧 GVA 列残留
SET @tm_sql = IF((SELECT COUNT(*) FROM information_schema.columns
                   WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                     AND column_name IN ('theme_id', 'stock_id')
                     AND column_type = 'bigint unsigned' AND is_nullable = 'NO') = 2
                 AND (SELECT COUNT(*) FROM information_schema.columns
                       WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                         AND column_name IN ('reason', 'ai_reason', 'tier', 'relevance', 'in_date')) = 0,
  'DO 0', 'SELECT * FROM `tm_migration_abort__theme_stock_key_columns_or_legacy_columns`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- 4.3 依据 CHECK 约束存在且生效
SET @tm_sql = IF((SELECT COUNT(*) FROM information_schema.table_constraints
                   WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                     AND constraint_name = 'chk_theme_stock_evidence'
                     AND constraint_type = 'CHECK' AND enforced = 'YES') = 1,
  'DO 0', 'SELECT * FROM `tm_migration_abort__theme_stock_evidence_check_missing`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- 4.4 唯一键 (theme_id, stock_id, alive) 存在，生成列 alive 存在
SET @tm_sql = IF((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
                    FROM information_schema.statistics
                   WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                     AND index_name = 'uk_theme_stock_alive' AND non_unique = 0) = 'theme_id,stock_id,alive'
                 AND (SELECT COUNT(*) FROM information_schema.columns
                       WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme_stock'
                         AND column_name = 'alive' AND extra LIKE '%STORED GENERATED%') = 1,
  'DO 0', 'SELECT * FROM `tm_migration_abort__theme_stock_unique_key_or_alive_missing`');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

-- ── 5. 记录本次走了哪个分支（运维请写进变更记录）──────────────────────────────
SET @tm_state = CASE
  WHEN @tm_exists = 0 THEN 'created'
  WHEN @tm_ev_cols = 0 THEN 'legacy_archived'
  WHEN @tm_alter <> '' OR @tm_null_keys > 0 THEN 'repaired'
  ELSE 'already_converged'
END;

-- ── 6. 重算题材上的"可见股票数量"（addon_quant_theme.stock_count）──────────────
-- 旧表被封存或被修复之后，题材表里存的是旧口径的数量（GVA 曾把"没下架"的都计入）。现在的口径与 C 端列表一致：
-- 审核已通过、启用中、未删除。该列只在 GVA 的题材表里有，表或列不存在就跳过（例如只有最小复刻的测试库）。
-- 每次执行都重算，所以即使上一次执行在这一步之前中断，重跑也能自愈；数值没变的行不会被写入。
SET @tm_has_count = (SELECT COUNT(*) FROM information_schema.columns
                      WHERE table_schema = @tm_db AND table_name = 'addon_quant_theme' AND column_name = 'stock_count');
SET @tm_sql = IF(@tm_has_count = 1,
  CONCAT('UPDATE `addon_quant_theme` t LEFT JOIN (',
         'SELECT theme_id, COUNT(*) AS c FROM `addon_quant_theme_stock` ',
         'WHERE deleted_at IS NULL AND audit_status = 2 AND status = 1 GROUP BY theme_id) x ',
         'ON x.theme_id = t.id SET t.stock_count = IFNULL(x.c, 0)'),
  'DO 0');
PREPARE tm_stmt FROM @tm_sql; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;

SELECT 'converge_addon_quant_theme_stock' AS migration, @tm_state AS state,
       IF(@tm_exists = 1 AND @tm_ev_cols = 0, CONCAT('tm_theme_stock_legacy_', @tm_ts), NULL) AS legacy_archive_table,
       IF(@tm_need_snapshot = 1, @tm_snapshot, NULL) AS snapshot_table;
