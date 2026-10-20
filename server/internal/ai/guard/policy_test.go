package guard

import "testing"

func TestPolicyDigestBindsAlgorithmAndActualRules(t *testing.T) {
	got := PolicyDigest()
	if len(got) != 64 || got != digestRules(AlgorithmVersion, wordsTxt, patternsTxt) || got != PolicyDigest() {
		t.Fatal("确定性策略摘要不稳定")
	}
	for _, changed := range []string{
		digestRules(AlgorithmVersion+"-changed", wordsTxt, patternsTxt),
		digestRules(AlgorithmVersion, wordsTxt+"\n测试新增规则\n", patternsTxt),
		digestRules(AlgorithmVersion, wordsTxt, patternsTxt+"\n测试句式 = 另一测试表达式\n"),
	} {
		if changed == got {
			t.Fatal("版本或实际规则内容改变未影响摘要")
		}
	}
	if digestRules("a", "bc", "d") == digestRules("ab", "c", "d") {
		t.Fatal("摘要输入拼接歧义")
	}
}
