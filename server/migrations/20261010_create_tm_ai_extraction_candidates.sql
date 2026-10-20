-- L0 内部抽取候选待审队列。候选不可直接展示，也不写入 addon_quant_theme_stock。
-- 本迁移按文件名字典序位于 20261010_create_ai_source_snapshots.sql 之后。

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `tm_ai_extraction_candidates` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `snapshot_id` bigint UNSIGNED NOT NULL,
  `claim_index` smallint unsigned NOT NULL COMMENT '本次模型输出中的零基索引，仅作追溯',
  `theme_id` bigint UNSIGNED NOT NULL COMMENT '从题材主数据精确解析，允许任意层级节点',
  `theme_key` varchar(64) NOT NULL COMMENT '模型选择的规范题材键，供审计解析过程',
  `stock_id` bigint UNSIGNED NOT NULL COMMENT '从股票主数据精确解析',
  `entity_key` varchar(96) NOT NULL COMMENT '从原文提取并与股票主数据精确匹配的名称或代码',
  `ts_code` varchar(20) NOT NULL COMMENT '与 stock_id 对账，目标表冗余字段',
  `predicate` varchar(32) NOT NULL COMMENT '封闭事实类型，不是投资评价',
  `source_type` tinyint NOT NULL COMMENT '映射到 addon_quant_theme_stock.source_type',
  `source_excerpt` varchar(1000) NOT NULL COMMENT '从快照句子编号回填的逐字摘录',
  `sentence_ids` json NOT NULL COMMENT '模型返回的快照句子编号',
  `quote_segments` json NOT NULL COMMENT '代码从不可变快照回填并通过筛查的逐字分段',
  `excerpt_trimmed` tinyint NOT NULL DEFAULT 0 COMMENT '含省略或红线筛查截取时须标注节选',
  `source_url` varchar(512) NOT NULL,
  `collected_at` datetime(3) NOT NULL,
  `evidence_hash` binary(32) NOT NULL COMMENT '句子编号与回填分段的 SHA-256，用于幂等去重',
  `provider` varchar(64) NOT NULL,
  `model` varchar(128) NOT NULL,
  `prompt_version` varchar(64) NOT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_candidate_evidence` (`snapshot_id`, `theme_id`, `stock_id`, `evidence_hash`),
  KEY `idx_tm_ai_candidate_created` (`created_at`, `id`),
  KEY `idx_tm_ai_candidate_snapshot` (`snapshot_id`, `id`),
  CONSTRAINT `fk_tm_ai_candidate_snapshot` FOREIGN KEY (`snapshot_id`)
    REFERENCES `tm_ai_source_snapshots` (`id`) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_tm_ai_candidate_values` CHECK (
    `claim_index` < 50 AND `theme_id` > 0 AND `theme_key` <> '' AND `stock_id` > 0 AND `entity_key` <> '' AND
    `predicate` IN ('DISCLOSES_BUSINESS', 'LISTED_IN_CATALOG') AND
    `source_type` BETWEEN 1 AND 5 AND `source_excerpt` <> '' AND `source_url` <> '' AND
    `excerpt_trimmed` IN (0, 1)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='L0 AI 事实抽取候选；仅供内部 100% 人工审核';

-- 候选依据完全不可变；待审/通过/驳回状态由独立唯一审计事件推导。
DROP TRIGGER IF EXISTS `tm_ai_candidates_no_update`;
CREATE TRIGGER `tm_ai_candidates_no_update` BEFORE UPDATE ON `tm_ai_extraction_candidates`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI candidates are immutable';

DROP TRIGGER IF EXISTS `tm_ai_candidates_no_delete`;
CREATE TRIGGER `tm_ai_candidates_no_delete` BEFORE DELETE ON `tm_ai_extraction_candidates`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI candidates are immutable';

CREATE TABLE IF NOT EXISTS `tm_ai_review_audits` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `request_id` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `candidate_id` bigint UNSIGNED NOT NULL,
  `evidence_digest` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actor_id` bigint UNSIGNED NOT NULL,
  `decision` varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `reason` varchar(250) NOT NULL DEFAULT '',
  `mapping_id` bigint UNSIGNED DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_review_request` (`request_id`),
  UNIQUE KEY `uk_tm_ai_review_candidate` (`candidate_id`),
  KEY `idx_tm_ai_review_actor_time` (`actor_id`, `created_at`),
  KEY `idx_tm_ai_review_decision_id` (`decision`, `id`),
  CONSTRAINT `fk_tm_ai_review_candidate` FOREIGN KEY (`candidate_id`)
    REFERENCES `tm_ai_extraction_candidates` (`id`) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_tm_ai_review_decision` CHECK (
    `actor_id` > 0 AND
    ((`decision` = 'approve' AND `mapping_id` IS NOT NULL AND `mapping_id` > 0 AND `reason` = '') OR
     (`decision` = 'reject' AND `mapping_id` IS NULL AND `reason` <> ''))
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='L0 人工审核不可变审计事件；与候选终态和归属写入同事务';

DROP TRIGGER IF EXISTS `tm_ai_review_audits_no_update`;
CREATE TRIGGER `tm_ai_review_audits_no_update` BEFORE UPDATE ON `tm_ai_review_audits`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI review audits are immutable';

DROP TRIGGER IF EXISTS `tm_ai_review_audits_no_delete`;
CREATE TRIGGER `tm_ai_review_audits_no_delete` BEFORE DELETE ON `tm_ai_review_audits`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI review audits are immutable';
