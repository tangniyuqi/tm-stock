package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

func referenceRequest() Request {
	return Request{RequestID: "reference_request_01", MemberID: 1, SubjectType: "theme", SubjectID: 1001, MaterialsDigest: strings.Repeat("a", 64)}
}

func TestReferenceRequestPreservesLegacyEncoding(t *testing.T) {
	old := Request{RequestID: "legacy_request_01", Symbol: "600001"}
	raw, err := json.Marshal(old)
	if err != nil || string(raw) != `{"request_id":"legacy_request_01","symbol":"600001"}` || !old.Valid() || old.IsReference() {
		t.Fatal("原股票请求编码或语义改变")
	}
	var decoded Request
	if json.Unmarshal(raw, &decoded) != nil || decoded != old {
		t.Fatal("原股票请求无法原样重读")
	}
	for _, kind := range []string{"theme", "topic"} {
		t.Run(kind, func(t *testing.T) {
			req := referenceRequest()
			req.SubjectType = kind
			raw, err := json.Marshal(req)
			if err != nil || !req.Valid() || !req.IsReference() || json.Unmarshal(raw, &decoded) != nil || decoded != req {
				t.Fatal("显式资料对象无法重读")
			}
		})
	}
}

func TestReferenceRequestRejectsMixedOrMissingScope(t *testing.T) {
	for name, change := range map[string]func(*Request){
		"股票代码混入": func(r *Request) { r.Symbol = "600001" },
		"空会员":    func(r *Request) { r.MemberID = 0 },
		"负会员":    func(r *Request) { r.MemberID = -1 },
		"越界会员":   func(r *Request) { r.MemberID = 9007199254740992 },
		"空对象":    func(r *Request) { r.SubjectID = 0 },
		"越界对象":   func(r *Request) { r.SubjectID = 9007199254740992 },
		"未知对象类别": func(r *Request) { r.SubjectType = "stock" },
		"空对象类别":  func(r *Request) { r.SubjectType = "" },
		"空资料摘要":  func(r *Request) { r.MaterialsDigest = "" },
		"摘要别名":   func(r *Request) { r.MaterialsDigest = strings.Repeat("A", 64) },
		"空原键":    func(r *Request) { r.RequestID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			req := referenceRequest()
			change(&req)
			if req.Valid() {
				t.Fatal("不完整或混合请求通过")
			}
		})
	}
}

func TestReferenceRequestJSONRejectsAmbiguousFields(t *testing.T) {
	raw, err := json.Marshal(referenceRequest())
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(string) string{
		"缺空股票字段": func(s string) string { return strings.Replace(s, `"symbol":"",`, "", 1) },
		"空值股票":   func(s string) string { return strings.Replace(s, `"symbol":""`, `"symbol":null`, 1) },
		"空值会员":   func(s string) string { return strings.Replace(s, `"member_id":1`, `"member_id":null`, 1) },
		"字符串编号":  func(s string) string { return strings.Replace(s, `"subject_id":1001`, `"subject_id":"1001"`, 1) },
		"小数编号":   func(s string) string { return strings.Replace(s, `"subject_id":1001`, `"subject_id":1001.0`, 1) },
		"重复字段":   func(s string) string { return strings.Replace(s, `"symbol":""`, `"symbol":"","symbol":""`, 1) },
		"额外地址":   func(s string) string { return strings.TrimSuffix(s, "}") + `,"url":"https://example.test"}` },
		"未知别名":   func(s string) string { return strings.Replace(s, `"subject_id"`, `"subjectId"`, 1) },
		"隐式空范围": func(string) string {
			return `{"request_id":"legacy_request_01","symbol":"600001","member_id":0,"subject_type":"","subject_id":0,"materials_digest":""}`
		},
	} {
		t.Run(name, func(t *testing.T) {
			var req Request
			if json.Unmarshal([]byte(change(string(raw))), &req) == nil {
				t.Fatal("歧义资料请求被解码")
			}
		})
	}
}
