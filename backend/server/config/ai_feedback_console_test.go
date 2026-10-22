package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestFeedbackConsoleConfig(t *testing.T) {
	t.Setenv("TM_AI_FEEDBACK_CONSOLE_ENABLED", "")
	c, err := LoadFeedbackConsole()
	if err != nil || c.Enabled {
		t.Fatal(c, err)
	}
	for _, s := range []string{"TRUE", "1", "yes"} {
		t.Setenv("TM_AI_FEEDBACK_CONSOLE_ENABLED", s)
		if _, err := LoadFeedbackConsole(); err == nil {
			t.Fatal("坏开关放行")
		}
	}
	t.Setenv("TM_AI_FEEDBACK_CONSOLE_ENABLED", "true")
	t.Setenv("TM_AI_FEEDBACK_CONSOLE_UPSTREAM", "http://127.0.0.1:8081")
	t.Setenv("TM_AI_FEEDBACK_CONSOLE_WEB_ORIGIN", "https://admin.example.test")
	t.Setenv("TM_AI_FEEDBACK_CONSOLE_REVIEWERS", `[{"userId":7,"actorId":70,"token":"`+strings.Repeat("t", 40)+`"}]`)
	c, err = LoadFeedbackConsole()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprintf("%+v %#v %+v", c, c, c.Reviewers[0]), strings.Repeat("t", 40)) {
		t.Fatal("凭据格式化泄漏")
	}
	for _, url := range []string{"https://example.test", "http://localhost:8081", "http://192.0.2.1:8081", "http://127.0.0.1:0", "http://127.0.0.1:8081/", "http://x@127.0.0.1:8081", "http://127.0.0.1:8081?", "http://127.0.0.1:8081#x"} {
		v := c
		v.Upstream = url
		if v.Validate() == nil {
			t.Fatal("非法上游放行", url)
		}
	}
	for _, fn := range []func(*FeedbackConsoleConfig){func(c *FeedbackConsoleConfig) { c.WebOrigin = "bad" }, func(c *FeedbackConsoleConfig) { c.Reviewers = nil }, func(c *FeedbackConsoleConfig) { c.Reviewers = append(c.Reviewers, c.Reviewers[0]) }, func(c *FeedbackConsoleConfig) { c.Reviewers[0].UserID = 0 }, func(c *FeedbackConsoleConfig) { c.Reviewers[0].Token = "bad" }, func(c *FeedbackConsoleConfig) { c.Reviewers[0].Token = strings.Repeat(" ", 40) }} {
		v := c
		v.Reviewers = append([]FeedbackConsoleReviewer(nil), c.Reviewers...)
		fn(&v)
		if v.Validate() == nil {
			t.Fatal("坏登记放行")
		}
	}
	for _, s := range []string{`[]`, `[{}]`, `[{"bad":1}]`, `[] []`, `bad`} {
		t.Setenv("TM_AI_FEEDBACK_CONSOLE_REVIEWERS", s)
		if _, err := LoadFeedbackConsole(); err == nil {
			t.Fatal("非法JSON放行")
		}
	}
}
