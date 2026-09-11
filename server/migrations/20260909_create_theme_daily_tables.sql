-- 题材动态与收盘快照；快照按日落库，禁止查询时用今日行情重算历史。
CREATE TABLE IF NOT EXISTS theme_daily_item (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  biz_date DATE NOT NULL,
  theme_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(255) NOT NULL,
  source VARCHAR(64) NOT NULL,
  source_url VARCHAR(512) NOT NULL DEFAULT '',
  audit_status TINYINT NOT NULL DEFAULT 0 COMMENT '2=已审核通过',
  publish_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id), KEY idx_date (biz_date, audit_status, publish_at, id), KEY idx_theme (theme_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS theme_daily_quote (
  biz_date DATE NOT NULL,
  theme_id BIGINT UNSIGNED NOT NULL,
  change_pct DECIMAL(8,4) NULL,
  caliber VARCHAR(128) NOT NULL,
  is_mock TINYINT NOT NULL DEFAULT 0,
  snapshot_at DATETIME(3) NOT NULL,
  PRIMARY KEY (biz_date, theme_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS trading_calendar (
  biz_date DATE NOT NULL,
  is_trading_day TINYINT NOT NULL,
  remark VARCHAR(64) NOT NULL DEFAULT '',
  PRIMARY KEY (biz_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
