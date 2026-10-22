-- 受限资料问答持久底座：不开放接口，不改积分、冻结、任务或报告表。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_qa_scopes (
 subject_type varchar(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,subject_id bigint UNSIGNED NOT NULL,
 subject_name varchar(128) NOT NULL,approval_ref varchar(128) NOT NULL,
 policy_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 evidence_json text NOT NULL,valid_until datetime(3) NOT NULL,
 enabled tinyint NOT NULL DEFAULT 0,version bigint NOT NULL DEFAULT 1,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(subject_type,subject_id),
 CONSTRAINT chk_tm_ai_qa_scope CHECK(subject_type IN('theme','topic') AND subject_id BETWEEN 1 AND 9007199254740991
 AND CHAR_LENGTH(subject_name) BETWEEN 1 AND 128 AND CHAR_LENGTH(approval_ref) BETWEEN 1 AND 128
 AND policy_digest REGEXP '^[a-f0-9]{64}$' AND JSON_VALID(evidence_json)
 AND JSON_TYPE(CAST(evidence_json AS JSON))='ARRAY' AND JSON_LENGTH(evidence_json)<=8
 AND enabled IN(0,1) AND version BETWEEN 1 AND 9007199254740991
 AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at AND valid_until>created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='默认关闭的题材专题问答范围批准';

CREATE TABLE IF NOT EXISTS tm_ai_qa_limits (
 id tinyint NOT NULL,approval_ref varchar(128) NOT NULL,enabled tinyint NOT NULL DEFAULT 0,
 user_requests bigint NOT NULL,instance_requests bigint NOT NULL,user_tokens bigint NOT NULL,instance_tokens bigint NOT NULL,
 version bigint NOT NULL DEFAULT 1,created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(id),
 CONSTRAINT chk_tm_ai_qa_limits CHECK(id=1 AND enabled IN(0,1) AND CHAR_LENGTH(approval_ref) BETWEEN 1 AND 128
 AND user_requests BETWEEN 1 AND 1000000000 AND instance_requests BETWEEN user_requests AND 1000000000
 AND user_tokens BETWEEN 1 AND 1000000000000 AND instance_tokens BETWEEN user_tokens AND 1000000000000
 AND version BETWEEN 1 AND 9007199254740991 AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='默认关闭的问答日安全配额政策，不是积分收费';

CREATE TABLE IF NOT EXISTS tm_ai_qa_quotas (
 quota_day date NOT NULL,user_id bigint UNSIGNED NOT NULL,requests bigint NOT NULL,tokens bigint NOT NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(quota_day,user_id),
 CONSTRAINT chk_tm_ai_qa_quota CHECK(user_id BETWEEN 0 AND 9007199254740991
 AND requests BETWEEN 0 AND 1000000000 AND tokens BETWEEN 0 AND 1000000000000
 AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='UTC日最坏预留，只增不减；会员0为实例合计';

CREATE TABLE IF NOT EXISTS tm_ai_qa_requests (
 conversation_id bigint UNSIGNED NOT NULL,seq bigint NOT NULL,user_id bigint UNSIGNED NOT NULL,
 input_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 message_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,proof_json text NOT NULL,
 state varchar(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,reason varchar(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 citations_json text NOT NULL,calls int NOT NULL,reserved_tokens int NOT NULL,quota_day date NULL,version bigint NOT NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(conversation_id,seq),UNIQUE KEY uq_tm_ai_qa_input(input_digest),KEY idx_tm_ai_qa_owner(user_id,conversation_id,seq),
 CONSTRAINT fk_tm_ai_qa_owner FOREIGN KEY(conversation_id,user_id) REFERENCES tm_ai_conversations(id,user_id),
 CONSTRAINT fk_tm_ai_qa_message FOREIGN KEY(conversation_id,seq) REFERENCES tm_ai_conversation_messages(conversation_id,seq),
 CONSTRAINT chk_tm_ai_qa_request CHECK(conversation_id BETWEEN 1 AND 9007199254740991 AND seq BETWEEN 1 AND 9007199254740990
 AND user_id BETWEEN 1 AND 9007199254740991 AND input_digest REGEXP '^[a-f0-9]{64}$' AND message_hash REGEXP '^[a-f0-9]{64}$'
 AND JSON_VALID(proof_json) AND OCTET_LENGTH(proof_json)<=16384 AND JSON_VALID(citations_json) AND OCTET_LENGTH(citations_json)<=2048
 AND calls BETWEEN 0 AND 3 AND reserved_tokens BETWEEN 0 AND 1048576 AND version BETWEEN 1 AND 9007199254740991
 AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at
 AND ((state='queued' AND reason='' AND calls=0 AND reserved_tokens=0 AND quota_day IS NULL AND version=1)
 OR (state='running' AND reason='' AND quota_day IS NOT NULL AND version>=2)
 OR (state='answered' AND reason='' AND calls=3 AND quota_day IS NOT NULL AND version>=3)
 OR (state='refused' AND reason IN('no_evidence','out_of_scope','request_not_allowed','unverified') AND quota_day IS NOT NULL AND version>=3)
 OR (state='failed' AND reason IN('execution_failed','cancelled','unconfirmed') AND version>=2)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='私有用户问题的唯一执行及闭集结果引用';

CREATE TABLE IF NOT EXISTS tm_ai_qa_calls (
 input_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,step_no int NOT NULL,
 call_json text NOT NULL,state varchar(10) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,result_json text NULL,
 created_at datetime(3) NOT NULL,updated_at datetime(3) NOT NULL,
 PRIMARY KEY(input_digest,step_no),
 CONSTRAINT fk_tm_ai_qa_call FOREIGN KEY(input_digest) REFERENCES tm_ai_qa_requests(input_digest),
 CONSTRAINT chk_tm_ai_qa_call CHECK(step_no BETWEEN 1 AND 3 AND JSON_VALID(call_json) AND OCTET_LENGTH(call_json)<=2048
 AND (result_json IS NULL OR (JSON_VALID(result_json) AND OCTET_LENGTH(result_json)<=8192))
 AND created_at>='1970-01-01 00:00:00.001' AND updated_at>=created_at
 AND ((state='reserved' AND result_json IS NULL AND updated_at=created_at) OR (state='recorded' AND result_json IS NOT NULL)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='问答逐次预算预约及闭集判定存证，不存模型自由正文';

DROP TRIGGER IF EXISTS tm_ai_qa_scope_insert;
DROP TRIGGER IF EXISTS tm_ai_qa_scope_update;
DROP TRIGGER IF EXISTS tm_ai_qa_limits_insert;
DROP TRIGGER IF EXISTS tm_ai_qa_limits_update;
DROP TRIGGER IF EXISTS tm_ai_qa_quota_insert;
DROP TRIGGER IF EXISTS tm_ai_qa_quota_update;
DROP TRIGGER IF EXISTS tm_ai_qa_request_insert;
DROP TRIGGER IF EXISTS tm_ai_qa_request_update;
DROP TRIGGER IF EXISTS tm_ai_qa_call_insert;
DROP TRIGGER IF EXISTS tm_ai_qa_call_update;
DELIMITER $$
CREATE TRIGGER tm_ai_qa_scope_insert BEFORE INSERT ON tm_ai_qa_scopes FOR EACH ROW
BEGIN
 IF NEW.enabled<>0 OR NEW.version<>1 OR NEW.created_at<>NEW.updated_at THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA scope starts disabled'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_scope_update BEFORE UPDATE ON tm_ai_qa_scopes FOR EACH ROW
BEGIN
 IF NOT(NEW.subject_type=OLD.subject_type AND NEW.subject_id=OLD.subject_id AND NEW.created_at=OLD.created_at AND NEW.version=OLD.version+1 AND NEW.updated_at>=OLD.updated_at)
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA scope version must advance'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_limits_insert BEFORE INSERT ON tm_ai_qa_limits FOR EACH ROW
BEGIN
 IF NEW.enabled<>0 OR NEW.version<>1 OR NEW.created_at<>NEW.updated_at THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA limits start disabled'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_limits_update BEFORE UPDATE ON tm_ai_qa_limits FOR EACH ROW
BEGIN
 IF NOT(NEW.id=OLD.id AND NEW.created_at=OLD.created_at AND NEW.version=OLD.version+1 AND NEW.updated_at>=OLD.updated_at)
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA limits version must advance'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_quota_insert BEFORE INSERT ON tm_ai_qa_quotas FOR EACH ROW
BEGIN
 IF NEW.requests<>0 OR NEW.tokens<>0 OR NEW.created_at<>NEW.updated_at THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA quota starts empty'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_quota_update BEFORE UPDATE ON tm_ai_qa_quotas FOR EACH ROW
BEGIN
 IF NOT(NEW.quota_day=OLD.quota_day AND NEW.user_id=OLD.user_id AND NEW.created_at=OLD.created_at AND NEW.updated_at>=OLD.updated_at
 AND ((NEW.requests=OLD.requests AND NEW.tokens=OLD.tokens AND NEW.updated_at=OLD.updated_at)
 OR (NEW.requests=OLD.requests+1 AND NEW.tokens>OLD.tokens AND NEW.tokens<=OLD.tokens+1048576)))
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA quota reservation cannot be released'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_request_insert BEFORE INSERT ON tm_ai_qa_requests FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_conversation_messages m WHERE m.conversation_id=NEW.conversation_id AND m.seq=NEW.seq
 AND m.user_id=NEW.user_id AND m.req_hash=NEW.message_hash AND m.kind='question' AND (m.skill='material_qa' OR (m.skill='report_qa' AND m.task_id BETWEEN 1 AND 9007199254740991 AND LEFT(JSON_UNQUOTE(JSON_EXTRACT(NEW.proof_json,'$.ScopeApproval')),7)='report:')) AND m.created_at=NEW.created_at;
 IF matched<>1 OR NEW.state<>'queued' OR NEW.version<>1
 OR NOT(CAST(JSON_UNQUOTE(JSON_EXTRACT(NEW.proof_json,'$.Identity.UserID')) AS UNSIGNED)<=>NEW.user_id)
 OR NOT(CAST(JSON_UNQUOTE(JSON_EXTRACT(NEW.proof_json,'$.Identity.ConversationID')) AS UNSIGNED)<=>NEW.conversation_id)
 OR NOT(CAST(JSON_UNQUOTE(JSON_EXTRACT(NEW.proof_json,'$.Identity.MessageSeq')) AS UNSIGNED)<=>NEW.seq)
 OR NOT(JSON_UNQUOTE(JSON_EXTRACT(NEW.proof_json,'$.InputDigest'))<=>NEW.input_digest)
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA request requires original owned question'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_request_update BEFORE UPDATE ON tm_ai_qa_requests FOR EACH ROW
BEGIN
 IF NOT(NEW.conversation_id=OLD.conversation_id AND NEW.seq=OLD.seq AND NEW.user_id=OLD.user_id AND NEW.input_digest=OLD.input_digest
 AND NEW.message_hash=OLD.message_hash AND BINARY NEW.proof_json=BINARY OLD.proof_json AND NEW.created_at=OLD.created_at
 AND NEW.version=OLD.version+1 AND NEW.updated_at>=OLD.updated_at
 AND ((OLD.state='queued' AND NEW.state='running' AND NEW.quota_day=DATE(UTC_TIMESTAMP(3)) AND NEW.calls=0 AND NEW.reserved_tokens=0)
 OR (OLD.state='running' AND NEW.state='running' AND (NEW.quota_day<=>OLD.quota_day) AND NEW.calls=OLD.calls+1 AND NEW.reserved_tokens>OLD.reserved_tokens)
 OR (OLD.state='running' AND NEW.state IN('answered','refused','failed') AND (NEW.quota_day<=>OLD.quota_day) AND NEW.calls=OLD.calls AND NEW.reserved_tokens=OLD.reserved_tokens)
 OR (OLD.state='queued' AND NEW.state='failed' AND NEW.quota_day IS NULL AND NEW.calls=0 AND NEW.reserved_tokens=0)))
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA request identity or terminal state is immutable'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_call_insert BEFORE INSERT ON tm_ai_qa_calls FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_qa_requests r WHERE r.input_digest=NEW.input_digest AND r.state='running' AND NEW.step_no=r.calls+1;
 IF matched<>1 OR NEW.state<>'reserved' OR NOT(JSON_UNQUOTE(JSON_EXTRACT(NEW.call_json,'$.InputDigest'))<=>NEW.input_digest)
 OR NOT(CAST(JSON_UNQUOTE(JSON_EXTRACT(NEW.call_json,'$.Step')) AS UNSIGNED)<=>NEW.step_no)
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA call requires one running reservation'; END IF;
END$$
CREATE TRIGGER tm_ai_qa_call_update BEFORE UPDATE ON tm_ai_qa_calls FOR EACH ROW
BEGIN
 IF NOT(NEW.input_digest=OLD.input_digest AND NEW.step_no=OLD.step_no AND BINARY NEW.call_json=BINARY OLD.call_json
 AND NEW.created_at=OLD.created_at AND NEW.updated_at>=OLD.updated_at AND OLD.state='reserved' AND NEW.state='recorded')
 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA call result is immutable'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_qa_scope_no_delete;
CREATE TRIGGER tm_ai_qa_scope_no_delete BEFORE DELETE ON tm_ai_qa_scopes FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA scope is disabled, not deleted';
DROP TRIGGER IF EXISTS tm_ai_qa_limits_no_delete;
CREATE TRIGGER tm_ai_qa_limits_no_delete BEFORE DELETE ON tm_ai_qa_limits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA limits are disabled, not deleted';
DROP TRIGGER IF EXISTS tm_ai_qa_quota_no_delete;
CREATE TRIGGER tm_ai_qa_quota_no_delete BEFORE DELETE ON tm_ai_qa_quotas FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA quota is immutable history';
DROP TRIGGER IF EXISTS tm_ai_qa_request_no_delete;
CREATE TRIGGER tm_ai_qa_request_no_delete BEFORE DELETE ON tm_ai_qa_requests FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA request is immutable history';
DROP TRIGGER IF EXISTS tm_ai_qa_call_no_delete;
CREATE TRIGGER tm_ai_qa_call_no_delete BEFORE DELETE ON tm_ai_qa_calls FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='QA call is immutable history';
