package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func runCorrectionCommand(client *reviewClient, command string, input io.Reader, output io.Writer) error {
	if client == nil || client.http == nil || input == nil || output == nil {
		return fmt.Errorf("更正声明管理输入无效")
	}
	raw, err := io.ReadAll(io.LimitReader(input, dossier.MaxCorrectionBytes+1))
	if err != nil || len(raw) > dossier.MaxCorrectionBytes || task.CheckStageJSON(raw) != nil {
		return fmt.Errorf("更正声明管理JSON无效")
	}
	var result any
	switch command {
	case "correction-apply":
		r, err := dossier.ParseCorrectionRequest(raw)
		if err != nil {
			return err
		}
		var receipt dto.AIDossierCorrectionReceipt
		if err = correctionCall(client, http.MethodPost, "/internal/v1/ai-dossier-corrections", r, &receipt, false); err != nil {
			return err
		}
		if !validCorrectionReceipt(receipt) || receipt.RequestID != r.RequestID || receipt.Object != r.Object || receipt.Revision != r.ExpectedRevision+1 || receipt.Enabled != r.Enabled {
			return fmt.Errorf("更正声明回执未确认，请保留原请求编号核对")
		}
		result = receipt
	case "correction-inspect":
		var p dossier.CorrectionPair
		if !dossierManagementShape(raw, []string{"themeId", "beforeVersion", "afterVersion"}, nil) || json.Unmarshal(raw, &p) != nil || !p.Valid() {
			return fmt.Errorf("更正声明版本对象无效")
		}
		path := fmt.Sprintf("/internal/v1/ai-dossier-corrections?themeId=%d&beforeVersion=%d&afterVersion=%d", p.ThemeID, p.BeforeVersion, p.AfterVersion)
		var v dto.AIDossierCorrectionInspect
		if err = correctionCall(client, http.MethodGet, path, nil, &v, true); err != nil {
			return err
		}
		if v.Notice != dto.AIDossierManagementNotice || v.Object != p || !dossierManagementDigest(v.BeforeDigest) || !dossierManagementDigest(v.AfterDigest) || v.Revision < 0 || v.Revision > 9007199254740991 || v.ValidUntil < 0 || v.ValidUntil > dossier.ManagementMaxTime || v.Enabled && v.ValidUntil == 0 || v.SourceState != "available" && v.SourceState != "unavailable" || v.SourceState == "available" && (v.Preview == nil || v.Preview.ThemeID != p.ThemeID || len(v.Preview.Versions) != 2 || v.Preview.Versions[0].Version != p.BeforeVersion || v.Preview.Versions[1].Version != p.AfterVersion) || v.SourceState == "unavailable" && v.Preview != nil {
			return fmt.Errorf("更正声明预览响应无效")
		}
		result = v
	case "correction-request":
		var q struct {
			RequestID string `json:"requestId"`
		}
		if !dossierManagementShape(raw, []string{"requestId"}, nil) || json.Unmarshal(raw, &q) != nil || !dossier.ValidManagementRequestID(q.RequestID) {
			return fmt.Errorf("更正声明原请求编号无效")
		}
		var receipt dto.AIDossierCorrectionReceipt
		if err = correctionCall(client, http.MethodGet, "/internal/v1/ai-dossier-corrections/requests/"+q.RequestID, nil, &receipt, false); err != nil {
			return err
		}
		if !validCorrectionReceipt(receipt) || receipt.RequestID != q.RequestID {
			return fmt.Errorf("更正声明原请求回执不符")
		}
		result = receipt
	default:
		return fmt.Errorf("未知更正声明管理命令")
	}
	if err = json.NewEncoder(output).Encode(result); err != nil {
		return fmt.Errorf("更正声明结果输出失败")
	}
	return nil
}
func validCorrectionReceipt(v dto.AIDossierCorrectionReceipt) bool {
	return dossier.ValidManagementRequestID(v.RequestID) && v.Object.Valid() && v.ActorID > 0 && v.ActorID <= 9007199254740991 && v.Revision > 0 && v.Revision <= 9007199254740991 && v.RecordedAt > 0 && v.RecordedAt <= dossier.ManagementMaxTime
}
func correctionCall(client *reviewClient, method, path string, body, out any, inspect bool) error {
	var raw json.RawMessage
	if err := client.call(context.Background(), method, path, body, &raw); err != nil {
		return err
	}
	names := []string{"requestId", "object", "actorId", "revision", "enabled", "recordedAt"}
	var nullable []string
	if inspect {
		names = []string{"notice", "object", "beforeDigest", "afterDigest", "revision", "enabled", "validUntil", "sourceState", "preview"}
		nullable = []string{"preview"}
	}
	if !dossierManagementShape(raw, names, nullable) {
		return fmt.Errorf("更正声明响应字段无效")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || !dossierManagementShape(fields["object"], []string{"themeId", "beforeVersion", "afterVersion"}, nil) || json.Unmarshal(raw, out) != nil {
		return fmt.Errorf("更正声明响应对象无效")
	}
	return nil
}
