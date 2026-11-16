package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/materialpack"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// AgentMaterialTaskIdentity 只能由可信任务编排提供；操作人不是会员身份。
type AgentMaterialTaskIdentity struct {
	TaskID, UserID int64
	TaskNo, Owner  string
	Fence          int64
}

var materialTaskNo = regexp.MustCompile("^[a-f0-9]{32}$")

func (v AgentMaterialTaskIdentity) valid() bool {
	return v.TaskID > 0 && v.UserID > 0 && v.UserID <= 9007199254740991 && materialTaskNo.MatchString(v.TaskNo) && task.ValidOwner(v.Owner) && v.Fence > 0
}

// RequestID 固定第一步身份，与旧Plan编码一致，不包含可变资料或配置。
func (v AgentMaterialTaskIdentity) RequestID() string {
	raw, _ := json.Marshal([]any{"task_agent_request_v1", v.UserID, v.TaskID, v.TaskNo, "agent_call:1"})
	return task.StageDigest(raw)
}

// AgentMaterialBinding 不导出原始manifest；默认JSON和格式化均不泄露正文。
// 此对象仅供可信内部编排，不代表当前会员或来源授权。
type AgentMaterialBinding struct {
	taskID, userID                 int64
	taskNo                         string
	record                         agents.Record
	materialsDigest, contentDigest string
	manifest                       []byte
}

func (AgentMaterialBinding) String() string            { return "Agent持久资料绑定（正文已隐藏）" }
func (b AgentMaterialBinding) GoString() string        { return b.String() }
func (b AgentMaterialBinding) TaskID() int64           { return b.taskID }
func (b AgentMaterialBinding) UserID() int64           { return b.userID }
func (b AgentMaterialBinding) TaskNo() string          { return b.taskNo }
func (b AgentMaterialBinding) Record() agents.Record   { return b.record }
func (b AgentMaterialBinding) MaterialsDigest() string { return b.materialsDigest }
func (b AgentMaterialBinding) ContentDigest() string   { return b.contentDigest }

// Pack 每次从不可变行摘要恢复独立副本；调用前仍需当前权限复核。
func (b AgentMaterialBinding) Pack() (*materialpack.Pack, error) {
	p, err := materialpack.RestoreManifest(b.manifest, b.contentDigest)
	if err != nil || !b.record.Valid() || !b.record.Request.IsReference() || b.taskID <= 0 || !materialTaskNo.MatchString(b.taskNo) {
		return nil, agents.ErrInvalid
	}
	s := p.Scope()
	req := b.record.Request
	identity := AgentMaterialTaskIdentity{TaskID: b.taskID, UserID: b.userID, TaskNo: b.taskNo}
	if req.RequestID != identity.RequestID() || p.MaterialsDigest() != b.materialsDigest || req.MaterialsDigest != b.materialsDigest || s.MemberID != b.userID || req.MemberID != b.userID || s.SubjectType != req.SubjectType || s.SubjectID != req.SubjectID {
		return nil, agents.ErrInvalid
	}
	return p, nil
}

// AgentMaterialBindingTx 是agentTx的独立能力，不改变旧agents.Tx及股票路径。
// 调用方必须将任何错误返回给WithinAgentTx，不能吞错提交部分预留。
// Prepare/Recheck须在进入写事务前完成，不可持任务/配置锁另开连接读取资料。
// newlyReserved=false仅为历史重放，绝不授予发送权；true也只表示持久预留，未完成当前发送准入。
type AgentMaterialBindingTx interface {
	ReserveMaterialBinding(context.Context, AgentMaterialTaskIdentity, agents.Profile, agents.Request, int64, *materialpack.Pack) (AgentMaterialBinding, bool, error)
}

var _ AgentMaterialBindingTx = (*agentTx)(nil)

func (t *agentTx) ReserveMaterialBinding(ctx context.Context, identity AgentMaterialTaskIdentity, profile agents.Profile, req agents.Request, actorID int64, pack *materialpack.Pack) (AgentMaterialBinding, bool, error) {
	fail := func(err error) (AgentMaterialBinding, bool, error) { return AgentMaterialBinding{}, false, err }
	if ctx == nil || t == nil || t.tx == nil || !identity.valid() || !profile.Valid() || !req.Valid() || !req.IsReference() || actorID <= 0 || pack == nil {
		return fail(agents.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	scope := pack.Scope()
	if req.RequestID != identity.RequestID() || req.MemberID != identity.UserID || scope.MemberID != identity.UserID || scope.SubjectType != req.SubjectType || scope.SubjectID != req.SubjectID || pack.MaterialsDigest() != req.MaterialsDigest {
		return fail(agents.ErrConflict)
	}
	raw, err := materialpack.MarshalManifest(pack)
	if err != nil {
		return fail(agents.ErrInvalid)
	}
	// 全局顺序：任务→配置→执行登记→绑定。不在锁内读取当前来源或调用网络。
	var taskNo, subjectType string
	var subjectID int64
	var eligible bool
	err = t.tx.QueryRowContext(ctx, `SELECT task_no,subject_type,subject_id,
 COALESCE((status=2 AND current_step=0 AND cancel_req=0 AND fence=? AND BINARY lease_owner=BINARY ? AND lease_until>UTC_TIMESTAMP(3) AND deadline_at>UTC_TIMESTAMP(3)),0)
 FROM tm_ai_tasks WHERE id=? AND user_id=? FOR UPDATE`, identity.Fence, identity.Owner, identity.TaskID, identity.UserID).Scan(&taskNo, &subjectType, &subjectID, &eligible)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(agents.ErrNotFound)
	}
	if err != nil {
		return fail(fmt.Errorf("锁定资料绑定任务失败"))
	}
	if taskNo != identity.TaskNo || subjectType != req.SubjectType || subjectID != req.SubjectID {
		return fail(agents.ErrConflict)
	}
	// 已绑定重放只复核原身份，不要求任务仍停留在首次预留步骤。
	old, err := readMaterialBinding(ctx, t.tx, identity.TaskID)
	if err == nil {
		if old.record.ActorID != actorID || old.record.Request != req || old.record.ProfileDigest != profile.Digest || old.contentDigest != pack.ContentDigest() {
			return fail(agents.ErrConflict)
		}
		// 同摘要配置的限额仍须与持久登记相同；只读核对，不再次预留。
		var limit, used int
		if err := t.tx.QueryRowContext(ctx, `SELECT max_submissions,reserved_submissions FROM tm_ai_agent_profiles WHERE profile_digest=? FOR UPDATE`, profile.Digest).Scan(&limit, &used); err != nil {
			return fail(fmt.Errorf("核对Agent资料绑定配置失败"))
		}
		if limit != profile.MaxSubmissions || used < 0 || used > limit {
			return fail(agents.ErrConflict)
		}
		return old, false, nil
	}
	if !errors.Is(err, agents.ErrNotFound) {
		return fail(err)
	}
	if !eligible {
		return fail(task.ErrLease)
	}
	record, started, err := t.Reserve(ctx, profile, req, actorID)
	if err != nil {
		return fail(err)
	}
	// 旧reference记录没有原manifest证明，不能用今日资料补造历史。
	if !started {
		return fail(agents.ErrConflict)
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO tm_ai_agent_material_bindings(task_id,user_id,run_id,materials_digest,content_digest,manifest,created_at,updated_at) VALUES(?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, identity.TaskID, identity.UserID, record.ID, pack.MaterialsDigest(), pack.ContentDigest(), raw)
	if err != nil {
		return fail(fmt.Errorf("写入Agent资料绑定失败"))
	}
	return AgentMaterialBinding{taskID: identity.TaskID, userID: identity.UserID, taskNo: taskNo, record: record, materialsDigest: pack.MaterialsDigest(), contentDigest: pack.ContentDigest(), manifest: raw}, true, nil
}

type materialBindingQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readMaterialBinding(ctx context.Context, db materialBindingQuery, taskID int64) (AgentMaterialBinding, error) {
	var b AgentMaterialBinding
	var runID int64
	var subjectType string
	var subjectID int64
	err := db.QueryRowContext(ctx, `SELECT b.task_id,b.user_id,t.task_no,b.run_id,b.materials_digest,b.content_digest,b.manifest,t.subject_type,t.subject_id
 FROM tm_ai_agent_material_bindings b JOIN tm_ai_tasks t ON t.id=b.task_id AND t.user_id=b.user_id WHERE b.task_id=? AND OCTET_LENGTH(b.manifest) BETWEEN 2 AND 262144`, taskID).Scan(&b.taskID, &b.userID, &b.taskNo, &runID, &b.materialsDigest, &b.contentDigest, &b.manifest, &subjectType, &subjectID)
	if errors.Is(err, sql.ErrNoRows) {
		return b, agents.ErrNotFound
	}
	if err != nil {
		return AgentMaterialBinding{}, fmt.Errorf("读取Agent资料绑定失败")
	}
	b.record, err = scanAgentRun(db.QueryRowContext(ctx, agentSelect+` WHERE id=?`, runID))
	if err != nil {
		return AgentMaterialBinding{}, err
	}
	if b.record.Request.SubjectType != subjectType || b.record.Request.SubjectID != subjectID {
		return AgentMaterialBinding{}, agents.ErrInvalid
	}
	if _, err = b.Pack(); err != nil {
		return AgentMaterialBinding{}, agents.ErrInvalid
	}
	return b, nil
}

// MaterialBindingByRequest 只核内部操作人身份，不是会员读取授权；不注册HTTP能力。
// 不复核当前许可，也不返回裸manifest。调用方必须另核原任务会员与当前权限。
func (r *AIAgentRepo) MaterialBindingByRequest(ctx context.Context, actorID int64, requestID string) (AgentMaterialBinding, error) {
	if ctx == nil || r == nil || r.db == nil || actorID <= 0 || !agents.ValidRequestID(requestID) {
		return AgentMaterialBinding{}, agents.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return AgentMaterialBinding{}, err
	}
	var taskID int64
	err := r.db.QueryRowContext(ctx, `SELECT b.task_id FROM tm_ai_agent_material_bindings b JOIN tm_ai_agent_runs r ON r.id=b.run_id WHERE r.actor_id=? AND r.request_id=?`, actorID, requestID).Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentMaterialBinding{}, agents.ErrNotFound
	}
	if err != nil {
		return AgentMaterialBinding{}, fmt.Errorf("查询Agent资料绑定失败")
	}
	b, err := readMaterialBinding(ctx, r.db, taskID)
	if err != nil {
		return AgentMaterialBinding{}, err
	}
	if b.record.ActorID != actorID || b.record.Request.RequestID != requestID {
		return AgentMaterialBinding{}, agents.ErrInvalid
	}
	return b, nil
}
