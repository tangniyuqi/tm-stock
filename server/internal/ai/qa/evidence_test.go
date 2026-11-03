package qa

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestQAEvidenceCurrentScopeAndExactRequests(t *testing.T) {
	c, engine, store, plan := durableFixture(t)
	queueQA(t, c)
	if _, err := c.Run(context.Background(), engine); err != nil {
		t.Fatal(err)
	}
	before := copyQA(store.data)
	v, err := c.ReadEvidence(context.Background())
	if err != nil || v.State != "answered" || v.PromptVersion != Version || !digest(v.CandidateDigest) || len(v.Sources) != len(plan.scope.Snapshots) || len(v.Calls) != 3 {
		t.Fatal("缺少完整候选与原调用", err)
	}
	for n, source := range v.Sources {
		original := plan.scope.Snapshots[n]
		if source.Digest != original.Hash || source.SourceURL != original.URL || len(source.Sentences) != len(original.Sentences) {
			t.Fatal("只导出选中引文或改变来源")
		}
		for i, sentence := range source.Sentences {
			if sentence.ID != i+1 || sentence.Text != original.Body[sentence.StartByte:sentence.EndByte] {
				t.Fatal("没有逐字回取候选")
			}
		}
	}
	for n, call := range v.Calls {
		if call.Step != n+1 || call.Status != "confirmed" || call.RequestDigest != store.data.Calls[n].Call.RequestDigest || call.InputTokens == nil || *call.InputTokens != testUsage.Prompt || call.OutputTokens == nil || *call.OutputTokens != testUsage.Completion {
			t.Fatal("调用/实际用量不符")
		}
	}
	if !reflect.DeepEqual(before, store.data) {
		t.Fatal("读取修改存证或配额")
	}
	v.Sources[0].Sentences[0].Text = "修改副本"
	again, err := c.ReadEvidence(context.Background())
	if err != nil || again.Sources[0].Sentences[0].Text == "修改副本" {
		t.Fatal("副本修改影响原计划", err)
	}
	for _, raw := range []string{fmt.Sprint(again), fmt.Sprintf("%#v", again)} {
		if strings.Contains(raw, plan.input.Question) || strings.Contains(raw, plan.scope.Snapshots[0].Body) {
			t.Fatal("调试格式泄漏正文")
		}
	}
	store.authorized = false
	if denied, err := c.ReadEvidence(context.Background()); err == nil || len(denied.Sources) != 0 || len(denied.Calls) != 0 {
		t.Fatal("撤权仍可读取候选或调用")
	}
}

func TestQAEvidenceFailedCallsAndTerminalRules(t *testing.T) {
	for _, kind := range []string{"queued", "before_begin", "reserved", "not_sent", "response_unknown", "invalid_metadata", "invalid_protocol", "confirmed_prefix"} {
		t.Run(kind, func(t *testing.T) {
			c, _, store, plan := durableFixture(t)
			queueQA(t, c)
			if kind == "queued" {
				if _, err := c.ReadEvidence(context.Background()); err == nil {
					t.Fatal("在途状态冒充完整存证")
				}
				return
			}
			if kind == "before_begin" {
				if err := c.RejectQueued(context.Background()); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := c.Begin(context.Background(), plan.Proof()); err != nil {
					t.Fatal(err)
				}
				call := manualCall(t, plan, 1, nil)
				if err := c.StartCall(context.Background(), call); err != nil {
					t.Fatal(err)
				}
				if kind != "reserved" {
					result := CallResult{Failure: kind}
					if kind == "invalid_protocol" || kind == "confirmed_prefix" {
						result = manualRows(t, plan)[0].Result
						if kind == "invalid_protocol" {
							result.Failure, result.Decision = kind, Decision{}
						}
					}
					if err := c.FinishCall(context.Background(), call, result); err != nil {
						t.Fatal(err)
					}
				}
				if err := c.Fail(context.Background(), "unconfirmed"); err != nil {
					t.Fatal(err)
				}
			}
			before := copyQA(store.data)
			v, err := c.ReadEvidence(context.Background())
			if err != nil || v.State != "failed" || len(v.Calls) != len(before.Calls) || !reflect.DeepEqual(before, store.data) {
				t.Fatal("失败记录丢失或发生写入", err)
			}
			if len(v.Calls) > 0 {
				known := kind == "invalid_protocol" || kind == "confirmed_prefix"
				if (v.Calls[0].InputTokens != nil) != known || (v.Calls[0].OutputTokens != nil) != known {
					t.Fatal("未知/未发出用量被补零或丢弃实际用量")
				}
			}
		})
	}
	for _, kind := range []string{"request", "profile", "model", "count", "reservation", "order", "usage", "receipt"} {
		t.Run("tamper_"+kind, func(t *testing.T) {
			c, engine, store, _ := durableFixture(t)
			queueQA(t, c)
			if _, err := c.Run(context.Background(), engine); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "request":
				store.data.Calls[0].Call.RequestDigest = strings.Repeat("e", 64)
			case "profile":
				store.data.Calls[0].Call.ProfileDigest = strings.Repeat("e", 64)
			case "model":
				store.data.Calls[0].Call.Model = "另一个测试模型"
			case "count":
				store.data.Calls = store.data.Calls[:2]
			case "reservation":
				store.data.Question.ReservedTokens--
			case "order":
				store.data.Calls[0], store.data.Calls[1] = store.data.Calls[1], store.data.Calls[0]
			case "usage":
				store.data.Calls[0].Result.Usage.Cached = testUsage.Prompt + 1
			case "receipt":
				store.data.Question.Citations[0].SentenceIDs = []int{2}
			}
			if v, err := c.ReadEvidence(context.Background()); err == nil || len(v.Sources) != 0 || len(v.Calls) != 0 {
				t.Fatal("篡改后返回完整资料")
			}
		})
	}
}

type evidenceChangeStore struct {
	*qaMemoryStore
	reads  int
	change func()
}

func (s *evidenceChangeStore) WithinQA(ctx context.Context, write bool, fn func(Tx) error) error {
	s.reads++
	if s.reads == 2 {
		s.change()
	}
	return s.qaMemoryStore.WithinQA(ctx, write, fn)
}
func TestQAEvidenceSecondReadRevocationAndLateResult(t *testing.T) {
	for _, kind := range []string{"authorization", "late_result"} {
		t.Run(kind, func(t *testing.T) {
			c, engine, store, _ := durableFixture(t)
			queueQA(t, c)
			if _, err := c.Run(context.Background(), engine); err != nil {
				t.Fatal(err)
			}
			wrapped := &evidenceChangeStore{qaMemoryStore: store, change: func() {
				if kind == "authorization" {
					store.authorized = false
				} else {
					store.data.Calls[0].Result.RequestID = "late-test-result"
				}
			}}
			c.store = wrapped
			if v, err := c.ReadEvidence(context.Background()); err == nil || len(v.Sources) != 0 || len(v.Calls) != 0 || wrapped.reads != 2 {
				t.Fatal("第二遍撤权或结果变化仍泄漏原资料")
			}
		})
	}
	var empty *DurableControl
	if _, err := empty.ReadEvidence(context.Background()); err == nil {
		t.Fatal("空读取器有效")
	}
}
