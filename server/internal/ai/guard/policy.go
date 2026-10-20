package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// AlgorithmVersion 绑定规范化、切句、共现与抽取校验的确定性语义。
// 实现语义改变必须递增；词表/句式文件内容另外自动参与摘要，不依赖手工改版本。
const AlgorithmVersion = "deterministic-extraction-v1"

func PolicyDigest() string { return digestRules(AlgorithmVersion, wordsTxt, patternsTxt) }

func digestRules(version, words, patterns string) string {
	// 长度前缀避免三份内容在拼接边界上产生歧义。
	material := fmt.Sprintf("%d:%s%d:%s%d:%s", len(version), version, len(words), words, len(patterns), patterns)
	h := sha256.Sum256([]byte(material))
	return hex.EncodeToString(h[:])
}
