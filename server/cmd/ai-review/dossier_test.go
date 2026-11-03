package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func TestDossierManagementCLI(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	key := hex.EncodeToString(seed[:])
	request := dossier.ManagementRequest{RequestID: "test_management_cli_001", Object: dossier.ManagementObject{Kind: "publication", ThemeID: 7, Version: 1}, VersionDigest: strings.Repeat("a", 64), Enabled: true, ValidUntil: 1793703600000, ReviewReference: "test-fixture:cli"}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	target, err := json.Marshal(request.Object)
	if err != nil {
		t.Fatal(err)
	}
	receipt := dto.AIDossierManagementReceipt{RequestID: request.RequestID, Object: request.Object, ActorID: 7, Revision: 1, Enabled: true, RecordedAt: 1793700000000}
	inspect := dto.AIDossierManagementInspect{Notice: dto.AIDossierManagementNotice, Object: request.Object, VersionDigest: request.VersionDigest, SourceState: "unavailable"}
	for _, tc := range []struct {
		name, command, input string
		response             any
		mutate               func(map[string]any)
		status               int
		valid                bool
	}{
		{"apply", "dossier-apply", string(raw), receipt, nil, 200, true},
		{"request", "dossier-request", `{"requestId":"test_management_cli_001"}`, receipt, nil, 200, true},
		{"inspect", "dossier-inspect", string(target), inspect, nil, 200, true},
		{"missing_boolean", "dossier-apply", string(raw), receipt, func(v map[string]any) { delete(v, "enabled") }, 200, false},
		{"null_boolean", "dossier-apply", string(raw), receipt, func(v map[string]any) { v["enabled"] = nil }, 200, false},
		{"string_boolean", "dossier-apply", string(raw), receipt, func(v map[string]any) { v["enabled"] = "true" }, 200, false},
		{"null_zero_id", "dossier-apply", string(raw), receipt, func(v map[string]any) { v["object"].(map[string]any)["userId"] = nil }, 200, false},
		{"private_extra", "dossier-apply", string(raw), receipt, func(v map[string]any) { v["authorization"] = key }, 200, false},
		{"wrong_revision", "dossier-apply", string(raw), receipt, func(v map[string]any) { v["revision"] = 2 }, 200, false},
		{"wrong_request", "dossier-request", `{"requestId":"test_management_cli_001"}`, receipt, func(v map[string]any) { v["requestId"] = "test_management_other_002" }, 200, false},
		{"null_managed", "dossier-inspect", string(target), inspect, func(v map[string]any) { v["managed"] = nil }, 200, false},
		{"free_notice", "dossier-inspect", string(target), inspect, func(v map[string]any) { v["notice"] = "自由生成说明" }, 200, false},
		{"bad_digest", "dossier-inspect", string(target), inspect, func(v map[string]any) { v["versionDigest"] = strings.Repeat("z", 64) }, 200, false},
		{"known_failure", "dossier-apply", string(raw), receipt, nil, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer "+key {
					t.Error("凭据未只通过授权头传入")
				}
				body, e := io.ReadAll(r.Body)
				if e != nil {
					t.Error(e)
				}
				if bytes.Contains(body, []byte(key)) {
					t.Error("命令把凭据放进业务正文")
				}
				data, e := json.Marshal(tc.response)
				if e != nil {
					t.Error(e)
					return
				}
				var object map[string]any
				if e = json.Unmarshal(data, &object); e != nil {
					t.Error(e)
					return
				}
				if tc.mutate != nil {
					tc.mutate(object)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if e = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": object}); e != nil {
					t.Error(e)
				}
			}))
			defer server.Close()
			client, e := newReviewClient(server.URL, key)
			if e != nil {
				t.Fatal(e)
			}
			defer client.http.CloseIdleConnections()
			var output bytes.Buffer
			e = runDossierCommand(client, tc.command, strings.NewReader(tc.input), &output)
			if (e == nil) != tc.valid || calls != 1 || !tc.valid && output.Len() != 0 {
				t.Fatal("命令未保持单次传输/闭集回执", e, calls)
			}
			if strings.Contains(output.String(), key) || e != nil && strings.Contains(e.Error(), key) {
				t.Fatal("命令输出泄漏凭据")
			}
		})
	}
	t.Run("invalid_input_zero_requests", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
		defer server.Close()
		client, e := newReviewClient(server.URL, key)
		if e != nil {
			t.Fatal(e)
		}
		defer client.http.CloseIdleConnections()
		for _, value := range []struct{ command, body string }{
			{"dossier-apply", strings.Replace(string(raw), "\"enabled\":true", "\"enabled\":null", 1)},
			{"dossier-inspect", `{"kind":"publication","userId":null,"themeId":7,"version":1}`},
			{"dossier-inspect", `{"kind":"publication","userId":0,"themeId":7,"version":1,"actorId":8}`},
			{"dossier-request", `{"requestId":"short"}`},
			{"dossier-request", `{"requestId":"test_management_cli_001","requestId":"test_management_cli_001"}`},
			{"unknown", "{}"},
		} {
			var out bytes.Buffer
			if e = runDossierCommand(client, value.command, strings.NewReader(value.body), &out); e == nil || out.Len() != 0 {
				t.Fatal("非法输入未在传输前拒绝")
			}
		}
		if calls != 0 {
			t.Fatal("无效命令仍访问管理服务")
		}
	})
}
