-- 固定 agent_call:1 的内部资料绑定；不启用执行、不补绑历史记录。
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS tm_ai_agent_material_bindings (
 task_id bigint UNSIGNED NOT NULL,
 user_id bigint UNSIGNED NOT NULL,
 run_id bigint UNSIGNED NOT NULL,
 materials_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 content_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 manifest mediumblob NOT NULL,
 created_at datetime(3) NOT NULL,
 updated_at datetime(3) NOT NULL,
 PRIMARY KEY(task_id),
 UNIQUE KEY uk_tm_ai_agent_material_run(run_id),
 CONSTRAINT fk_tm_ai_agent_material_task FOREIGN KEY(task_id,user_id) REFERENCES tm_ai_tasks(id,user_id),
 CONSTRAINT fk_tm_ai_agent_material_run FOREIGN KEY(run_id) REFERENCES tm_ai_agent_runs(id),
 CONSTRAINT chk_tm_ai_agent_material CHECK (
  user_id BETWEEN 1 AND 9007199254740991
  AND OCTET_LENGTH(materials_digest)=64 AND materials_digest REGEXP '^[a-f0-9]{64}$'
  AND OCTET_LENGTH(content_digest)=64 AND content_digest REGEXP '^[a-f0-9]{64}$'
  AND OCTET_LENGTH(manifest) BETWEEN 2 AND 262144
  AND JSON_VALID(CONVERT(manifest USING utf8mb4))
  AND created_at=updated_at
 )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='固定第一步资料包不可变绑定；正文仅内部恢复';
DROP TRIGGER IF EXISTS tm_ai_agent_material_insert;
DELIMITER $$
CREATE TRIGGER tm_ai_agent_material_insert BEFORE INSERT ON tm_ai_agent_material_bindings FOR EACH ROW
BEGIN
 DECLARE matched int;
 SELECT COUNT(*) INTO matched FROM tm_ai_tasks t
 JOIN tm_ai_agent_runs r ON r.id=NEW.run_id
 JOIN tm_ai_agent_events e ON e.run_id=r.id AND e.seq=1 AND e.event='submit_reserved'
 WHERE t.id=NEW.task_id AND t.user_id=NEW.user_id
  AND t.status=2 AND t.current_step=0 AND t.cancel_req=0 AND t.fence>0
  AND t.lease_until>UTC_TIMESTAMP(3) AND t.deadline_at>UTC_TIMESTAMP(3)
  AND r.member_id=t.user_id AND BINARY r.subject_type=BINARY t.subject_type AND r.subject_id=t.subject_id
  AND BINARY r.symbol=BINARY '' AND BINARY r.materials_digest=BINARY NEW.materials_digest
  AND r.state='submitting' AND r.event_seq=1
  -- 固定ASCII字段的Go json.Marshal字节；JSON_ARRAY的空格不能参与此哈希。
  AND BINARY r.request_id=BINARY SHA2(CONVERT(CONCAT('["task_agent_request_v1",',CAST(t.user_id AS CHAR),',',CAST(t.id AS CHAR),',"',t.task_no,'","agent_call:1"]') USING ascii),256);
 IF matched<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Agent material task or original request mismatch'; END IF;
END$$
DELIMITER ;
DROP TRIGGER IF EXISTS tm_ai_agent_material_no_update;
CREATE TRIGGER tm_ai_agent_material_no_update BEFORE UPDATE ON tm_ai_agent_material_bindings FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Agent material binding is immutable';
DROP TRIGGER IF EXISTS tm_ai_agent_material_no_delete;
CREATE TRIGGER tm_ai_agent_material_no_delete BEFORE DELETE ON tm_ai_agent_material_bindings FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Agent material binding cannot be deleted';
