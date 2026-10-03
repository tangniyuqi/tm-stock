package provider

import "strings"

// Caps 是模型或路由的能力位集合。能力必须表驱动，不得假设各厂商一致（设计 5.4）。
type Caps uint8

const (
	// CapJSONObject 支持 response_format = json_object。
	CapJSONObject Caps = 1 << iota
	// CapJSONSchema 支持 response_format = json_schema。
	CapJSONSchema
	// CapTools 支持工具调用。
	CapTools
	// CapStream 支持流式输出（一期不实现流式，仅作能力登记）。
	CapStream
)

// capNames 是能力位与注册表里 caps 字段字面量的对应关系。
var capNames = map[string]Caps{
	"json_object": CapJSONObject,
	"json_schema": CapJSONSchema,
	"tools":       CapTools,
	"stream":      CapStream,
}

// capOrder 固定 String 的输出顺序。
var capOrder = []struct {
	name string
	bit  Caps
}{
	{"json_object", CapJSONObject},
	{"json_schema", CapJSONSchema},
	{"tools", CapTools},
	{"stream", CapStream},
}

// Has 判断 c 是否包含 want 的全部能力位。want 为 0 时恒为 true。
func (c Caps) Has(want Caps) bool {
	return c&want == want
}

// String 以竖线连接已有能力名，空集合返回 "none"。
func (c Caps) String() string {
	var names []string
	for _, item := range capOrder {
		if c.Has(item.bit) {
			names = append(names, item.name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, "|")
}

// requiredCaps 返回请求本身需要的能力：要 json_schema 就必须有 CapJSONSchema，
// 要 json_object 就必须有 CapJSONObject，普通文本不需要任何能力。
func requiredCaps(req Request) Caps {
	switch req.Format {
	case FormatJSONObject:
		return CapJSONObject
	case FormatJSONSchema:
		return CapJSONSchema
	default:
		return 0
	}
}
