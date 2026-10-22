package qa

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

func TestSelectionProtocolRejectsUntrustedStructure(t *testing.T) {
	for n, raw := range []string{
		`{}`, `null`, `[]`, selectedJSON + `{}`, "```json\n" + selectedJSON + "\n```",
		strings.Replace(selectedJSON, `"action"`, `"Action"`, 1),
		strings.Replace(selectedJSON, `"action":"answer"`, `"action":"answer","ACTION":"answer"`, 1),
		strings.Replace(selectedJSON, `"reason":"none"`, `"reason":null`, 1),
		strings.Replace(selectedJSON, `"reason":"none"`, `"reason":"none","body":"伪造正文"`, 1),
		strings.Replace(selectedJSON, `"snapshot_id":7`, `"snapshot_id":7,"url":"https://example.test/"`, 1),
		strings.Replace(selectedJSON, `"snapshot_id":7`, `"snapshot_id":"7"`, 1),
		strings.Replace(selectedJSON, `"sentence_ids":[1]`, `"Sentence_ids":[1]`, 1),
		strings.Replace(selectedJSON, `"sentence_ids":[1]`, `"sentence_ids":null`, 1),
		`{"action":"answer","reason":"none","citations":[]}`,
		`{"action":"refuse","reason":"none","citations":[]}`,
		`{"action":"refuse","reason":"no_evidence","citations":null}`,
		strings.Replace(selectedJSON, `"answer"`, `"refuse"`, 1),
		strings.Repeat(" ", 8193) + selectedJSON,
		string([]byte{0xff}),
	} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			e, p, c, srv := fixture(t, providertest.Success(raw, testUsage))
			if v, err := e.Run(context.Background(), p); err == nil || v != nil || srv.Count() != 1 || len(c.receipts) != 0 {
				t.Fatal("非法选择协议仍交付")
			}
		})
	}
}

func TestJudgementProtocolRejectsFreeOutputAndAmbiguity(t *testing.T) {
	for n, raw := range []string{
		`null`, `[]`, `{}`, passedJSON + passedJSON,
		strings.Replace(passedJSON, `"verdict"`, `"Verdict"`, 1),
		strings.Replace(passedJSON, `"verdict":"pass"`, `"verdict":"pass","VERDICT":"block"`, 1),
		strings.Replace(passedJSON, `"reason":"verified"`, `"reason":null`, 1),
		strings.Replace(passedJSON, `"reason":"verified"`, `"reason":"verified","body":"模型自由回答"`, 1),
		`{"verdict":"pass","reason":"unsupported"}`,
		`{"verdict":"block","reason":"verified"}`,
		`{"verdict":"uncertain","reason":"unsupported"}`,
		`{"verdict":"unknown","reason":"verified"}`,
		strings.Repeat(" ", 1025) + passedJSON,
	} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			e, p, c, srv := fixture(t, providertest.Success(selectedJSON, testUsage), providertest.Success(raw, testUsage))
			if v, err := e.Run(context.Background(), p); err == nil || v != nil || srv.Count() != 2 || len(c.receipts) != 0 {
				t.Fatal("非法核验协议仍交付")
			}
		})
	}
}

func TestRawUsageAndModelCannotBeFilledOrRounded(t *testing.T) {
	base := `{"id":"test-request","model":"qa-test-model","choices":[{"message":{"content":%s},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":30,"total_tokens":50}}`
	text, err := json.Marshal(selectedJSON)
	if err != nil {
		t.Fatal(err)
	}
	valid := fmt.Sprintf(base, text)
	for name, raw := range map[string]string{
		"缺model":  strings.Replace(valid, `"model":"qa-test-model",`, ``, 1),
		"模型漂移":    strings.Replace(valid, "qa-test-model", "another-test-model", 1),
		"缺total":  strings.Replace(valid, `,"total_tokens":50`, ``, 1),
		"total为空": strings.Replace(valid, `"total_tokens":50`, `"total_tokens":null`, 1),
		"零total":  strings.Replace(valid, `"total_tokens":50`, `"total_tokens":0`, 1),
		"小数token": strings.Replace(valid, `"prompt_tokens":20`, `"prompt_tokens":20.1`, 1),
		"重复token": strings.Replace(valid, `"prompt_tokens":20`, `"prompt_tokens":20,"prompt_tokens":20`, 1),
		"缓存小数":    strings.Replace(valid, `"prompt_tokens":20`, `"prompt_tokens":20,"prompt_tokens_details":{"cached_tokens":0.1}`, 1),
		"缓存负数":    strings.Replace(valid, `"prompt_tokens":20`, `"prompt_tokens":20,"prompt_tokens_details":{"cached_tokens":-1}`, 1),
		"缓存为空":    strings.Replace(valid, `"prompt_tokens":20`, `"prompt_tokens":20,"prompt_tokens_details":null`, 1),
		"旧缓存小数":   strings.Replace(valid, `"prompt_tokens":20`, `"prompt_tokens":20,"prompt_cache_hit_tokens":0.1`, 1),
		"推理小数":    strings.Replace(valid, `"completion_tokens":30`, `"completion_tokens":30,"completion_tokens_details":{"reasoning_tokens":0.1}`, 1),
		"多choice": strings.Replace(valid, `"stop"}]`, `"stop"},{"message":{"content":"任意"},"finish_reason":"stop"}]`, 1),
		"截断完成":    strings.Replace(valid, `"finish_reason":"stop"`, `"finish_reason":"length"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			e, p, c, srv := fixture(t, providertest.Status(200, raw))
			if v, err := e.Run(context.Background(), p); err == nil || v != nil || srv.Count() != 1 || len(c.results) != 1 || c.results[0].Failure != "invalid_metadata" || len(c.receipts) != 0 {
				t.Fatalf("兼容补值未拒绝：%v %+v", err, c.results)
			}
		})
	}
	t.Run("已耗用但安全拦截", func(t *testing.T) {
		e, p, c, _ := fixture(t, providertest.Status(200, strings.Replace(valid, `"finish_reason":"stop"`, `"finish_reason":"content_filter"`, 1)))
		if v, err := e.Run(context.Background(), p); err == nil || v != nil || len(c.results) != 1 || c.results[0].Usage != testUsage || c.results[0].RequestID != "test-request" || c.results[0].Failure != "response_unknown" {
			t.Fatal("丢失失败但已耗用token")
		}
	})
	for _, extra := range []string{`,"prompt_tokens_details":{"cached_tokens":5},"completion_tokens_details":{"reasoning_tokens":3}`, `,"prompt_cache_hit_tokens":5`, `,"prompt_tokens_details":{},"completion_tokens_details":{}`} {
		t.Run("有效扩展用量"+extra, func(t *testing.T) {
			e, p, c, _ := fixture(t, providertest.Status(200, strings.Replace(valid, `"total_tokens":50`, `"total_tokens":50`+extra, 1)), providertest.Success(passedJSON, testUsage), providertest.Success(passedJSON, testUsage))
			if v, err := e.Run(context.Background(), p); err != nil || v == nil || len(c.results) != 3 {
				t.Fatalf("有效原始用量被拒绝：%v", err)
			}
		})
	}
}

func TestResponseMetadataLimits(t *testing.T) {
	_, p, _, _ := fixture(t)
	for name, edit := range map[string]func(*provider.Response){
		"空请求编号": func(r *provider.Response) { r.RequestID = "" },
		"估算":    func(r *provider.Response) { r.UsageEstimated = true },
		"输入超限":  func(r *provider.Response) { r.Usage = provider.Usage{Prompt: 12001, Completion: 30, Total: 12031} },
		"零输入":   func(r *provider.Response) { r.Usage = provider.Usage{Completion: 30, Total: 30} },
		"零输出":   func(r *provider.Response) { r.Usage = provider.Usage{Prompt: 20, Total: 20} },
		"负缓存":   func(r *provider.Response) { r.Usage.Cached = -1 },
		"缓存溢出":  func(r *provider.Response) { r.Usage.Cached = 21 },
		"负推理":   func(r *provider.Response) { r.Usage.Reasoning = -1 },
		"推理溢出":  func(r *provider.Response) { r.Usage.Reasoning = 31 },
		"坏原始包":  func(r *provider.Response) { r.Raw = []byte("not-json") },
	} {
		t.Run(name, func(t *testing.T) {
			r := provider.Response{Model: p.cfg.Selector.Name, RequestID: "test-request", FinishReason: "stop", Usage: testUsage}
			edit(&r)
			if responseValid(r, p.cfg.Selector, p.cfg.Limits) {
				t.Fatal("元数据失败仍放行")
			}
		})
	}
}

func FuzzClosedDecoders(f *testing.F) {
	for _, s := range []string{selectedJSON, passedJSON, `null`, `{"action":"refuse","reason":"no_evidence","citations":[]}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if v, err := decodeSelection(s); err == nil {
			b, e := json.Marshal(v)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = decodeSelection(string(b)); e != nil {
				t.Fatal("选择闭集不可往返")
			}
		}
		if v, err := decodeJudge(s); err == nil {
			b, e := json.Marshal(v)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = decodeJudge(string(b)); e != nil {
				t.Fatal("判定闭集不可往返")
			}
		}
	})
}
