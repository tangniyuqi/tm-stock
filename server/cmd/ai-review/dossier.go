package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

// 专题命令一次只处理一个原请求；未知结果不自动重提，先查原编号。
func runDossierCommand(client *reviewClient, command string, input io.Reader, output io.Writer) error {
	if input == nil || output == nil || client == nil || client.http == nil {
		return fmt.Errorf("专题管理须从标准输入读取JSON")
	}
	raw, err := io.ReadAll(io.LimitReader(input, 8193))
	if err != nil || len(raw) > 8192 || task.CheckStageJSON(raw) != nil {
		return fmt.Errorf("专题管理输入JSON无效")
	}
	var result any
	switch command {
	case "dossier-apply":
		request, err := dossier.ParseManagementRequest(raw)
		if err != nil {
			return err
		}
		var receipt dto.AIDossierManagementReceipt
		if err = dossierManagementCall(client, http.MethodPost, "/internal/v1/ai-dossier-management", request, &receipt, false); err != nil {
			return err
		}
		if !validDossierReceipt(receipt) || receipt.RequestID != request.RequestID || receipt.Object != request.Object || receipt.Revision != request.ExpectedRevision+1 || receipt.Enabled != request.Enabled {
			return fmt.Errorf("专题管理回执未确认，请保留原请求编号核对")
		}
		result = receipt
	case "dossier-inspect":
		var key dossier.ManagementObject
		var shape map[string]json.RawMessage
		if json.Unmarshal(raw, &key) != nil || json.Unmarshal(raw, &shape) != nil || len(shape) != 4 || !key.Valid() {
			return fmt.Errorf("专题管理对象无效")
		}
		for _, name := range []string{"kind", "userId", "themeId", "version"} {
			if strings.TrimSpace(string(shape[name])) == "null" || len(shape[name]) == 0 {
				return fmt.Errorf("专题管理对象字段缺失")
			}
		}
		var view dto.AIDossierManagementInspect
		path := fmt.Sprintf("/internal/v1/ai-dossier-management?kind=%s&userId=%d&themeId=%d&version=%d", key.Kind, key.UserID, key.ThemeID, key.Version)
		if err = dossierManagementCall(client, http.MethodGet, path, nil, &view, true); err != nil {
			return err
		}
		if view.Notice != dto.AIDossierManagementNotice || view.Object != key || !dossierManagementDigest(view.VersionDigest) || view.Revision < 0 || view.Revision > 9007199254740991 || view.Managed != (view.Revision > 0) || view.SourceState != "available" && view.SourceState != "unavailable" || view.SourceState == "available" && (view.Preview == nil || view.Preview.ThemeID != key.ThemeID || len(view.Preview.Versions) != 1 || view.Preview.Versions[0].Version != key.Version) || view.SourceState == "unavailable" && view.Preview != nil || view.Permission != nil && (view.Permission.ValidUntil <= 0 || view.Permission.ValidUntil > dossier.ManagementMaxTime) {
			return fmt.Errorf("专题预览响应无效")
		}
		result = view
	case "dossier-request":
		var value struct {
			RequestID string `json:"requestId"`
		}
		var shape map[string]json.RawMessage
		if json.Unmarshal(raw, &value) != nil || json.Unmarshal(raw, &shape) != nil || len(shape) != 1 || !dossier.ValidManagementRequestID(value.RequestID) {
			return fmt.Errorf("专题原请求编号无效")
		}
		var receipt dto.AIDossierManagementReceipt
		if err = dossierManagementCall(client, http.MethodGet, "/internal/v1/ai-dossier-management/requests/"+value.RequestID, nil, &receipt, false); err != nil {
			return err
		}
		if !validDossierReceipt(receipt) || receipt.RequestID != value.RequestID {
			return fmt.Errorf("专题原请求回执不一致")
		}
		result = receipt
	default:
		return fmt.Errorf("未知专题管理命令")
	}
	if err = json.NewEncoder(output).Encode(result); err != nil {
		return fmt.Errorf("专题管理结果输出失败")
	}
	return nil
}
func validDossierReceipt(r dto.AIDossierManagementReceipt) bool {
	return dossier.ValidManagementRequestID(r.RequestID) && r.Object.Valid() && r.ActorID > 0 && r.ActorID <= 9007199254740991 && r.Revision > 0 && r.Revision <= 9007199254740991 && r.RecordedAt > 0 && r.RecordedAt <= dossier.ManagementMaxTime
}
func dossierManagementDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func dossierManagementShape(raw []byte, names []string, nullable []string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(names) {
		return false
	}
	for _, name := range names {
		value, ok := fields[name]
		if !ok {
			return false
		}
		if strings.TrimSpace(string(value)) == "null" {
			allowed := false
			for _, n := range nullable {
				if n == name {
					allowed = true
				}
			}
			if !allowed {
				return false
			}
		}
	}
	return true
}

// 先验原响应的闭集/必填/实际类型，再转换DTO；缺失的false/0不能被当作确认回执。
func dossierManagementCall(c *reviewClient, method, path string, body, out any, inspect bool) error {
	var raw json.RawMessage
	if err := c.call(context.Background(), method, path, body, &raw); err != nil {
		return err
	}
	names := []string{"requestId", "object", "actorId", "revision", "enabled", "recordedAt"}
	nullable := []string{}
	if inspect {
		names = []string{"notice", "object", "versionDigest", "revision", "managed", "permission", "sourceState", "preview"}
		nullable = []string{"permission", "preview"}
	}
	if len(raw) > 2<<20 || task.CheckStageJSON(raw) != nil || !dossierManagementShape(raw, names, nullable) {
		return fmt.Errorf("专题管理响应结构未确认")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || !dossierManagementShape(fields["object"], []string{"kind", "userId", "themeId", "version"}, nil) {
		return fmt.Errorf("专题管理响应对象未确认")
	}
	if inspect && strings.TrimSpace(string(fields["permission"])) != "null" && !dossierManagementShape(fields["permission"], []string{"enabled", "validUntil"}, nil) {
		return fmt.Errorf("专题管理许可状态未确认")
	}
	if json.Unmarshal(raw, out) != nil {
		return fmt.Errorf("专题管理响应类型未确认")
	}
	return nil
}
