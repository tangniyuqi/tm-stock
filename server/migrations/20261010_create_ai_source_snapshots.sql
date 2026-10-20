-- AI L0 资料来源登记与不可变快照。
-- 生产来源必须有可追溯的书面授权记录；新来源默认关闭。

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `tm_ai_sources` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `domain` varchar(253) NOT NULL COMMENT '精确允许的 HTTPS 主机名，不自动包含子域名',
  `category` varchar(32) NOT NULL COMMENT 'announcement/annual_report/prospectus/official_catalog/interactive_qa',
  `license_tag` varchar(128) NOT NULL DEFAULT '' COMMENT '授权范围标签；空值不得启用',
  `license_ref` varchar(512) DEFAULT NULL COMMENT '书面授权归档编号或不可变文件引用',
  `license_expires_at` datetime(3) DEFAULT NULL COMMENT '授权到期时间；NULL 表示授权文件未约定到期日',
  `min_interval_ms` int unsigned NOT NULL DEFAULT 1000 COMMENT '同一来源两次请求的最小间隔',
  `enabled` tinyint NOT NULL DEFAULT 0 COMMENT '默认关闭；仅授权有效的来源可启用',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_sources_domain` (`domain`),
  KEY `idx_tm_ai_sources_enabled_license` (`enabled`, `license_expires_at`),
  CONSTRAINT `chk_tm_ai_sources_enabled_authorized` CHECK (
    `enabled` IN (0, 1) AND
    `domain` <> '' AND `min_interval_ms` > 0 AND
    `category` IN ('announcement', 'annual_report', 'prospectus', 'official_catalog', 'interactive_qa') AND
    (`enabled` = 0 OR (TRIM(`license_tag`) <> '' AND `license_ref` IS NOT NULL AND TRIM(`license_ref`) <> ''))
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='AI 资料来源白名单与授权登记';

-- 正文按内容哈希去重存放；body_ref 指向本表的 content_hash。
-- 将正文与来源元数据分表，保留 design.md 的 body_ref 契约，避免快照元数据行重复承载大正文。
CREATE TABLE IF NOT EXISTS `tm_ai_source_bodies` (
  `content_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `body_text` mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `sentences_json` json NOT NULL COMMENT '规范化正文的句子编号与 UTF-8 字节偏移',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`content_hash`),
  CONSTRAINT `chk_tm_ai_source_body_text` CHECK (`body_text` <> '')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='按内容哈希寻址的 AI 来源正文';

CREATE TABLE IF NOT EXISTS `tm_ai_source_snapshots` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `source_id` bigint UNSIGNED NOT NULL,
  `category_snapshot` varchar(32) NOT NULL COMMENT '固化时的来源类别，避免来源登记变更改写历史口径',
  `url` varchar(512) NOT NULL COMMENT '固化时的 HTTPS 原文地址；与归属依据表的字段上限一致',
  `url_hash` binary(32) NOT NULL COMMENT '规范化 URL 的 SHA-256',
  `content_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '规范化 UTF-8 正文的 SHA-256',
  `body_ref` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'tm_ai_source_bodies.content_hash',
  `license_tag_snapshot` varchar(128) NOT NULL,
  `license_ref_snapshot` varchar(512) NOT NULL,
  `license_expires_snapshot` datetime(3) DEFAULT NULL,
  `fetched_at` datetime(3) NOT NULL COMMENT '采集或导入固化时点',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tm_ai_snapshot_source_url_content` (`source_id`, `url_hash`, `content_hash`),
  KEY `idx_tm_ai_snapshot_source_time` (`source_id`, `fetched_at`),
  KEY `idx_tm_ai_snapshot_body_ref` (`body_ref`),
  CONSTRAINT `fk_tm_ai_snapshot_source` FOREIGN KEY (`source_id`)
    REFERENCES `tm_ai_sources` (`id`) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_tm_ai_snapshot_body` FOREIGN KEY (`body_ref`)
    REFERENCES `tm_ai_source_bodies` (`content_hash`) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_tm_ai_snapshot_evidence` CHECK (
    `category_snapshot` IN ('announcement', 'annual_report', 'prospectus', 'official_catalog', 'interactive_qa') AND
    `url` <> '' AND `content_hash` <> '' AND `body_ref` = `content_hash` AND
    `license_tag_snapshot` <> '' AND `license_ref_snapshot` <> ''
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='AI 资料来源不可变快照与可还原句子偏移';

-- 应用层仓储只提供 INSERT 与 SELECT；数据库再阻止快照被覆盖或删除。
DROP TRIGGER IF EXISTS `tm_ai_source_snapshots_no_update`;
CREATE TRIGGER `tm_ai_source_snapshots_no_update`
BEFORE UPDATE ON `tm_ai_source_snapshots`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI资料快照不可修改';

DROP TRIGGER IF EXISTS `tm_ai_source_snapshots_no_delete`;
CREATE TRIGGER `tm_ai_source_snapshots_no_delete`
BEFORE DELETE ON `tm_ai_source_snapshots`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI资料快照不可删除';

DROP TRIGGER IF EXISTS `tm_ai_source_bodies_no_update`;
CREATE TRIGGER `tm_ai_source_bodies_no_update`
BEFORE UPDATE ON `tm_ai_source_bodies`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI资料正文不可修改';

DROP TRIGGER IF EXISTS `tm_ai_source_bodies_no_delete`;
CREATE TRIGGER `tm_ai_source_bodies_no_delete`
BEFORE DELETE ON `tm_ai_source_bodies`
FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'AI资料正文不可删除';
