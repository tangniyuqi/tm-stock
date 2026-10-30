package qa

import "testing"

func TestPolicyDigestMatchesExecutionWithoutMutatingDomains(t *testing.T) {
	_, plan, _, _ := fixture(t)
	c := plan.cfg
	c.AllowedDomains = []string{"z.example.test", "example.test"}
	digest, err := c.PolicyDigest()
	if err != nil || digest == "" || c.AllowedDomains[0] != "z.example.test" {
		t.Fatal("摘要变更了原配置")
	}
	p, err := NewPlan(plan.identity, plan.input, plan.scope, c, plan.scope.ValidUntil.Add(-1e9))
	if err != nil || p.Proof().PolicyDigest != digest {
		t.Fatal("登记与执行策略摘要不一致", err)
	}
	c.Selector.ProfileDigest = "invalid"
	if _, err := c.PolicyDigest(); err == nil {
		t.Fatal("未校验模型身份")
	}
}
