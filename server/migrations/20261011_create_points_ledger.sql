-- 专用积分账本底座；不读取/迁移 GVA 旧余额，不启用充值、收费任务或支付入口。
-- 每笔入账从零写正分录；冻结是可用到冻结的转移，结算扣冻结，释放反向转移。
-- 账户、批次、冻结与欠款是快照；流水、分录、分摊只增不改，更正另写冲正。
-- DDL 非事务；执行/重跑前须停止账务写入。可重复运行，但不会升级未知旧表。
-- 启用账务前必须运行真实约束探针；触发器重建失败时不得恢复写入。
-- datetime 字段统一按 UTC 写入；服务必须显式赋 UTC 值，不能依赖会话时区的默认时间。
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS tm_points_accounts (
 user_id bigint UNSIGNED NOT NULL,
 avail bigint NOT NULL DEFAULT 0,
 frozen bigint NOT NULL DEFAULT 0,
 debt bigint NOT NULL DEFAULT 0,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(user_id),
 CONSTRAINT chk_tm_points_account CHECK (user_id > 0 AND avail >= 0 AND frozen >= 0 AND debt >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='积分账户快照；不与广告、会员兑换过账';

CREATE TABLE IF NOT EXISTS tm_points_txns (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 user_id bigint UNSIGNED NOT NULL,
 idem_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 request_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 kind varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 biz_type varchar(30) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 biz_id varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 reverses_id bigint UNSIGNED NULL,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(id), UNIQUE KEY uk_tm_points_txn_owner(id,user_id),
 UNIQUE KEY uk_tm_points_txn_idem(idem_key),
 UNIQUE KEY uk_tm_points_txn_business(kind,biz_type,biz_id),
 UNIQUE KEY uk_tm_points_txn_reverses(reverses_id),
 KEY idx_tm_points_txn_history(user_id,id),
 CONSTRAINT fk_tm_points_txn_account FOREIGN KEY(user_id) REFERENCES tm_points_accounts(user_id),
 CONSTRAINT fk_tm_points_txn_reversal FOREIGN KEY(reverses_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT chk_tm_points_txn CHECK (
  CHAR_LENGTH(idem_key) BETWEEN 16 AND 80 AND idem_key REGEXP '^[A-Za-z0-9_-]+$'
  AND request_hash REGEXP '^[0-9a-f]{64}$' AND CHAR_LENGTH(biz_id) BETWEEN 1 AND 64 AND biz_id REGEXP '^[A-Za-z0-9_.:-]+$'
  AND ((kind='grant' AND biz_type='points_grant')
    OR (kind IN ('freeze','settle','release') AND biz_type IN ('ai_task','content_unlock'))
    OR (kind='expire' AND biz_type='lot_expiry')
    OR (kind='reversal' AND biz_type='points_refund')
    OR (kind='debt_offset' AND biz_type='debt_offset'))
  AND ((kind='reversal' AND reverses_id IS NOT NULL) OR (kind<>'reversal' AND reverses_id IS NULL)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变积分流水头；完整请求摘要绑定幂等键';

CREATE TABLE IF NOT EXISTS tm_points_lots (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 user_id bigint UNSIGNED NOT NULL,
 grant_txn_id bigint UNSIGNED NOT NULL,
 src tinyint NOT NULL COMMENT '1购买 2赠送 3贡献奖励 4签到',
 cash_backed tinyint NOT NULL,
 total bigint NOT NULL,
 remain bigint NOT NULL,
 frozen bigint NOT NULL DEFAULT 0,
 prio smallint NOT NULL DEFAULT 100,
 eff_at datetime(3) NOT NULL,
 exp_at datetime(3) NULL,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(id), UNIQUE KEY uk_tm_points_lot_owner(id,user_id),
 UNIQUE KEY uk_tm_points_lot_grant(grant_txn_id),
 KEY idx_tm_points_lot_use(user_id,prio,exp_at,id),
 KEY idx_tm_points_lot_expiry(exp_at,id),
 CONSTRAINT fk_tm_points_lot_account FOREIGN KEY(user_id) REFERENCES tm_points_accounts(user_id),
 CONSTRAINT fk_tm_points_lot_grant FOREIGN KEY(grant_txn_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT chk_tm_points_lot CHECK (total > 0 AND remain >= 0 AND frozen >= 0 AND frozen <= remain AND remain <= total
  AND src IN (1,2,3,4) AND cash_backed IN (0,1) AND ((src=1 AND cash_backed=1) OR (src<>1 AND cash_backed=0))
  AND (exp_at IS NULL OR exp_at > eff_at))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='积分批次快照；来源和购买属性不可改';

CREATE TABLE IF NOT EXISTS tm_points_entries (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 user_id bigint UNSIGNED NOT NULL,
 txn_id bigint UNSIGNED NOT NULL,
 lot_id bigint UNSIGNED NOT NULL,
 bucket tinyint NOT NULL COMMENT '1可用 2冻结',
 delta bigint NOT NULL,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(id), UNIQUE KEY uk_tm_points_entry(txn_id,lot_id,bucket),
 KEY idx_tm_points_entry_lot(lot_id,bucket,id),
 KEY idx_tm_points_entry_user(user_id,id),
 CONSTRAINT fk_tm_points_entry_txn FOREIGN KEY(txn_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT fk_tm_points_entry_lot FOREIGN KEY(lot_id,user_id) REFERENCES tm_points_lots(id,user_id),
 CONSTRAINT chk_tm_points_entry CHECK (bucket IN (1,2) AND delta <> 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变积分分录；含初始入账，从零累计';

CREATE TABLE IF NOT EXISTS tm_points_holds (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 user_id bigint UNSIGNED NOT NULL,
 biz_type varchar(30) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 biz_id varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 freeze_txn_id bigint UNSIGNED NOT NULL,
 terminal_txn_id bigint UNSIGNED NULL,
 amount bigint NOT NULL,
 captured bigint NOT NULL DEFAULT 0,
 price_ver varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 state tinyint NOT NULL DEFAULT 1 COMMENT '1冻结中 2已结算 3已释放 4已过期',
 expire_at datetime(3) NOT NULL,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(id), UNIQUE KEY uk_tm_points_hold_owner(id,user_id),
 UNIQUE KEY uk_tm_points_hold_biz(biz_type,biz_id),
 UNIQUE KEY uk_tm_points_hold_freeze(freeze_txn_id),
 UNIQUE KEY uk_tm_points_hold_terminal(terminal_txn_id),
 KEY idx_tm_points_hold_expiry(state,expire_at,id),
 KEY idx_tm_points_hold_user(user_id,state,id),
 CONSTRAINT fk_tm_points_hold_account FOREIGN KEY(user_id) REFERENCES tm_points_accounts(user_id),
 CONSTRAINT fk_tm_points_hold_freeze FOREIGN KEY(freeze_txn_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT fk_tm_points_hold_terminal FOREIGN KEY(terminal_txn_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT chk_tm_points_hold CHECK (biz_type IN ('ai_task','content_unlock') AND CHAR_LENGTH(biz_id) BETWEEN 1 AND 64 AND biz_id REGEXP '^[A-Za-z0-9_.:-]+$'
  AND amount > 0 AND CHAR_LENGTH(price_ver) BETWEEN 1 AND 32 AND price_ver REGEXP '^[A-Za-z0-9_.:-]+$' AND expire_at > created_at
  AND ((state=1 AND captured=0 AND terminal_txn_id IS NULL)
    OR (state=2 AND captured=amount AND terminal_txn_id IS NOT NULL)
    OR (state IN (3,4) AND captured=0 AND terminal_txn_id IS NOT NULL)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='冻结快照；终态全额结算或全额释放';

CREATE TABLE IF NOT EXISTS tm_points_hold_allocs (
 hold_id bigint UNSIGNED NOT NULL,
 lot_id bigint UNSIGNED NOT NULL,
 user_id bigint UNSIGNED NOT NULL,
 amount bigint NOT NULL,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(hold_id,lot_id), KEY idx_tm_points_hold_alloc_lot(lot_id,hold_id),
 KEY idx_tm_points_hold_alloc_user(user_id,hold_id,lot_id),
 CONSTRAINT fk_tm_points_alloc_hold FOREIGN KEY(hold_id,user_id) REFERENCES tm_points_holds(id,user_id),
 CONSTRAINT fk_tm_points_alloc_lot FOREIGN KEY(lot_id,user_id) REFERENCES tm_points_lots(id,user_id),
 CONSTRAINT chk_tm_points_alloc CHECK (amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变冻结分摊；不得跨用户';

CREATE TABLE IF NOT EXISTS tm_points_debts (
 id bigint UNSIGNED NOT NULL AUTO_INCREMENT,
 user_id bigint UNSIGNED NOT NULL,
 reversal_txn_id bigint UNSIGNED NOT NULL,
 amount bigint NOT NULL,
 outstanding bigint NOT NULL,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(id), UNIQUE KEY uk_tm_points_debt_owner(id,user_id),
 UNIQUE KEY uk_tm_points_debt_reversal(reversal_txn_id),
 KEY idx_tm_points_debt_user(user_id,id),
 CONSTRAINT fk_tm_points_debt_account FOREIGN KEY(user_id) REFERENCES tm_points_accounts(user_id),
 CONSTRAINT fk_tm_points_debt_reversal FOREIGN KEY(reversal_txn_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT chk_tm_points_debt CHECK (amount > 0 AND outstanding >= 0 AND outstanding <= amount)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='退款冲正欠款快照；不允许余额为负';

CREATE TABLE IF NOT EXISTS tm_points_debt_allocs (
 txn_id bigint UNSIGNED NOT NULL,
 debt_id bigint UNSIGNED NOT NULL,
 lot_id bigint UNSIGNED NOT NULL,
 user_id bigint UNSIGNED NOT NULL,
 amount bigint NOT NULL,
 created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 PRIMARY KEY(txn_id,debt_id,lot_id), KEY idx_tm_points_debt_alloc_debt(debt_id,txn_id),
 KEY idx_tm_points_debt_alloc_user(user_id,txn_id,debt_id,lot_id),
 CONSTRAINT fk_tm_points_debt_alloc_txn FOREIGN KEY(txn_id,user_id) REFERENCES tm_points_txns(id,user_id),
 CONSTRAINT fk_tm_points_debt_alloc_debt FOREIGN KEY(debt_id,user_id) REFERENCES tm_points_debts(id,user_id),
 CONSTRAINT fk_tm_points_debt_alloc_lot FOREIGN KEY(lot_id,user_id) REFERENCES tm_points_lots(id,user_id),
 CONSTRAINT chk_tm_points_debt_alloc CHECK (amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='不可变欠款抵扣分摊；可追溯至奖励批次与流水';

-- 不可变事件和分摊用单语句触发器；下方复合触发器使用 mysql 客户端分隔符指令。
DROP TRIGGER IF EXISTS tm_points_txns_no_update;
CREATE TRIGGER tm_points_txns_no_update BEFORE UPDATE ON tm_points_txns FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points transactions are immutable';
DROP TRIGGER IF EXISTS tm_points_txns_no_delete;
CREATE TRIGGER tm_points_txns_no_delete BEFORE DELETE ON tm_points_txns FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points transactions are immutable';
DROP TRIGGER IF EXISTS tm_points_entries_no_update;
CREATE TRIGGER tm_points_entries_no_update BEFORE UPDATE ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points entries are immutable';
DROP TRIGGER IF EXISTS tm_points_entries_no_delete;
CREATE TRIGGER tm_points_entries_no_delete BEFORE DELETE ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points entries are immutable';
DROP TRIGGER IF EXISTS tm_points_hold_allocs_no_update;
CREATE TRIGGER tm_points_hold_allocs_no_update BEFORE UPDATE ON tm_points_hold_allocs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points allocations are immutable';
DROP TRIGGER IF EXISTS tm_points_hold_allocs_no_delete;
CREATE TRIGGER tm_points_hold_allocs_no_delete BEFORE DELETE ON tm_points_hold_allocs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points allocations are immutable';
DROP TRIGGER IF EXISTS tm_points_debt_allocs_no_update;
CREATE TRIGGER tm_points_debt_allocs_no_update BEFORE UPDATE ON tm_points_debt_allocs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points debt allocations are immutable';
DROP TRIGGER IF EXISTS tm_points_debt_allocs_no_delete;
CREATE TRIGGER tm_points_debt_allocs_no_delete BEFORE DELETE ON tm_points_debt_allocs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points debt allocations are immutable';

-- 快照只能改余额/状态；身份、来源、锁价和原始数量不能改，也不能删除。
DROP TRIGGER IF EXISTS tm_points_accounts_identity;
DELIMITER $$
CREATE TRIGGER tm_points_accounts_identity BEFORE UPDATE ON tm_points_accounts FOR EACH ROW
BEGIN
 IF NEW.user_id <> OLD.user_id OR NEW.created_at <> OLD.created_at THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points account identity is immutable'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_points_accounts_no_delete;
CREATE TRIGGER tm_points_accounts_no_delete BEFORE DELETE ON tm_points_accounts FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points accounts cannot be deleted';
DROP TRIGGER IF EXISTS tm_points_lots_identity;
DELIMITER $$
CREATE TRIGGER tm_points_lots_identity BEFORE UPDATE ON tm_points_lots FOR EACH ROW
BEGIN
 IF NOT (NEW.id=OLD.id AND NEW.user_id=OLD.user_id AND NEW.grant_txn_id=OLD.grant_txn_id AND NEW.src=OLD.src AND NEW.cash_backed=OLD.cash_backed AND NEW.total=OLD.total AND NEW.prio=OLD.prio AND NEW.eff_at=OLD.eff_at AND (NEW.exp_at <=> OLD.exp_at) AND NEW.created_at=OLD.created_at)
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points lot origin is immutable'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_points_lots_no_delete;
CREATE TRIGGER tm_points_lots_no_delete BEFORE DELETE ON tm_points_lots FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points lots cannot be deleted';
DROP TRIGGER IF EXISTS tm_points_holds_identity;
DELIMITER $$
CREATE TRIGGER tm_points_holds_identity BEFORE UPDATE ON tm_points_holds FOR EACH ROW
BEGIN
 IF NOT (NEW.id=OLD.id AND NEW.user_id=OLD.user_id AND NEW.biz_type=OLD.biz_type AND NEW.biz_id=OLD.biz_id AND NEW.freeze_txn_id=OLD.freeze_txn_id AND NEW.amount=OLD.amount AND NEW.price_ver=OLD.price_ver AND NEW.expire_at=OLD.expire_at AND NEW.created_at=OLD.created_at)
 OR OLD.state<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points hold identity or terminal state is immutable'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_points_holds_no_delete;
CREATE TRIGGER tm_points_holds_no_delete BEFORE DELETE ON tm_points_holds FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points holds cannot be deleted';
DROP TRIGGER IF EXISTS tm_points_debts_identity;
DELIMITER $$
CREATE TRIGGER tm_points_debts_identity BEFORE UPDATE ON tm_points_debts FOR EACH ROW
BEGIN
 IF NOT (NEW.id=OLD.id AND NEW.user_id=OLD.user_id AND NEW.reversal_txn_id=OLD.reversal_txn_id AND NEW.amount=OLD.amount AND NEW.created_at=OLD.created_at)
 OR NEW.outstanding>OLD.outstanding THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points debt origin is immutable'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_points_debts_no_delete;
CREATE TRIGGER tm_points_debts_no_delete BEFORE DELETE ON tm_points_debts FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Points debts cannot be deleted';
