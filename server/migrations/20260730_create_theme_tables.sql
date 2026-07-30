-- =============================================================================
-- 题材查询：分类 / 题材 / 产业链环节 / 个股 / 归属映射 / 事件
-- 依据：docs/specs/theme-query/design.md §4、docs/adr/0003-个股客观事实层可做.md
--
-- 🔴 本文件承载本项目的【合规命门】：
--    theme_stock_mapping 的四项证据字段是「排除品种选择认定」的核心抓手。
--    NOT NULL + CHECK <> '' 是三重防线里的最后一层——
--    它兜住批量导入与手工 SQL 这两条绕过表单与接口校验的路径。
-- =============================================================================

-- ── 分类（一/二/三级）──
CREATE TABLE IF NOT EXISTS theme_category (
  id         BIGINT      PRIMARY KEY AUTO_INCREMENT,
  name       VARCHAR(64) NOT NULL,
  level      TINYINT     NOT NULL COMMENT '1|2|3',
  parent_id  BIGINT      NOT NULL DEFAULT 0,
  sort_order INT         NOT NULL DEFAULT 0,
  created_at DATETIME    NOT NULL,
  updated_at DATETIME    NOT NULL,
  KEY idx_parent (parent_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='题材分类';

-- ── 题材 ──
CREATE TABLE IF NOT EXISTS theme (
  id          BIGINT       PRIMARY KEY AUTO_INCREMENT,
  name        VARCHAR(128) NOT NULL,
  category_id BIGINT       NOT NULL COMMENT '三级分类 id',
  summary     TEXT         NOT NULL COMMENT '一句话通俗解读（客观，须过禁用词校验）',
  data_source VARCHAR(255) NOT NULL DEFAULT '' COMMENT '数据来源说明',
  created_at  DATETIME     NOT NULL,
  updated_at  DATETIME     NOT NULL,
  UNIQUE KEY uk_name (name),
  KEY idx_category (category_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='题材';

-- ── 产业链环节 ──
CREATE TABLE IF NOT EXISTS theme_chain_node (
  id          BIGINT       PRIMARY KEY AUTO_INCREMENT,
  theme_id    BIGINT       NOT NULL,
  name        VARCHAR(64)  NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '' COMMENT '环节说明（须过禁用词校验）',
  sort_order  INT          NOT NULL DEFAULT 0,
  created_at  DATETIME     NOT NULL,
  updated_at  DATETIME     NOT NULL,
  KEY idx_theme (theme_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='产业链环节';

-- ── 个股基础信息 ──
CREATE TABLE IF NOT EXISTS stock (
  code       VARCHAR(16) PRIMARY KEY COMMENT '如 688502',
  name       VARCHAR(64) NOT NULL,
  market     VARCHAR(8)  NOT NULL COMMENT 'SH|SZ|BJ',
  delisted   TINYINT     NOT NULL DEFAULT 0,
  created_at DATETIME    NOT NULL,
  updated_at DATETIME    NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='个股标识。🔴 禁止在此表扩展基本面/财务字段——红线而非排期（ADR-0003）';

-- ── 🧭 题材 ↔ 个股归属映射（合规命门）──
CREATE TABLE IF NOT EXISTS theme_stock_mapping (
  id            BIGINT       PRIMARY KEY AUTO_INCREMENT,
  theme_id      BIGINT       NOT NULL,
  chain_node_id BIGINT       NOT NULL,
  stock_code    VARCHAR(16)  NOT NULL,

  -- 四项证据字段：全部 NOT NULL，且 CHECK 禁止空串
  source_type    VARCHAR(32)   NOT NULL COMMENT '公告|年报|招股书|官方目录',
  source_excerpt VARCHAR(1024) NOT NULL COMMENT '原文摘录（不允许"见链接"之类占位）',
  source_url     VARCHAR(512)  NOT NULL COMMENT '原文链接',
  collected_at   DATETIME      NOT NULL COMMENT '采集时点',

  -- 审核留痕。status 从建表就存在，不走后补 ALTER：
  -- 过滤条件（status='APPROVED'）必须与表同龄，事后给已有数据补过滤才是泄漏的典型来源。
  -- 默认 DRAFT 是 fail-safe——误插入的行不会直接对外可见。
  status        VARCHAR(16) NOT NULL DEFAULT 'DRAFT'
                COMMENT 'DRAFT|PENDING|APPROVED|REJECTED；仅 APPROVED 对 C 端可见',
  creator       VARCHAR(64) NOT NULL DEFAULT '',
  reviewer      VARCHAR(64) NOT NULL DEFAULT '',
  reviewed_at   DATETIME    NULL,
  reject_reason VARCHAR(255) NOT NULL DEFAULT '',

  sort_order INT      NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,

  UNIQUE KEY uk_node_stock (chain_node_id, stock_code),
  KEY idx_theme (theme_id),
  KEY idx_status (status),

  CONSTRAINT chk_evidence_not_blank CHECK (
    source_type <> '' AND source_excerpt <> '' AND source_url <> ''
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='🔴 无依据禁止入库。依据是排除「品种选择」认定的核心抓手（ADR-0003）';

-- ── 题材关联事件 ──
CREATE TABLE IF NOT EXISTS theme_event (
  id         BIGINT       PRIMARY KEY AUTO_INCREMENT,
  theme_id   BIGINT       NOT NULL,
  title      VARCHAR(255) NOT NULL COMMENT '客观标题（须过禁用词校验）',
  source     VARCHAR(64)  NOT NULL,
  source_url VARCHAR(512) NOT NULL DEFAULT '',
  publish_at DATETIME     NOT NULL,
  created_at DATETIME     NOT NULL,
  updated_at DATETIME     NOT NULL,
  KEY idx_theme_time (theme_id, publish_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='题材关联事件';
