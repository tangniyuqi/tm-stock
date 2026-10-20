package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func runLibraryCommand(client *reviewClient, command string, input io.Reader, output io.Writer) error {
	if input == nil {
		return fmt.Errorf("内部管理命令须从标准输入读取 JSON")
	}
	decoder := json.NewDecoder(io.LimitReader(input, 3<<20+1))
	decoder.DisallowUnknownFields()
	method, path := http.MethodPost, ""
	var body any
	var value any
	switch command {
	case "source-audits":
		value = &struct {
			SourceID int64 `json:"source_id"`
			AfterID  int64 `json:"after_id"`
			Limit    int   `json:"limit"`
		}{}
		method = http.MethodGet
	case "inspection-create":
		value = &candidate.InspectionBatchRequest{}
		path = "/internal/v1/ai-inspection-batches"
	case "inspection-decide":
		value = &candidate.InspectionRequest{}
		path = "/internal/v1/ai-inspection-decisions"
	case "inspection-issues":
		value = &struct {
			AfterID int64 `json:"after_id"`
			Limit   int   `json:"limit"`
		}{}
		method = http.MethodGet
	case "source-register":
		value = &dto.AISourceRegistration{}
		path = "/internal/v1/ai-sources"
	case "source-state":
		value = &struct {
			SourceID int64 `json:"source_id"`
			Enabled  *bool `json:"enabled"`
		}{}
	case "source-show", "snapshot-show", "inspection-item":
		value = &struct {
			ID int64 `json:"id"`
		}{}
		method = http.MethodGet
	case "snapshot-import":
		value = &dto.AISnapshotImport{}
		path = "/internal/v1/ai-snapshot-imports"
	case "snapshot-fetch":
		value = &dto.AISnapshotFetch{}
		path = "/internal/v1/ai-snapshot-fetches"
	case "extract":
		value = &pipeline.RunRequest{}
		path = "/internal/v1/ai-extractions"
	case "extract-show", "inspection-show":
		value = &struct {
			RequestID string `json:"request_id"`
		}{}
		method = http.MethodGet
	default:
		return fmt.Errorf("未知资料管理命令")
	}
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("资料管理输入 JSON 无效")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("资料管理输入含尾随内容或超限")
	}
	body = value
	switch v := value.(type) {
	case *struct {
		SourceID int64 `json:"source_id"`
		AfterID  int64 `json:"after_id"`
		Limit    int   `json:"limit"`
	}:
		if v.SourceID <= 0 || v.AfterID < 0 || v.Limit < 1 || v.Limit > 200 {
			return fmt.Errorf("来源审计分页参数无效")
		}
		path = fmt.Sprintf("/internal/v1/ai-sources/%d/audits?after_id=%d&limit=%d", v.SourceID, v.AfterID, v.Limit)
		body = nil
	case *candidate.InspectionBatchRequest:
		if err := v.Validate(); err != nil {
			return err
		}
	case *candidate.InspectionRequest:
		if err := v.Validate(); err != nil {
			return err
		}
	case *struct {
		AfterID int64 `json:"after_id"`
		Limit   int   `json:"limit"`
	}:
		if v.AfterID < 0 || v.Limit < 1 || v.Limit > 200 {
			return candidate.ErrInvalidInspection
		}
		path = fmt.Sprintf("/internal/v1/ai-inspection-issues?after_id=%d&limit=%d", v.AfterID, v.Limit)
		body = nil
	case *struct {
		SourceID int64 `json:"source_id"`
		Enabled  *bool `json:"enabled"`
	}:
		if v.SourceID <= 0 || v.Enabled == nil {
			return fmt.Errorf("来源状态参数无效")
		}
		path = "/internal/v1/ai-sources/" + strconv.FormatInt(v.SourceID, 10) + "/state"
		body = map[string]bool{"enabled": *v.Enabled}
	case *struct {
		ID int64 `json:"id"`
	}:
		if v.ID <= 0 {
			return fmt.Errorf("资料编号无效")
		}
		base := "/internal/v1/ai-sources/"
		if command == "snapshot-show" {
			base = "/internal/v1/ai-snapshots/"
		}
		if command == "inspection-item" {
			base = "/internal/v1/ai-inspection-items/"
		}
		path = base + strconv.FormatInt(v.ID, 10)
		body = nil
	case *struct {
		RequestID string `json:"request_id"`
	}:
		if !pipeline.ValidRequestID(v.RequestID) {
			return pipeline.ErrInvalidRun
		}
		path = "/internal/v1/ai-extractions/" + v.RequestID
		if command == "inspection-show" {
			path = "/internal/v1/ai-inspection-batches/" + v.RequestID
		}
		body = nil
	case *pipeline.RunRequest:
		if err := v.Validate(); err != nil {
			return err
		}
	}
	var data json.RawMessage
	if err := client.call(context.Background(), method, path, body, &data); err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("写内部管理结果失败: %w", err)
	}
	return nil
}
