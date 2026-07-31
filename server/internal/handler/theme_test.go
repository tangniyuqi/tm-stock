package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type fakeSvc struct {
	detail      *dto.ThemeDetailResp
	detailErr   error
	evidence    *dto.EvidenceResp
	evidenceErr error

	gotThemeID int64
	gotTsCode  string
	gotAccess  service.Access
}

func (f *fakeSvc) GetDetail(_ context.Context, id int64, a service.Access) (*dto.ThemeDetailResp, error) {
	f.gotThemeID, f.gotAccess = id, a
	return f.detail, f.detailErr
}
func (f *fakeSvc) GetEvidence(_ context.Context, id int64, code string, a service.Access) (*dto.EvidenceResp, error) {
	f.gotThemeID, f.gotTsCode, f.gotAccess = id, code, a
	return f.evidence, f.evidenceErr
}

func do(h *ThemeHandler, method, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\n%s", err, rec.Body.String())
	}
	return m
}

// ── 默认必须 fail-closed ──────────────────────────────────────────────────

// 会员系统尚未实现。默认放行会让付费内容在会员上线前整体裸奔，
// 这条测试就是防止有人把默认值改成"放行"。
func TestDefaultAccessIsDenied(t *testing.T) {
	f := &fakeSvc{detail: &dto.ThemeDetailResp{ID: 1, Name: "光刻机", Locked: true}}
	h := NewThemeHandler(f, DenyAllAccess{})

	rec := do(h, http.MethodGet, "/api/v1/theme/100001")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d", rec.Code)
	}
	if f.gotAccess.CanViewPaid {
		t.Error("默认解析器必须拒绝付费内容（fail-closed）")
	}
}

func TestDevGrantAccessOnlyWhenExplicit(t *testing.T) {
	f := &fakeSvc{detail: &dto.ThemeDetailResp{ID: 1}}
	h := NewThemeHandler(f, NewDevGrantAllAccess())
	do(h, http.MethodGet, "/api/v1/theme/100001")
	if !f.gotAccess.CanViewPaid {
		t.Error("显式启用开发放行时应可见付费内容")
	}
}

// ── 参数校验 ─────────────────────────────────────────────────────────────

func TestDetail_RejectsBadID(t *testing.T) {
	for _, bad := range []string{"0", "-1", "abc", "1.5"} {
		f := &fakeSvc{detail: &dto.ThemeDetailResp{}}
		h := NewThemeHandler(f, DenyAllAccess{})
		rec := do(h, http.MethodGet, "/api/v1/theme/"+bad)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id=%q 应返回 400，得到 %d", bad, rec.Code)
		}
		if f.gotThemeID != 0 {
			t.Errorf("id=%q 非法时不应调用 service", bad)
		}
	}
}

func TestEvidence_RejectsBadTsCode(t *testing.T) {
	for _, bad := range []string{"688502", "abc.sh", "688502.SH'", "x", strings.Repeat("A", 25) + ".SH"} {
		f := &fakeSvc{evidence: &dto.EvidenceResp{}}
		h := NewThemeHandler(f, DenyAllAccess{})
		rec := do(h, http.MethodGet, "/api/v1/theme/1/stock/"+bad+"/evidence")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("tsCode=%q 应返回 400，得到 %d", bad, rec.Code)
		}
	}
}

func TestEvidence_AcceptsValidTsCode(t *testing.T) {
	f := &fakeSvc{evidence: &dto.EvidenceResp{TsCode: "688502.SH", SourceType: "年报"}}
	h := NewThemeHandler(f, NewDevGrantAllAccess())
	rec := do(h, http.MethodGet, "/api/v1/theme/100001/stock/688502.SH/evidence")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d：%s", rec.Code, rec.Body.String())
	}
	if f.gotTsCode != "688502.SH" || f.gotThemeID != 100001 {
		t.Errorf("参数未正确传递: id=%d code=%q", f.gotThemeID, f.gotTsCode)
	}
}

// ── 错误映射 ─────────────────────────────────────────────────────────────

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantHTTP int
		wantCode float64
	}{
		{"题材不存在", service.ErrThemeNotFound, http.StatusNotFound, 40401},
		{"依据不存在", service.ErrEvidenceNotFound, http.StatusNotFound, 40402},
		{"无权限", service.ErrAccessDenied, http.StatusForbidden, 40301},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeSvc{detailErr: c.err}
			h := NewThemeHandler(f, DenyAllAccess{})
			rec := do(h, http.MethodGet, "/api/v1/theme/1")
			if rec.Code != c.wantHTTP {
				t.Errorf("HTTP 状态期望 %d，得到 %d", c.wantHTTP, rec.Code)
			}
			if got := decode(t, rec)["code"]; got != c.wantCode {
				t.Errorf("业务码期望 %v，得到 %v", c.wantCode, got)
			}
		})
	}
}

// 无权限时必须是【引导订阅】，不能说成"没有依据"——
// 后者会让用户以为数据缺失，是错误的信息。
func TestAccessDeniedMessageGuidesToSubscribe(t *testing.T) {
	f := &fakeSvc{evidenceErr: service.ErrAccessDenied}
	h := NewThemeHandler(f, DenyAllAccess{})
	rec := do(h, http.MethodGet, "/api/v1/theme/1/stock/688502.SH/evidence")
	msg, _ := decode(t, rec)["msg"].(string)
	if !strings.Contains(msg, "订阅") {
		t.Errorf("403 文案应引导订阅，实际 %q", msg)
	}
	if strings.Contains(msg, "无依据") || strings.Contains(msg, "不存在") {
		t.Errorf("403 不得暗示数据缺失，实际 %q", msg)
	}
}

// 内部错误不得把表名/SQL/连接串等细节回给客户端。
func TestInternalErrorDoesNotLeakDetails(t *testing.T) {
	f := &fakeSvc{detailErr: errFake("查归属映射失败: Error 1146: Table 'go_noooya_com.addon_quant_theme_stock' doesn't exist")}
	h := NewThemeHandler(f, DenyAllAccess{})
	rec := do(h, http.MethodGet, "/api/v1/theme/1")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("期望 500，得到 %d", rec.Code)
	}
	body := rec.Body.String()
	for _, leak := range []string{"addon_quant", "go_noooya_com", "Table", "1146"} {
		if strings.Contains(body, leak) {
			t.Errorf("响应体泄漏了内部细节 %q：%s", leak, body)
		}
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

// ── 响应格式 ─────────────────────────────────────────────────────────────

func TestSuccessEnvelope(t *testing.T) {
	f := &fakeSvc{detail: &dto.ThemeDetailResp{ID: 100001, Name: "光刻机", UpdatedAt: 1753900000000}}
	h := NewThemeHandler(f, DenyAllAccess{})
	rec := do(h, http.MethodGet, "/api/v1/theme/100001")

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type 应为 JSON，得到 %q", ct)
	}
	m := decode(t, rec)
	if m["code"] != float64(0) {
		t.Errorf("成功时 code 应为 0，得到 %v", m["code"])
	}
	data, _ := m["data"].(map[string]any)
	if data["name"] != "光刻机" {
		t.Errorf("data.name 错误: %v", data["name"])
	}
	// 时间字段必须是数字（毫秒时间戳），不能是字符串
	if _, isNum := data["updatedAt"].(float64); !isNum {
		t.Errorf("updatedAt 必须是毫秒时间戳数字，实际 %T", data["updatedAt"])
	}
}

// 方法不匹配应 405，不能当成 404 或误命中其他路由。
func TestWrongMethodNotRouted(t *testing.T) {
	f := &fakeSvc{detail: &dto.ThemeDetailResp{}}
	h := NewThemeHandler(f, DenyAllAccess{})
	mux := http.NewServeMux()
	h.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/theme/1", nil))
	if rec.Code == http.StatusOK {
		t.Error("POST 不应命中只注册了 GET 的路由")
	}
	if f.gotThemeID != 0 {
		t.Error("方法不匹配时不应调用 service")
	}
}
