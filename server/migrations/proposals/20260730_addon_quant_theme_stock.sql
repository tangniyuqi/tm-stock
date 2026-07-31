-- =============================================================================
-- 题材 ↔ 股票 关联表（多对多）
--
-- 配套现有表：addon_quant_theme（题材，层级树）、addon_quant_base_stock（股票，5497 条）
-- 命名、字段风格、软删除与操作人字段均沿用这两张表的约定。
--
-- ⚠️ 状态：提案。tm-stock 是否与 addon_quant_* 共库尚未拍板
--    （tm-stock 自己的 server/migrations/20260730_create_theme_tables.sql
--      建了功能重复的 theme / stock / theme_stock_mapping，二者必须二选一）。
--    定了之后本文件移到 server/migrations/ 正式目录。
--
-- 🔴 本表承载 ADR-0003 的【合规命门】：
--    "凭什么把这家公司归到这个题材" 必须有可溯源的客观依据。
--    依据缺失时，归属就从【事实归集】退化成【品种选择】，落入荐股认定要件二。
--    因此四项证据字段 NOT NULL + CHECK 非空，无依据禁止入库。
-- =============================================================================

SET NAMES utf8mb4;

CREATE TABLE `addon_quant_theme_stock` (
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


-- =============================================================================
-- 附：C 端查询必须带的三重过滤（写进 repository 基础查询，不要各处手写）
-- =============================================================================
-- SELECT ts.stock_id, s.ts_code, s.name
--   FROM addon_quant_theme_stock ts
--   JOIN addon_quant_base_stock  s ON s.id = ts.stock_id AND s.deleted_at IS NULL
--   JOIN addon_quant_theme       t ON t.id = ts.theme_id AND t.deleted_at IS NULL
--  WHERE ts.theme_id = ?
--    AND ts.deleted_at   IS NULL     -- ① 软删除
--    AND ts.audit_status = 2         -- ② 仅已审核通过
--    AND ts.status       = 1         -- ③ 启用中
--  ORDER BY ts.sort, s.ts_code;      -- 排序只用客观字段
--
-- 三个条件漏任意一个都是【静默泄漏】：不会报错，只会把不该露的数据露出去。


-- =============================================================================
-- 建议：补两个现有表缺失的唯一键（⚠️ 执行前必须先查重）
-- =============================================================================

-- ① addon_quant_base_stock 没有 ts_code 唯一键 → 重复导入会产生重复股票，
--    关联表将无法确定该指向哪一条。
-- 先查重：
--   SELECT ts_code, COUNT(*) c FROM addon_quant_base_stock
--    WHERE deleted_at IS NULL GROUP BY ts_code HAVING c > 1;
-- 无重复后再执行：
-- ALTER TABLE `addon_quant_base_stock`
--   ADD UNIQUE KEY `uk_ts_code_alive` (`ts_code`);

-- ② addon_quant_theme 的 uk_source_code(source, code) 在 code 为 NULL 时形同虚设
--    （MySQL 唯一键允许多个 NULL），现有两条数据 code 均为 NULL → 题材可重名。
-- 先查重：
--   SELECT parent_id, name, COUNT(*) c FROM addon_quant_theme
--    WHERE deleted_at IS NULL GROUP BY parent_id, name HAVING c > 1;
-- 无重复后再执行（同父节点下题材名唯一）：
-- ALTER TABLE `addon_quant_theme`
--   ADD UNIQUE KEY `uk_parent_name` (`parent_id`, `name`);


-- =============================================================================
-- 三个坑（前两个会【静默】出错，第三个直接建表失败）
-- =============================================================================
-- ① UNIQUE KEY (theme_id, stock_id, deleted_at) 是错的
--    很多人这样写以为兼顾了软删除，但 MySQL 唯一键【允许多个 NULL】，
--    而存活记录的 deleted_at 恰好都是 NULL → 约束对存活数据完全不生效，
--    同一股票能在同一题材下重复挂无数次，且不报错。
--    本表改用生成列 alive（存活=1，删除=NULL）解决：
--    存活行受约束，删除行因为是 NULL 而不受约束，正好符合语义。
--
-- ② 只写 NOT NULL 不足以保证依据非空
--    NOT NULL 拦不住空字符串。运营完全可以存一条 source_excerpt='' 的记录，
--    表面上"有依据字段"，实际什么都没有。必须配 CHECK <> ''。
--    且 CHECK 在 MySQL 5.7 会被静默忽略、8.0.16 前不强制——
--    上线前须在目标实例上实测（参考 scripts/dev/verify-migrations.sh 的做法）。
--
-- ③ 生成列不能用非确定性函数（实测踩过）
--    初稿写 IFNULL(UNIX_TIMESTAMP(deleted_at), 0)，建表直接失败：
--      ERROR 3763: Expression of generated column contains a disallowed
--                  function: unix_timestamp
--    UNIX_TIMESTAMP 依赖会话时区，属非确定性函数。IF / IFNULL 可以。
--
-- 本文件已在 MySQL 8.0.45 实测通过（建表 + 重复拒绝 + 软删后可重加 +
-- CHECK 拒空 + 一股多题材 + C端三重过滤）。
-- 注意：目标库版本是 MySQL 9.7.0，比 8.0 新，上述特性均支持；
-- 但正式执行前仍建议在目标实例上再跑一遍实测。
