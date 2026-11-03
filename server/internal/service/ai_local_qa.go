package service

import (
	"context"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// 只装配原问答策略/模型接口；范围、配额、调用存证继续由原问答核控制。
type AILocalQAControl struct {
	source   *config.AILocalQASource
	identity localInstanceCheck
	submit   bool
}

func NewAILocalQAControl(source *config.AILocalQASource, identity localInstanceCheck, submit bool) (*AILocalQAControl, error) {
	if source == nil || identity == nil {
		return nil, qa.ErrInvalid
	}
	return &AILocalQAControl{source, identity, submit}, nil
}
func (c *AILocalQAControl) current(ctx context.Context) (*config.AILocalQA, error) {
	if c == nil || ctx == nil {
		return nil, qa.ErrInvalid
	}
	p, err := c.source.Current(ctx)
	if err != nil {
		return nil, qa.ErrUnavailable
	}
	if err := c.identity.Verify(ctx, p.InstanceID); err != nil {
		return nil, qa.ErrUnavailable
	}
	return p, nil
}
func (c *AILocalQAControl) Ready(ctx context.Context) error { _, err := c.current(ctx); return err }
func (c *AILocalQAControl) InstanceID(ctx context.Context) (string, error) {
	p, err := c.current(ctx)
	if err != nil {
		return "", err
	}
	return p.InstanceID, nil
}
func (c *AILocalQAControl) ForPolicy(ctx context.Context, digest string) (qa.Config, error) {
	p, err := c.current(ctx)
	if err != nil {
		return qa.Config{}, err
	}
	v, err := p.QAConfig()
	if err != nil {
		return qa.Config{}, qa.ErrUnavailable
	}
	expected, err := v.PolicyDigest()
	if err != nil || expected != digest {
		return qa.Config{}, qa.ErrUnavailable
	}
	return v, nil
}
func (c *AILocalQAControl) ForReport(ctx context.Context, evidence qa.ReportEvidence) (qa.Config, error) {
	p, err := c.current(ctx)
	if err != nil {
		return qa.Config{}, err
	}
	if p.Version != 1 || evidence.TaskID <= 0 || evidence.SubjectType != "theme" || !slices.Contains(p.Subjects, evidence.SubjectID) {
		return qa.Config{}, qa.ErrUnavailable
	}
	return p.QAConfig()
}
func (c *AILocalQAControl) ForPlan(ctx context.Context, proof qa.Proof) ([3]*provider.OpenAICompat, error) {
	var models [3]*provider.OpenAICompat
	if c == nil || !c.submit {
		return models, qa.ErrUnavailable
	}
	p, err := c.current(ctx)
	if err != nil {
		return models, err
	}
	if !proof.Identity.Valid() || !slices.Contains(p.Members, proof.Identity.UserID) || proof.SubjectType != "theme" || !slices.Contains(p.Subjects, proof.SubjectID) || len(proof.Models) != 3 || proof.MaxCalls != 3 || proof.MaxTokens != p.Limits.MaxTotalTokens || !proof.ValidUntil.After(time.Now()) || proof.ValidUntil.UnixMilli() > p.ExpiresAtMS {
		return models, qa.ErrUnavailable
	}
	if err := localQAMaterialProof(p, proof); err != nil {
		return models, err
	}
	for n, m := range p.Models {
		if proof.Models[n] != m.Metadata() {
			return models, qa.ErrUnavailable
		}
		models[n], err = provider.NewDirect(m.ProviderConfig(), true)
		if err != nil || !models[n].MatchesDirect(m.ProfileDigest, m.Endpoint.Model) {
			return [3]*provider.OpenAICompat{}, qa.ErrUnavailable
		}
	}
	return models, nil
}

// 读取不依赖模型实例；正式资料仍须复核原快照和开发用途，不能只匹配模型策略摘要。
func (c *AILocalQAControl) ValidateProof(ctx context.Context, proof qa.Proof) error {
	p, err := c.current(ctx)
	if err != nil {
		return err
	}
	return localQAMaterialProof(p, proof)
}
func localQAMaterialProof(p *config.AILocalQA, proof qa.Proof) error {
	if p.Version == 1 {
		return nil
	}
	if p.MaterialUse == nil || !proof.Identity.Valid() || !slices.Contains(p.Members, proof.Identity.UserID) || proof.SubjectType != "theme" || !proof.ValidUntil.After(time.Now()) || proof.ValidUntil.UnixMilli() > p.ExpiresAtMS {
		return qa.ErrUnavailable
	}
	for _, b := range p.MaterialUse.Bindings {
		if b.SubjectID == proof.SubjectID && b.ScopeApproval == proof.ScopeApproval && slices.Equal(b.Evidence, proof.Evidence) {
			return nil
		}
	}
	return qa.ErrUnavailable
}
