package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func TestDossierCorrectionCLI(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	key := hex.EncodeToString(seed[:])
	r := dossier.CorrectionRequest{RequestID: "test_correction_cli_001", Object: dossier.CorrectionPair{ThemeID: 7, BeforeVersion: 1, AfterVersion: 2}, BeforeDigest: strings.Repeat("a", 64), AfterDigest: strings.Repeat("b", 64), Enabled: true, ValidUntil: 1793703600000, Declarations: []dossier.CorrectionDeclaration{{Before: dossier.CorrectionLocator{Key: "sample"}, After: dossier.CorrectionLocator{Key: "sample"}, StatementKey: "sample"}}, ReviewReference: "test-fixture:cli"}
	body, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := json.Marshal(r.Object)
	if err != nil {
		t.Fatal(err)
	}
	receipt := dto.AIDossierCorrectionReceipt{RequestID: r.RequestID, Object: r.Object, ActorID: 7, Revision: 1, Enabled: true, RecordedAt: 1793700000000}
	inspect := dto.AIDossierCorrectionInspect{Notice: dto.AIDossierManagementNotice, Object: r.Object, BeforeDigest: r.BeforeDigest, AfterDigest: r.AfterDigest, SourceState: "unavailable"}
	for _, tc := range []struct {
		name, command, input string
		response             any
		edit                 func(map[string]any)
		status               int
		valid                bool
	}{
		{"apply", "correction-apply", string(body), receipt, nil, 200, true}, {"inspect", "correction-inspect", string(pair), inspect, nil, 200, true}, {"request", "correction-request", `{"requestId":"test_correction_cli_001"}`, receipt, nil, 200, true},
		{"missing_boolean", "correction-apply", string(body), receipt, func(v map[string]any) { delete(v, "enabled") }, 200, false}, {"null_boolean", "correction-apply", string(body), receipt, func(v map[string]any) { v["enabled"] = nil }, 200, false}, {"string_version", "correction-apply", string(body), receipt, func(v map[string]any) { v["object"].(map[string]any)["afterVersion"] = "2" }, 200, false}, {"extra_private", "correction-apply", string(body), receipt, func(v map[string]any) { v["reviewReference"] = "private" }, 200, false}, {"wrong_pair", "correction-apply", string(body), receipt, func(v map[string]any) { v["object"].(map[string]any)["afterVersion"] = 3 }, 200, false}, {"wrong_revision", "correction-apply", string(body), receipt, func(v map[string]any) { v["revision"] = 2 }, 200, false}, {"wrong_request", "correction-request", `{"requestId":"test_correction_cli_001"}`, receipt, func(v map[string]any) { v["requestId"] = "test_correction_other_002" }, 200, false}, {"free_notice", "correction-inspect", string(pair), inspect, func(v map[string]any) { v["notice"] = "free" }, 200, false}, {"bad_digest", "correction-inspect", string(pair), inspect, func(v map[string]any) { v["afterDigest"] = "bad" }, 200, false}, {"unknown_result", "correction-apply", string(body), receipt, nil, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Header.Get("Authorization") != "Bearer "+key {
					t.Error("凭据不在授权头")
				}
				raw, er := json.Marshal(tc.response)
				if er != nil {
					t.Error(er)
					return
				}
				var v map[string]any
				if json.Unmarshal(raw, &v) != nil {
					t.Error("夹具响应无效")
					return
				}
				if tc.edit != nil {
					tc.edit(v)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if er = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": v}); er != nil {
					t.Error(er)
				}
			}))
			defer server.Close()
			client, er := newReviewClient(server.URL, key)
			if er != nil {
				t.Fatal(er)
			}
			defer client.http.CloseIdleConnections()
			var output bytes.Buffer
			er = runCorrectionCommand(client, tc.command, strings.NewReader(tc.input), &output)
			if (er == nil) != tc.valid || calls != 1 || !tc.valid && output.Len() != 0 || strings.Contains(output.String(), key) || strings.Contains(output.String(), r.ReviewReference) {
				t.Fatal("更正CLI未封闭错误、重试或私有输出", er)
			}
		})
	}
	if err = runCorrectionCommand(nil, "correction-apply", nil, nil); err == nil {
		t.Fatal("缺失依赖被接受")
	}
}
