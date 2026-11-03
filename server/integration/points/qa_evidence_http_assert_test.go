//go:build integration

package points_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func readQAEvidenceHTTP(t *testing.T, mux http.Handler, bearer string, cid, seq int64, allowed bool) dto.AIQAEvidence {
	t.Helper()
	r := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/ai/conversations/%d/questions/%d/evidence", cid, seq), nil)
	r.Header.Set("Authorization", "Bearer "+bearer)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var envelope struct {
		Code int
		Data json.RawMessage
	}
	if json.Unmarshal(w.Body.Bytes(), &envelope) != nil || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("资料核验回包或缓存策略不符")
	}
	if !allowed {
		if w.Code == 200 || envelope.Code == 0 || len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			t.Fatal("拒绝后仍返回候选资料/调用")
		}
		return dto.AIQAEvidence{}
	}
	var v dto.AIQAEvidence
	if w.Code != 200 || envelope.Code != 0 || json.Unmarshal(envelope.Data, &v) != nil {
		t.Fatal("原JWT资料核验读取失败；正文与凭据隐藏", w.Code)
	}
	if v.Seq != seq || v.PromptVersion != qa.Version || v.CandidateDigest == "" {
		t.Fatal("原问题与提示版本不符")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(envelope.Data, &object) != nil || len(object) != 13 {
		t.Fatal("公开字段闭集不符")
	}
	for _, call := range v.Calls {
		if call.CostMicros != nil {
			t.Fatal("未知账单被补零")
		}
		known := call.Status == "confirmed" || call.Status == "invalid_protocol"
		if (call.InputTokens != nil) != known || (call.OutputTokens != nil) != known {
			t.Fatal("未知/实际用量投影错误")
		}
	}
	return v
}

func assertQAEvidenceScope(t *testing.T, v dto.AIQAEvidence, scope qa.Scope, taskID int64) {
	t.Helper()
	if v.SubjectType != scope.SubjectType || v.SubjectID != scope.SubjectID || v.SubjectName != scope.SubjectName || v.TaskID != taskID || len(v.Sources) != len(scope.Snapshots) {
		t.Fatal("原对象/任务或完整候选范围不符")
	}
	type sentence struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	}
	type source struct {
		ID        int64      `json:"snapshot_id"`
		Hash      string     `json:"hash"`
		Sentences []sentence `json:"sentences"`
	}
	candidates := []source{}
	for n, row := range v.Sources {
		original := scope.Snapshots[n]
		if row.SnapshotID != original.ID || row.Digest != original.Hash || row.SourceType != original.SourceCategory || row.SourceURL != original.URL || row.CollectedAt != original.FetchedAt.UnixMilli() || len(row.Sentences) != len(original.Sentences) {
			t.Fatal("省略或改写了候选来源")
		}
		candidate := source{ID: row.SnapshotID, Hash: row.Digest, Sentences: []sentence{}}
		for i, s := range row.Sentences {
			old := original.Sentences[i]
			if s.ID != old.Number || s.StartByte != old.StartByte || s.EndByte != old.EndByte || s.Text != old.Text {
				t.Fatal("没有完整逐字回取候选句")
			}
			candidate.Sentences = append(candidate.Sentences, sentence{s.ID, s.Text})
		}
		candidates = append(candidates, candidate)
	}
	raw, err := json.Marshal(candidates)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if v.CandidateDigest != hex.EncodeToString(digest[:]) {
		t.Fatal("摘要不是实际完整候选的规范摘要")
	}
}
