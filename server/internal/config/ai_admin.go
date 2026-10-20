package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// AIAdminReviewer 的身份由独立凭据绑定，客户端不能更改；禁止打印此结构。
type AIAdminReviewer struct {
	ActorID int64  `json:"actor_id"`
	Token   string `json:"token"`
}

type AIAdminConfig struct {
	Addr      string
	DSN       string
	Reviewers []AIAdminReviewer
}

func LoadAIAdmin() (*AIAdminConfig, error) {
	base, err := Load()
	if err != nil {
		return nil, err
	}
	cfg := &AIAdminConfig{DSN: base.DSN, Addr: envStr("TM_AI_ADMIN_ADDR", "127.0.0.1:8081")}
	if err := ValidateAIAdminAddress(cfg.Addr); err != nil {
		return nil, err
	}
	raw := os.Getenv("TM_AI_ADMIN_REVIEWERS")
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg.Reviewers); err != nil {
		return nil, fmt.Errorf("TM_AI_ADMIN_REVIEWERS 必须是有效审核人凭据数组")
	}
	// 用完整 JSON 校验同时拒绝尾随内容，不把解析错误原文或密钥带入日志。
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("TM_AI_ADMIN_REVIEWERS 含无效内容")
	}
	if err := ValidateAIAdminReviewers(cfg.Reviewers); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ValidateAIAdminAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	ip := net.ParseIP(host)
	number, portErr := strconv.Atoi(port)
	if err != nil || ip == nil || !ip.IsLoopback() || portErr != nil || number < 1 || number > 65535 {
		return fmt.Errorf("内部 AI 管理地址必须为回环 IP 与有效端口，禁止公网监听")
	}
	return nil
}

func ValidateAIAdminReviewers(reviewers []AIAdminReviewer) error {
	if len(reviewers) < 1 || len(reviewers) > 100 {
		return fmt.Errorf("必须配置 1 至 100 个内部审核凭据")
	}
	seen := map[string]bool{}
	for _, reviewer := range reviewers {
		if reviewer.ActorID <= 0 || len(reviewer.Token) < 32 || len(reviewer.Token) > 256 || seen[reviewer.Token] {
			return fmt.Errorf("内部审核凭据身份、长度或唯一性无效")
		}
		for _, c := range reviewer.Token {
			if c < 33 || c > 126 {
				return fmt.Errorf("内部审核凭据须为无空白的 ASCII 字符")
			}
		}
		seen[reviewer.Token] = true
	}
	return nil
}
