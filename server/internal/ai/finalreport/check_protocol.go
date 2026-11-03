package finalreport

import (
	"fmt"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// PairedCheckProtocol只收紧模型请求约束，响应判定仍沿原闭集规则。
const PairedCheckProtocol = "paired-report-check-v2"

func ValidCheckProtocol(value string) bool { return value == "" || value == PairedCheckProtocol }

const pairedCheckSchema = `{"type":"object","additionalProperties":false,"properties":{"verdict":{"type":"string","enum":["pass","block","uncertain"]},"reason":{"type":"string","enum":["%[1]s","%[2]s","%[3]s"]}},"required":["verdict","reason"],"anyOf":[{"properties":{"verdict":{"enum":["pass"]},"reason":{"enum":["%[1]s"]}}},{"properties":{"verdict":{"enum":["block"]},"reason":{"enum":["%[2]s"]}}},{"properties":{"verdict":{"enum":["uncertain"]},"reason":{"enum":["%[3]s"]}}}]}`

func (c CheckConfig) protocolVersion() string {
	if c.CheckProtocol == "" {
		return CheckVersion
	}
	if c.CheckProtocol == PairedCheckProtocol {
		return PairedCheckProtocol
	}
	return ""
}

// 旧协议的文本及Schema字节保持原样，历史任务不自动切换新提示。
func checkContract(kind CheckKind, c CheckConfig) (version, rule, schema string, err error) {
	if kind != CheckEntailment && kind != CheckSemantic || c.protocolVersion() == "" {
		return "", "", "", points.ErrInvalid
	}
	version, rule, schema = c.protocolVersion(), entailmentRule, checkSchema
	if kind == CheckSemantic {
		rule = semanticRule
	}
	if c.CheckProtocol == PairedCheckProtocol {
		pass, block, uncertain := "supported", "unsupported", "insufficient"
		if kind == CheckSemantic {
			pass, block, uncertain = "factual", "prohibited", "uncertain"
		}
		schema = fmt.Sprintf(pairedCheckSchema, pass, block, uncertain)
		rule += fmt.Sprintf("\nverdict和reason必须严格配对：pass/%s、block/%s、uncertain/%s。只返回其中一组JSON对象；不得混用原因值或增加解释字段。", pass, block, uncertain)
	}
	return version, rule, schema, nil
}
