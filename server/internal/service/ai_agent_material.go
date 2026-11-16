package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/materialpack"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

// agentMaterialStore 仅暴露当前资料读取，不授予模型调用或积分操作能力。
type agentMaterialStore interface {
	Load(context.Context, repository.AgentMaterialRequest) (repository.AgentMaterialScope, error)
}

// AIAgentMaterialService 只准备和复核内存资料包，不持久化、不装配执行器。
// 批准固定使用真实文件来源，不能以可注入接口绕过独立用途检查。
type AIAgentMaterialService struct {
	source   *config.AILocalAgentMaterialSource
	identity localInstanceCheck
	store    agentMaterialStore
}

func NewAIAgentMaterialService(source *config.AILocalAgentMaterialSource, identity localInstanceCheck, store agentMaterialStore) (*AIAgentMaterialService, error) {
	if source == nil || agentMaterialNil(identity) || agentMaterialNil(store) {
		return nil, agents.ErrInvalid
	}
	return &AIAgentMaterialService{source: source, identity: identity, store: store}, nil
}

func (s *AIAgentMaterialService) current(ctx context.Context) (*config.AILocalAgentMaterial, error) {
	if s == nil || s.source == nil || agentMaterialNil(s.identity) || agentMaterialNil(s.store) || ctx == nil {
		return nil, agents.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	approval, err := s.source.Current(ctx)
	if err != nil {
		return nil, agentMaterialFailure(ctx)
	}
	if err := s.identity.Verify(ctx, approval.InstanceID); err != nil {
		return nil, agentMaterialFailure(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return approval, nil
}

// Prepare 的身份参数只能来自服务器鉴权和对象选择；不接收正文、URL、配置或许可声明。
// topic 尚无可信目录读取链路，在服务入口失败关闭，不能依赖注入仓储自行拒绝。
func (s *AIAgentMaterialService) Prepare(ctx context.Context, memberID int64, subjectType string, subjectID int64) (*materialpack.Pack, error) {
	approval, err := s.current(ctx)
	if err != nil {
		return nil, err
	}
	binding, ok := approval.Binding(memberID, subjectType, subjectID)
	if !ok || subjectType != "theme" {
		return nil, agents.ErrInvalid
	}
	before, err := agentMaterialApprovalDigest(approval)
	if err != nil {
		return nil, agents.ErrInvalid
	}
	evidence := make([]repository.AgentMaterialEvidence, len(binding.Evidence))
	for i, e := range binding.Evidence {
		evidence[i] = repository.AgentMaterialEvidence{SnapshotID: e.SnapshotID, Hash: e.Hash}
	}
	loaded, err := s.store.Load(ctx, repository.AgentMaterialRequest{
		MemberID: memberID, SubjectType: subjectType, SubjectID: subjectID, ValidUntil: time.UnixMilli(approval.ExpiresAtMS),
		Evidence: evidence, MaxSources: approval.Limits.MaxSources, MaxEvidenceBytes: approval.Limits.MaxEvidenceBytes,
	})
	if err != nil {
		return nil, agentMaterialFailure(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// 固定批准时窗参与摘要；数据库当前时钟只用于校验，不写入包的身份。
	pack, err := materialpack.New(materialpack.Scope{MemberID: memberID, SubjectType: subjectType, SubjectID: subjectID,
		SubjectName: loaded.SubjectName, ScopeApproval: binding.ScopeApproval, IssuedAtMS: approval.IssuedAtMS, ExpiresAtMS: approval.ExpiresAtMS},
		*approval, loaded.Sources, loaded.Snapshots, loaded.Now)
	if err != nil {
		return nil, agentMaterialFailure(ctx)
	}
	// 装配结束再次重读批准及实例，检测期间用途文件撤回、改写或实例失效。
	// 这不是第二次数据库权限读取，也不保证数据库撤权与后续发送之间的原子性。
	afterApproval, err := s.current(ctx)
	if err != nil {
		return nil, err
	}
	after, err := agentMaterialApprovalDigest(afterApproval)
	if err != nil || before != after {
		return nil, agents.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return pack, nil
}

// Recheck 只核验服务器持有的原包，不替代当前调用者鉴权，也不得用作跨会员读取授权。
// 未来入口必须先核验原任务、当前会员与原包 Scope 一致。
// 不返回替代包；任一身份或内容变化都要求调用方停止后续发送。
// 成功不是长期授权：每次发送前仍需重新调用，且这里不执行模型、积分或数据库写入。
func (s *AIAgentMaterialService) Recheck(ctx context.Context, original *materialpack.Pack) error {
	if original == nil || !agents.ValidDigest(original.MaterialsDigest()) || !agents.ValidDigest(original.ContentDigest()) {
		return agents.ErrInvalid
	}
	scope := original.Scope()
	current, err := s.Prepare(ctx, scope.MemberID, scope.SubjectType, scope.SubjectID)
	if err != nil {
		return err
	}
	if current.MaterialsDigest() != original.MaterialsDigest() || current.ContentDigest() != original.ContentDigest() {
		return agents.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// 接口可能装有类型化空指针；在调用依赖前拒绝，避免配置错误引发恐慌。
func agentMaterialNil(v any) bool {
	if v == nil {
		return true
	}
	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	}
	return false
}

func agentMaterialApprovalDigest(approval *config.AILocalAgentMaterial) ([32]byte, error) {
	raw, err := json.Marshal(approval)
	if err != nil {
		return [32]byte{}, agents.ErrInvalid
	}
	return sha256.Sum256(raw), nil
}

// 不包装仓储、实例或批准读取原错，避免把正文、URL和内部细节带出服务边界。
func agentMaterialFailure(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return agents.ErrInvalid
}
