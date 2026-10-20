// ai-review 为内部审核薄客户端：export 写 CSV 到标准输出，apply 从标准输入回传决定。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/reviewcsv"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

type reviewClient struct {
	baseURL, bearer string
	http            *http.Client
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, input io.Reader, output, status io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("须指定一个内部管理命令，凭据从环境变量读取")
	}
	switch args[0] {
	case "export", "apply", "source-register", "source-state", "source-show", "source-audits", "snapshot-import", "snapshot-fetch", "snapshot-show", "extract", "extract-show", "inspection-create", "inspection-show", "inspection-item", "inspection-decide", "inspection-issues":
	default:
		return fmt.Errorf("未知内部管理命令")
	}
	client, err := newReviewClient(os.Getenv("TM_AI_ADMIN_URL"), os.Getenv("TM_AI_ADMIN_TOKEN"))
	if err != nil {
		return err
	}
	defer client.http.CloseIdleConnections()
	if args[0] != "export" && args[0] != "apply" {
		return runLibraryCommand(client, args[0], input, output)
	}
	if args[0] == "export" {
		afterID := int64(0)
		if raw := os.Getenv("TM_AI_REVIEW_AFTER_ID"); raw != "" {
			afterID, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || afterID < 0 {
				return fmt.Errorf("TM_AI_REVIEW_AFTER_ID 必须为非负整数")
			}
		}
		items := make([]candidate.Candidate, 0)
		for len(items) < reviewcsv.MaxRows {
			limit := min(200, reviewcsv.MaxRows-len(items))
			var queue dto.AIReviewQueue
			path := fmt.Sprintf("/internal/v1/ai-review-candidates?after_id=%d&limit=%d", afterID, limit)
			if err := client.call(context.Background(), http.MethodGet, path, nil, &queue); err != nil {
				return err
			}
			if len(queue.Items) == 0 {
				break
			}
			if len(queue.Items) > limit || queue.NextAfterID <= afterID {
				return fmt.Errorf("内部审核分页响应无效")
			}
			for _, item := range queue.Items {
				if item.ID <= afterID {
					return fmt.Errorf("内部审核队列顺序无效")
				}
				c := item.Candidate()
				digest, err := candidate.EvidenceDigest(c)
				if err != nil || digest != item.EvidenceDigest {
					return fmt.Errorf("内部审核候选摘要校验失败")
				}
				items = append(items, c)
				afterID = item.ID
			}
			if afterID != queue.NextAfterID {
				return fmt.Errorf("内部审核分页游标不一致")
			}
			if len(queue.Items) < limit {
				break
			}
		}
		if err := reviewcsv.Write(output, items); err != nil {
			return err
		}
		_, err := fmt.Fprintf(status, "已导出 %d 条待审记录；后续批次可设置 TM_AI_REVIEW_AFTER_ID=%d。\n", len(items), afterID)
		return err
	}
	requests, err := reviewcsv.ReadDecisions(input)
	if err != nil {
		return err
	}
	for i, request := range requests {
		var event dto.AIReviewEvent
		if err := client.call(context.Background(), http.MethodPost, "/internal/v1/ai-review-decisions", request, &event); err != nil {
			return fmt.Errorf("第 %d 条决定回传失败（前 %d 条已确认，可用原 CSV 幂等重试）: %w", i+1, i, err)
		}
		if event.ID <= 0 || event.RequestID != request.RequestID || event.CandidateID != request.CandidateID || event.Decision != request.Decision {
			return fmt.Errorf("第 %d 条审核响应不一致；请保留原 CSV 核对，不要重建请求编号", i+1)
		}
	}
	_, err = fmt.Fprintf(status, "已确认 %d 条审核决定；空决定的记录未处理。\n", len(requests))
	return err
}

func newReviewClient(rawURL, bearer string) (*reviewClient, error) {
	if rawURL == "" {
		rawURL = "http://127.0.0.1:8081"
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("TM_AI_ADMIN_URL 必须为不含凭据、路径和查询参数的本机 HTTP 地址")
	}
	ip := net.ParseIP(parsed.Hostname())
	port, err := strconv.Atoi(parsed.Port())
	if ip == nil || !ip.IsLoopback() || err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("TM_AI_ADMIN_URL 必须指向回环 IP 与有效端口")
	}
	if len(bearer) < 32 || len(bearer) > 256 {
		return nil, fmt.Errorf("缺少有效 TM_AI_ADMIN_TOKEN")
	}
	for _, c := range bearer {
		if c < 33 || c > 126 {
			return nil, fmt.Errorf("TM_AI_ADMIN_TOKEN 含无效字符")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Transport: transport, Timeout: 95 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return &reviewClient{baseURL: strings.TrimSuffix(rawURL, "/"), bearer: bearer, http: client}, nil
}

func (c *reviewClient) call(ctx context.Context, method, path string, body, out any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("审核请求编码失败")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("审核请求构造失败")
	}
	req.Header.Set("Authorization", "Bearer "+c.bearer)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("内部审核连接失败，请核对服务是否启动")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<20+1))
	if err != nil || len(raw) > 16<<20 {
		return fmt.Errorf("内部审核响应读取失败或过大")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("内部审核返回 HTTP %d，未回显服务原文", response.StatusCode)
	}
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Code == nil || *envelope.Code != 0 || len(envelope.Data) == 0 {
		return fmt.Errorf("内部审核响应格式无效")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("内部审核响应内容无效")
	}
	return nil
}
