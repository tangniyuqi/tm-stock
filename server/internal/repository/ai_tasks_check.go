package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// AITaskSchemaVerifier 核对任务核及其账本依赖；只写随机探针后回滚，不运行迁移。
// 未来任务服务/worker 必须在接受任务前调用；通过不代表历史对账或公开 AI 获准。
type AITaskSchemaVerifier struct{ db *sql.DB }

func NewAITaskSchemaVerifier(db *sql.DB) *AITaskSchemaVerifier {
	return &AITaskSchemaVerifier{db: db}
}

func (v *AITaskSchemaVerifier) Verify(ctx context.Context) error {
	if v == nil || v.db == nil {
		return fmt.Errorf("任务自检数据库未初始化")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启任务约束探针失败: %w", err)
	}
	// 任何失败均不提交；上下文取消时 database/sql 同样负责回滚。
	defer func() { _ = tx.Rollback() }()
	for _, verify := range []func(context.Context, *sql.Tx) error{
		verifyPointsSession, verifyPointsSchemaMetadata, verifyAITaskMetadata, verifyAITaskBudgetMetadata, verifyAITaskStageMetadata,
		probePointsConstraints, probeAITaskConstraints, probeAITaskBudgetConstraints, probeAITaskStageConstraints,
	} {
		if err := verify(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("回滚任务约束探针失败: %w", err)
	}
	return nil
}

func verifyAITaskMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{"tm_ai_tasks", "tm_ai_task_events"} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=? AND engine='InnoDB'`, table).Scan(&count); err != nil {
			return fmt.Errorf("核对任务表引擎失败: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("任务表 %s 缺失或不是 InnoDB", table)
		}
	}
	for _, check := range []struct{ table, name string }{
		{"tm_ai_tasks", "chk_tm_ai_task"}, {"tm_ai_tasks", "chk_tm_ai_task_budget"}, {"tm_ai_task_events", "chk_tm_ai_task_event"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name=? AND constraint_name=? AND constraint_type='CHECK' AND enforced='YES'`, check.table, check.name).Scan(&count); err != nil {
			return fmt.Errorf("核对任务 CHECK 失败: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("任务 CHECK %s 缺失或未强制执行", check.name)
		}
	}
	// 检查全部运行列，不把早期 theme_id 结构、浮点金额、非二进制幂等键或低精度租约当作兼容。
	for _, column := range []struct{ table, name, kind, nullable, collation string }{
		{"tm_ai_tasks", "id", "bigint unsigned", "NO", ""}, {"tm_ai_tasks", "user_id", "bigint unsigned", "NO", ""},
		{"tm_ai_tasks", "task_no", "char(32)", "NO", "ascii_bin"}, {"tm_ai_tasks", "idem_key", "varchar(80)", "NO", "ascii_bin"},
		{"tm_ai_tasks", "req_hash", "char(64)", "NO", "ascii_bin"}, {"tm_ai_tasks", "analysis_type", "tinyint", "NO", ""},
		{"tm_ai_tasks", "subject_type", "varchar(8)", "NO", "ascii_bin"}, {"tm_ai_tasks", "subject_id", "bigint unsigned", "NO", ""},
		{"tm_ai_tasks", "hold_id", "bigint unsigned", "NO", ""}, {"tm_ai_tasks", "price_ver", "varchar(32)", "NO", "ascii_bin"},
		{"tm_ai_tasks", "price_points", "bigint", "NO", ""}, {"tm_ai_tasks", "budget_json", "json", "NO", ""},
		{"tm_ai_tasks", "status", "tinyint", "NO", ""}, {"tm_ai_tasks", "attempt", "int", "NO", ""},
		{"tm_ai_tasks", "current_step", "int", "NO", ""}, {"tm_ai_tasks", "not_before", "datetime(3)", "NO", ""},
		{"tm_ai_tasks", "deadline_at", "datetime(3)", "NO", ""}, {"tm_ai_tasks", "lease_owner", "varchar(64)", "YES", "ascii_bin"},
		{"tm_ai_tasks", "lease_until", "datetime(3)", "YES", ""}, {"tm_ai_tasks", "fence", "bigint", "NO", ""},
		{"tm_ai_tasks", "event_seq", "bigint", "NO", ""}, {"tm_ai_tasks", "cancel_req", "tinyint", "NO", ""},
		{"tm_ai_tasks", "error_code", "varchar(32)", "NO", "ascii_bin"}, {"tm_ai_tasks", "created_at", "datetime(3)", "NO", ""},
		{"tm_ai_tasks", "updated_at", "datetime(3)", "NO", ""},
		{"tm_ai_task_events", "task_id", "bigint unsigned", "NO", ""}, {"tm_ai_task_events", "seq", "bigint", "NO", ""},
		{"tm_ai_task_events", "type", "varchar(24)", "NO", "ascii_bin"}, {"tm_ai_task_events", "status", "tinyint", "NO", ""},
		{"tm_ai_task_events", "step_no", "int", "NO", ""}, {"tm_ai_task_events", "error_code", "varchar(32)", "NO", "ascii_bin"},
		{"tm_ai_task_events", "created_at", "datetime(3)", "NO", ""}, {"tm_ai_task_events", "updated_at", "datetime(3)", "NO", ""},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable=? AND COALESCE(collation_name,'')=? AND extra NOT LIKE '%GENERATED%'`, column.table, column.name, column.kind, column.nullable, column.collation).Scan(&count); err != nil {
			return fmt.Errorf("核对任务列失败: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("任务列 %s.%s 缺失或类型不兼容", column.table, column.name)
		}
	}
	for _, index := range []struct {
		table, name, columns string
		nonUnique            int
	}{
		{"tm_ai_tasks", "PRIMARY", "id", 0}, {"tm_ai_tasks", "uk_tm_ai_task_owner", "id,user_id", 0},
		{"tm_ai_tasks", "uk_tm_ai_task_no", "task_no", 0}, {"tm_ai_tasks", "uk_tm_ai_task_idem", "user_id,idem_key", 0},
		{"tm_ai_tasks", "uk_tm_ai_task_hold", "hold_id", 0}, {"tm_ai_tasks", "idx_tm_ai_task_history", "user_id,id", 1},
		{"tm_ai_tasks", "idx_tm_ai_task_claim", "status,not_before,lease_until,id", 1}, {"tm_ai_tasks", "idx_tm_ai_task_deadline", "status,deadline_at,id", 1},
		{"tm_ai_task_events", "PRIMARY", "task_id,seq", 0},
	} {
		var actual string
		// 前缀/降序/表达式列也进入指纹，不能过滤掉额外列后误判索引一致。
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(COALESCE(column_name,'#expression'),IF(sub_part IS NULL,'',':prefix'),IF(collation='A','',':order')) ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=? AND is_visible='YES'`, index.table, index.name, index.nonUnique).Scan(&actual); err != nil {
			return fmt.Errorf("核对任务索引失败: %w", err)
		}
		if actual != index.columns {
			return fmt.Errorf("任务索引 %s 缺失或不可用", index.name)
		}
	}
	for _, fk := range []struct{ table, name, target, columns string }{
		{"tm_ai_tasks", "fk_tm_ai_task_hold", "tm_points_holds", "hold_id:id,user_id:user_id"},
		{"tm_ai_task_events", "fk_tm_ai_event_task", "tm_ai_tasks", "task_id:id"},
	} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name=? AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, fk.table, fk.name, fk.target).Scan(&actual); err != nil {
			return fmt.Errorf("核对任务归属外键失败: %w", err)
		}
		if actual != fk.columns {
			return fmt.Errorf("任务外键 %s 缺失或归属不一致", fk.name)
		}
	}
	return nil
}

// 不调用业务服务，避免仓储反向依赖；探针仅验证行约束，不伪装成余额对账。
func probeAITaskConstraints(ctx context.Context, tx *sql.Tx) error {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Errorf("生成任务探针编号失败: %w", err)
	}
	uid := int64(binary.BigEndian.Uint64(random[:])&((1<<62)-2)) | (1 << 62)
	if _, err := tx.ExecContext(ctx, `INSERT INTO tm_points_accounts(user_id,created_at,updated_at) VALUES(?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)),(?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, uid+1); err != nil {
		return fmt.Errorf("写入任务依赖账户探针失败: %w", err)
	}
	prefix := fmt.Sprintf("task_probe_%x_", uid)
	holds := [2]int64{}
	for i := range holds {
		key := fmt.Sprintf("%sfreeze_%d", prefix, i)
		r, err := tx.ExecContext(ctx, `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id,created_at,updated_at) VALUES(?,?,?,'freeze','ai_task',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, key, strings.Repeat("0", 64), key)
		if err != nil {
			return fmt.Errorf("写入任务依赖流水失败: %w", err)
		}
		freeze, err := r.LastInsertId()
		if err != nil {
			return err
		}
		r, err = tx.ExecContext(ctx, `INSERT INTO tm_points_holds(user_id,biz_type,biz_id,freeze_txn_id,amount,price_ver,expire_at,created_at,updated_at) VALUES(?,'ai_task',?,?,7,'probe',UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, key, freeze)
		if err != nil {
			return fmt.Errorf("写入任务依赖冻结单失败: %w", err)
		}
		holds[i], err = r.LastInsertId()
		if err != nil {
			return err
		}
	}
	now, err := PointsTx(tx).Now(ctx)
	if err != nil {
		return err
	}
	base := task.Task{UserID: uid, No: fmt.Sprintf("%032x", uid), Key: prefix + "create", Hash: strings.Repeat("0", 64), AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, HoldID: holds[0], PriceVersion: "probe", PricePoints: 7, Budget: task.Budget{MaxAttempts: 3, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 3600000}, Status: task.Queued, NotBefore: now, Deadline: now.Add(time.Hour), EventSeq: 1}
	base.ID, err = insertAITaskProbe(ctx, tx, base, "")
	if err != nil {
		return fmt.Errorf("写入任务正例失败: %w", err)
	}
	event := task.Event{TaskID: base.ID, Seq: 1, Type: "created", Status: task.Queued}
	adapter := AITaskTx(tx)
	if err := adapter.InsertEvent(ctx, event); err != nil {
		return err
	}
	spare := base
	spare.ID, spare.HoldID, spare.No, spare.Key = 0, holds[1], fmt.Sprintf("%032x", uid+1), prefix+"spare"
	for _, p := range []struct {
		name   string
		change func(*task.Task)
		budget string
	}{
		{"主体类型", func(v *task.Task) { v.SubjectType = "stock" }, ""}, {"主体编号", func(v *task.Task) { v.SubjectID = 0 }, ""},
		{"分析类型", func(v *task.Task) { v.AnalysisType = 3 }, ""}, {"金额", func(v *task.Task) { v.PricePoints = 0 }, ""},
		{"任务编号", func(v *task.Task) { v.No = "bad" }, ""}, {"摘要", func(v *task.Task) { v.Hash = "bad" }, ""},
		{"幂等键", func(v *task.Task) { v.Key = "short" }, ""}, {"价格版本", func(v *task.Task) { v.PriceVersion = "" }, ""},
		{"状态", func(v *task.Task) { v.Status = 0 }, ""}, {"尝试次数", func(v *task.Task) { v.Attempt = -1 }, ""},
		{"阶段编号", func(v *task.Task) { v.CurrentStep = -1 }, ""}, {"围栏", func(v *task.Task) { v.Fence = -1 }, ""},
		{"事件序号", func(v *task.Task) { v.EventSeq = 0 }, ""}, {"截止时点", func(v *task.Task) { v.Deadline = now.Add(-time.Hour) }, ""},
		{"缺少租约", func(v *task.Task) { v.Status = task.Running }, ""},
		{"排队带租约", func(v *task.Task) { v.LeaseOwner = prefix + "owner" }, ""},
		{"错误码", func(v *task.Task) { v.ErrorCode = "internal_failure" }, ""},
		{"失败缺错误码", func(v *task.Task) { v.Status = task.Failed }, ""},
		{"预算缺字段", nil, `{}`}, {"预算非对象", nil, `[]`},
		{"预算零调用", func(v *task.Task) { v.Budget.MaxCalls = 0 }, ""},
		{"预算零 token", func(v *task.Task) { v.Budget.MaxTokens = 0 }, ""},
		{"预算零页数", func(v *task.Task) { v.Budget.MaxPages = 0 }, ""},
		{"预算超限尝试", func(v *task.Task) { v.Budget.MaxAttempts = 101 }, ""},
		{"预算不足时限", func(v *task.Task) { v.Budget.MaxDurationMS = 999 }, ""},
		{"预算超长时限", func(v *task.Task) { v.Budget.MaxDurationMS = 86400001 }, ""},
		{"预算非整数", nil, `{"max_attempts":3,"max_calls":4.5,"max_tokens":1000,"max_pages":3,"max_duration_ms":3600000}`},
		{"预算多字段", nil, `{"max_attempts":3,"max_calls":4,"max_tokens":1000,"max_pages":3,"max_duration_ms":3600000,"extra":1}`},
	} {
		bad := spare
		if p.change != nil {
			p.change(&bad)
		}
		if _, err := insertAITaskProbe(ctx, tx, bad, p.budget); !pointsProbeError(err, 3819) {
			return fmt.Errorf("任务探针 %s 未被 CHECK 拒绝: %w", p.name, errOrMissing(err))
		}
	}
	for _, p := range []struct {
		name   string
		change func(*task.Task)
		code   uint16
	}{
		{"编号唯一", func(v *task.Task) { v.No = base.No }, 1062},
		{"幂等唯一", func(v *task.Task) { v.Key = base.Key }, 1062},
		{"冻结唯一", func(v *task.Task) { v.HoldID = base.HoldID }, 1062},
		{"跨账户冻结", func(v *task.Task) { v.UserID++ }, 1452},
	} {
		bad := spare
		p.change(&bad)
		if _, err := insertAITaskProbe(ctx, tx, bad, ""); !pointsProbeError(err, p.code) {
			return fmt.Errorf("任务探针 %s 未按预期拒绝: %w", p.name, errOrMissing(err))
		}
	}
	for _, p := range []struct {
		change func(*task.Event)
		code   uint16
	}{
		{func(e *task.Event) { e.Seq = 0 }, 3819}, {func(e *task.Event) { e.Seq = 2; e.Step = -1 }, 3819},
		{func(e *task.Event) { e.Seq = 2; e.Type = "bad" }, 3819},
		{func(e *task.Event) { e.Seq = 2; e.Status = task.Succeeded }, 3819},
		{func(e *task.Event) { e.Seq = 2; e.ErrorCode = "internal_failure" }, 3819},
		{func(e *task.Event) {}, 1062}, {func(e *task.Event) { e.TaskID = 0 }, 1452},
	} {
		bad := event
		p.change(&bad)
		// 业务仓储将 1062 映射为领域冲突；探针须直接核对实际 MySQL 错误码。
		_, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_task_events(task_id,seq,type,status,step_no,error_code,created_at,updated_at) VALUES(?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, bad.TaskID, bad.Seq, bad.Type, bad.Status, bad.Step, bad.ErrorCode)
		if !pointsProbeError(err, p.code) {
			return fmt.Errorf("任务事件探针未按预期拒绝: %w", errOrMissing(err))
		}
	}
	// 不读触发器元数据（业务账号不授 TRIGGER），逐字段真实验证不可变保护。
	for _, assignment := range []string{
		"id=id+1", "user_id=user_id+1", "task_no=REPEAT('f',32)", "idem_key='task_probe_changed_key'", "req_hash=REPEAT('f',64)",
		"analysis_type=2", "subject_type='topic'", "subject_id=2", fmt.Sprintf("hold_id=%d", holds[1]),
		"price_ver='changed'", "price_points=8", "budget_json=JSON_SET(budget_json,'$.max_calls',5)",
		"deadline_at=deadline_at+INTERVAL 1 SECOND", "created_at=created_at-INTERVAL 1 SECOND",
	} {
		// 赋值片段只来自此封闭常量集合与探针编号，不接受客户端输入。
		if _, err := tx.ExecContext(ctx, "UPDATE tm_ai_tasks SET "+assignment+" WHERE id=?", base.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("任务身份保护失效: %w", errOrMissing(err))
		}
	}
	next := base
	// 合法抢占不能顺便推进阶段；步骤推进由检查点探针验证。
	next.Status, next.Attempt, next.CurrentStep, next.Fence, next.EventSeq = task.Running, 2, 0, 2, 2
	next.CancelRequested, next.LeaseOwner = true, prefix+"owner"
	until := now.Add(time.Minute)
	next.LeaseUntil = &until
	if err := adapter.SetTask(ctx, base, next); err != nil {
		return fmt.Errorf("任务合法抢占探针失败: %w", err)
	}
	for _, column := range []string{"attempt", "current_step", "fence", "event_seq", "cancel_req"} {
		if _, err := tx.ExecContext(ctx, "UPDATE tm_ai_tasks SET "+column+"="+column+"-1 WHERE id=?", base.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("任务单调状态保护失效: %w", errOrMissing(err))
		}
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT task_probe_terminal"); err != nil {
		return fmt.Errorf("建立任务终态探针保存点失败: %w", err)
	}
	for _, state := range []struct {
		status int
		code   string
	}{{task.Succeeded, ""}, {task.Failed, "internal_failure"}, {task.Cancelled, "cancelled"}, {task.Expired, "expired"}} {
		terminal := next
		terminal.Status, terminal.ErrorCode, terminal.LeaseOwner, terminal.LeaseUntil = state.status, state.code, "", nil
		if err := adapter.SetTask(ctx, next, terminal); err != nil {
			return fmt.Errorf("任务合法终结探针失败: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE tm_ai_tasks SET updated_at=updated_at WHERE id=?", base.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("任务终态保护失效: %w", errOrMissing(err))
		}
		if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT task_probe_terminal"); err != nil {
			return fmt.Errorf("回滚任务终态探针保存点失败: %w", err)
		}
	}
	for _, query := range []string{
		"DELETE FROM tm_ai_tasks WHERE id=?",
		"UPDATE tm_ai_task_events SET updated_at=updated_at WHERE task_id=?", "DELETE FROM tm_ai_task_events WHERE task_id=?",
	} {
		if _, err := tx.ExecContext(ctx, query, base.ID); !pointsProbeError(err, 1644) {
			return fmt.Errorf("任务终态/历史保护失效: %w", errOrMissing(err))
		}
	}
	spare.AnalysisType, spare.SubjectType = 2, task.Topic
	// 使用真正的仓储创建路径（它省略计数/租约列），不能让显式探针掩盖错误默认值。
	spare.ID, err = adapter.InsertTask(ctx, spare)
	if err != nil {
		return fmt.Errorf("专题任务正例失败: %w", err)
	}
	stored, err := adapter.LockTask(ctx, uid, spare.ID)
	if err != nil {
		return fmt.Errorf("读取任务创建正例失败: %w", err)
	}
	if stored.Attempt != 0 || stored.CurrentStep != 0 || stored.Fence != 0 || stored.CancelRequested || stored.LeaseOwner != "" || stored.LeaseUntil != nil || stored.ErrorCode != "" || stored.EventSeq != 1 || stored.Status != task.Queued {
		return fmt.Errorf("任务创建默认状态不兼容")
	}
	return nil
}

func insertAITaskProbe(ctx context.Context, tx *sql.Tx, v task.Task, budget string) (int64, error) {
	if budget == "" {
		encoded, err := json.Marshal(v.Budget)
		if err != nil {
			return 0, err
		}
		budget = string(encoded)
	}
	owner, until := taskLeaseValues(v)
	r, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_tasks(user_id,task_no,idem_key,req_hash,analysis_type,subject_type,subject_id,hold_id,price_ver,price_points,budget_json,status,attempt,current_step,not_before,deadline_at,lease_owner,lease_until,fence,event_seq,cancel_req,error_code,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.UserID, v.No, v.Key, v.Hash, v.AnalysisType, v.SubjectType, v.SubjectID, v.HoldID, v.PriceVersion, v.PricePoints, budget, v.Status, v.Attempt, v.CurrentStep, pointsUTC(v.NotBefore), pointsUTC(v.Deadline), owner, until, v.Fence, v.EventSeq, v.CancelRequested, v.ErrorCode)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}
