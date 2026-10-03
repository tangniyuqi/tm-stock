package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
)

// 注册表加载失败的原因。LoadRegistry 返回的错误用 %w 包着它们，可以用 errors.Is 判断。
var (
	// ErrDuplicateModelID 表示 id 重复。
	ErrDuplicateModelID = errors.New("模型 id 重复")
	// ErrUnknownCap 表示 caps 里出现了四种之外的能力名。
	ErrUnknownCap = errors.New("未知的能力名")
	// ErrMissingFilingNo 表示 enabled_for_c 为 true 却没有备案号（AC-L4）。
	ErrMissingFilingNo = errors.New("启用于 C 端的模型缺少备案号")
	// ErrInvalidModel 表示条目的其他字段不合法（空 id、温度范围反了等）。
	ErrInvalidModel = errors.New("模型条目不合法")
)

// maxRegistryBytes 是注册表文件的大小上限，防止误传超大输入。
const maxRegistryBytes = 1 << 20

// Model 是注册表里的一个模型条目，字段与 JSON 配置一一对应。
type Model struct {
	// ID 是注册表内唯一的标识，也是 Lookup 的键。
	ID     string `json:"id"`
	Vendor string `json:"vendor"`
	// Name 是对外公示的模型名。
	Name string `json:"name"`
	// FilingNo 是备案号或上线编号；页面公示的模型名与备案号取自注册表。
	FilingNo string `json:"filing_no"`
	// EnabledForC 表示该模型是否允许用于 C 端。为 true 时 FilingNo 必须非空。
	EnabledForC bool `json:"enabled_for_c"`
	// CapList 是能力名列表（json_object、json_schema、tools、stream）；用 Caps 方法取位集合。
	CapList []string `json:"caps"`
	// TempMin 与 TempMax 是 temperature 的取值范围（有的厂商不支持 0）；
	// 两者都为 0 表示注册表没有登记范围。
	TempMin float64 `json:"temp_min"`
	TempMax float64 `json:"temp_max"`
}

// Caps 返回 CapList 对应的能力位集合；不认识的能力名被忽略（LoadRegistry 已拒绝它们）。
func (m Model) Caps() Caps {
	var c Caps
	for _, name := range m.CapList {
		c |= capNames[name]
	}
	return c
}

func (m Model) clone() Model {
	m.CapList = slices.Clone(m.CapList)
	return m
}

// Registry 是加载后只读的模型注册表，可被多个 goroutine 并发使用。
type Registry struct {
	models map[string]Model
}

// LoadRegistry 从 JSON 加载注册表。顶层既可以是条目数组，也可以是 {"models": [...]}。
//
// 校验规则：未知字段一律拒绝；id 非空、首尾无空白且唯一；caps 只能取 json_object、json_schema、
// tools、stream；enabled_for_c 为 true 时 filing_no 去空白后必须非空，否则加载失败（AC-L4）——
// “空白”包含全角空格与零宽字符，避免用不可见字符绕过；温度范围若登记则须 0 ≤ temp_min ≤ temp_max。
// 只校验备案号非空，不校验它的格式与真伪，那需要法务提供口径。
func LoadRegistry(r io.Reader) (*Registry, error) {
	if r == nil {
		return nil, errors.New("模型注册表的输入为空")
	}
	data, err := io.ReadAll(io.LimitReader(r, maxRegistryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取模型注册表失败: %w", err)
	}
	if len(data) > maxRegistryBytes {
		return nil, errors.New("模型注册表超过 1 MiB 上限")
	}
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})) // 容忍带 BOM 的文件
	if len(data) == 0 {
		return nil, errors.New("模型注册表为空")
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var models []Model
	switch data[0] {
	case '[':
		if err := dec.Decode(&models); err != nil {
			return nil, fmt.Errorf("解析模型注册表失败: %w", err)
		}
	case '{':
		var file struct {
			Models []Model `json:"models"`
		}
		if err := dec.Decode(&file); err != nil {
			return nil, fmt.Errorf("解析模型注册表失败: %w", err)
		}
		models = file.Models
	default:
		return nil, errors.New("模型注册表必须是条目数组，或带 models 数组的对象")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("模型注册表在 JSON 之后还有多余内容")
	}

	reg := &Registry{models: make(map[string]Model, len(models))}
	for i, m := range models {
		if err := validateModel(m, i); err != nil {
			return nil, err
		}
		if _, dup := reg.models[m.ID]; dup {
			return nil, fmt.Errorf("模型 %q: %w", m.ID, ErrDuplicateModelID)
		}
		reg.models[m.ID] = m.clone()
	}
	return reg, nil
}

func validateModel(m Model, index int) error {
	if isBlank(m.ID) {
		return fmt.Errorf("第 %d 个模型的 id 为空: %w", index+1, ErrInvalidModel)
	}
	if m.ID != strings.TrimSpace(m.ID) {
		return fmt.Errorf("模型 %q 的 id 首尾不得有空白: %w", m.ID, ErrInvalidModel)
	}
	for _, name := range m.CapList {
		if _, ok := capNames[name]; !ok {
			return fmt.Errorf("模型 %q 的 caps 含未知能力 %q（只允许 json_object、json_schema、tools、stream）: %w",
				m.ID, name, ErrUnknownCap)
		}
	}
	if m.EnabledForC && isBlank(m.FilingNo) {
		return fmt.Errorf("模型 %q 的 enabled_for_c 为 true 但 filing_no 为空（AC-L4）: %w", m.ID, ErrMissingFilingNo)
	}
	if m.TempMin != 0 || m.TempMax != 0 {
		if m.TempMin < 0 || m.TempMax < m.TempMin {
			return fmt.Errorf("模型 %q 的温度范围不合法（须 0 ≤ temp_min ≤ temp_max）: %w", m.ID, ErrInvalidModel)
		}
	}
	return nil
}

// isBlank 判断 s 去掉空白字符与不可见格式字符（零宽空格、BOM 等）后是否为空。
func isBlank(s string) bool {
	return strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r)
	}) == ""
}

// Lookup 按 id 取条目。返回的是副本，修改它不会影响注册表。
func (r *Registry) Lookup(id string) (Model, bool) {
	if r == nil {
		return Model{}, false
	}
	m, ok := r.models[id]
	if !ok {
		return Model{}, false
	}
	return m.clone(), true
}

// ForC 返回启用于 C 端的模型，按 id 升序。每次返回新切片，没有时返回空切片。
func (r *Registry) ForC() []Model {
	out := []Model{}
	if r == nil {
		return out
	}
	for _, m := range r.models {
		if m.EnabledForC {
			out = append(out, m.clone())
		}
	}
	slices.SortFunc(out, func(a, b Model) int { return strings.Compare(a.ID, b.ID) })
	return out
}
