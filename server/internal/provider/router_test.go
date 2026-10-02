package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// newRoute 构造一条合规的路由。
func newRoute(name string, p provider.Provider, caps provider.Caps) provider.Route {
	return provider.Route{Name: name, Provider: p, Caps: caps, Compliant: true}
}

func wantNoCapableRoute(t *testing.T, err error) {
	t.Helper()
	pe := asProviderError(t, err)
	if pe.Kind != provider.BadRequest || pe.Code != "no_capable_route" || pe.HTTPStatus != 0 {
		t.Fatalf("得到 %+v，期望 BadRequest/no_capable_route 且无 HTTP 状态", pe)
	}
}

func Test路由_按顺序选第一条合规且能力满足的路由(t *testing.T) {
	const (
		obj    = provider.CapJSONObject
		schema = provider.CapJSONSchema
	)
	cases := []struct {
		name    string
		format  provider.Format
		bOff    bool // 把 B 设为不合规
		wantHit string
	}{
		{"文本请求选第一条", provider.FormatText, false, "A"},
		{"json_object请求选第一条支持它的", provider.FormatJSONObject, false, "A"},
		{"json_schema请求跳过只支持json_object的A", provider.FormatJSONSchema, false, "B"},
		{"json_schema请求且B不合规则选C", provider.FormatJSONSchema, true, "C"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hit := map[string]*scripted{
				"A": {results: []scriptedResult{okResult("A")}},
				"B": {results: []scriptedResult{okResult("B")}},
				"C": {results: []scriptedResult{okResult("C")}},
			}
			routeB := newRoute("B", hit["B"], obj|schema)
			routeB.Compliant = !c.bOff
			p := provider.NewRouter(
				newRoute("A", hit["A"], obj),
				routeB,
				newRoute("C", hit["C"], obj|schema|provider.CapTools),
			)
			resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x", Format: c.format})
			if err != nil || resp.Text != c.wantHit {
				t.Fatalf("得到 %+v, %v，期望由 %s 应答", resp, err, c.wantHit)
			}
			for name, s := range hit {
				want := 0
				if name == c.wantHit {
					want = 1
				}
				if s.Calls() != want {
					t.Fatalf("路由 %s 被调用 %d 次，期望 %d 次", name, s.Calls(), want)
				}
			}
		})
	}
}

func Test路由_json_schema请求不得退到只支持json_object的路由(t *testing.T) {
	a := &scripted{results: []scriptedResult{failResult(provider.Overloaded)}}
	weak := &scripted{results: []scriptedResult{okResult("弱路由")}}
	p := provider.NewRouter(
		newRoute("A", a, provider.CapJSONSchema),
		newRoute("weak", weak, provider.CapJSONObject),
	)
	_, err := p.Generate(context.Background(), provider.Request{Prompt: "x", Format: provider.FormatJSONSchema})
	if kind, _ := provider.KindOf(err); kind != provider.Overloaded {
		t.Fatalf("错误种类 = %v，期望返回 A 的 Overloaded 而不是借助弱路由蒙混过关", kind)
	}
	if weak.Calls() != 0 {
		t.Fatalf("只支持 json_object 的路由被调用了 %d 次，json_schema 请求绝不能退到它", weak.Calls())
	}
}

func Test路由_json_object请求需要明确声明CapJSONObject(t *testing.T) {
	onlySchema := &scripted{results: []scriptedResult{okResult("好")}}
	p := provider.NewRouter(newRoute("A", onlySchema, provider.CapJSONSchema))
	_, err := p.Generate(context.Background(), provider.Request{Prompt: "x", Format: provider.FormatJSONObject})
	wantNoCapableRoute(t, err)
	if onlySchema.Calls() != 0 {
		t.Fatalf("没有符合条件的路由时不得调用任何路由，实际 %d 次", onlySchema.Calls())
	}
}

func Test路由_Compliant为false的路由永不被选(t *testing.T) {
	t.Run("不合规路由被跳过，合规路由失败也不会借用它", func(t *testing.T) {
		bad := &scripted{results: []scriptedResult{okResult("不合规")}}
		good := &scripted{results: []scriptedResult{failResult(provider.Overloaded)}}
		routeBad := newRoute("bad", bad, provider.CapJSONObject|provider.CapJSONSchema)
		routeBad.Compliant = false
		p := provider.NewRouter(routeBad, newRoute("good", good, provider.CapJSONObject))

		_, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
		if kind, _ := provider.KindOf(err); kind != provider.Overloaded {
			t.Fatalf("错误种类 = %v，期望 Overloaded", kind)
		}
		if bad.Calls() != 0 || good.Calls() != 1 {
			t.Fatalf("不合规路由被调用 %d 次、合规路由 %d 次，期望 0 与 1", bad.Calls(), good.Calls())
		}
	})

	t.Run("只有不合规路由时报没有可用路由", func(t *testing.T) {
		bad := &scripted{results: []scriptedResult{okResult("不合规")}}
		routeBad := newRoute("bad", bad, provider.CapJSONObject)
		routeBad.Compliant = false
		_, err := provider.NewRouter(routeBad).Generate(context.Background(), provider.Request{Prompt: "x"})
		wantNoCapableRoute(t, err)
		if bad.Calls() != 0 {
			t.Fatalf("不合规路由被调用 %d 次，期望 0 次", bad.Calls())
		}
	})
}

func Test路由_没有符合条件的路由(t *testing.T) {
	weak := &scripted{results: []scriptedResult{okResult("好")}}
	cases := []struct {
		name   string
		routes []provider.Route
		format provider.Format
	}{
		{"没有任何路由", nil, provider.FormatText},
		{"路由的Provider为nil", []provider.Route{{Name: "nil", Compliant: true}}, provider.FormatText},
		{"能力不满足", []provider.Route{newRoute("weak", weak, provider.CapJSONObject)}, provider.FormatJSONSchema},
		{"能力为空而请求要求json_object", []provider.Route{newRoute("none", weak, 0)}, provider.FormatJSONObject},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := provider.NewRouter(c.routes...).Generate(context.Background(), provider.Request{Prompt: "x", Format: c.format})
			wantNoCapableRoute(t, err)
		})
	}
	if weak.Calls() != 0 {
		t.Fatalf("能力不满足的路由被调用了 %d 次", weak.Calls())
	}
}

func Test路由_回退规则按错误种类(t *testing.T) {
	fallback := map[provider.ErrKind]bool{
		provider.Overloaded: true, provider.Timeout: true, provider.CircuitOpen: true, provider.Auth: true,
		provider.Billing: true,
	}
	for _, kind := range allKinds {
		t.Run(kind.String(), func(t *testing.T) {
			a := &scripted{results: []scriptedResult{failResult(kind)}}
			b := &scripted{results: []scriptedResult{okResult("B")}}
			p := provider.NewRouter(newRoute("A", a, 0), newRoute("B", b, 0))

			resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
			if a.Calls() != 1 {
				t.Fatalf("第一条路由被调用 %d 次，期望 1 次", a.Calls())
			}
			if fallback[kind] {
				if err != nil || resp.Text != "B" || b.Calls() != 1 {
					t.Fatalf("%v 应回退到下一条路由，得到 %+v, %v（第二条被调用 %d 次）", kind, resp, err, b.Calls())
				}
				return
			}
			if got, ok := provider.KindOf(err); !ok || got != kind {
				t.Fatalf("%v 不应回退，应原样返回；得到 %v, %v", kind, got, ok)
			}
			if b.Calls() != 0 {
				t.Fatalf("%v 不应回退，第二条路由却被调用了 %d 次", kind, b.Calls())
			}
		})
	}

	t.Run("未分类的裸错误不回退且原样返回", func(t *testing.T) {
		boom := errors.New("裸错误")
		a := &scripted{results: []scriptedResult{{err: boom}}}
		b := &scripted{results: []scriptedResult{okResult("B")}}
		_, err := provider.NewRouter(newRoute("A", a, 0), newRoute("B", b, 0)).Generate(context.Background(), provider.Request{Prompt: "x"})
		if !errors.Is(err, boom) || b.Calls() != 0 {
			t.Fatalf("得到 %v，第二条被调用 %d 次，期望原样返回且不回退", err, b.Calls())
		}
	})
}

func Test路由_全部失败时返回最后一个错误(t *testing.T) {
	a := &scripted{results: []scriptedResult{failResult(provider.Overloaded)}}
	b := &scripted{results: []scriptedResult{failResult(provider.Timeout)}}
	p := provider.NewRouter(newRoute("A", a, 0), newRoute("B", b, 0))
	_, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
	if kind, _ := provider.KindOf(err); kind != provider.Timeout {
		t.Fatalf("错误种类 = %v，期望最后一条路由的 Timeout", kind)
	}
	if a.Calls() != 1 || b.Calls() != 1 {
		t.Fatalf("两条路由各应被调用 1 次，实际 %d 与 %d", a.Calls(), b.Calls())
	}
}

func Test路由_调用方ctx已结束后不再回退(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &scripted{
		results: []scriptedResult{failResult(provider.Overloaded)},
		hook:    func(ctx context.Context, n int) { cancel() },
	}
	b := &scripted{results: []scriptedResult{okResult("B")}}
	p := provider.NewRouter(newRoute("A", a, 0), newRoute("B", b, 0))

	_, err := p.Generate(ctx, provider.Request{Prompt: "x"})
	if kind, _ := provider.KindOf(err); kind != provider.Overloaded {
		t.Fatalf("错误种类 = %v，期望返回第一条路由的 Overloaded", kind)
	}
	if b.Calls() != 0 {
		t.Fatalf("调用方已不再等待，第二条路由却被调用了 %d 次", b.Calls())
	}
}

func Test路由_Route的Model覆盖请求里的Model(t *testing.T) {
	a := &scripted{results: []scriptedResult{failResult(provider.Overloaded)}}
	b := &scripted{results: []scriptedResult{failResult(provider.Overloaded)}}
	c := &scripted{results: []scriptedResult{okResult("C")}}
	routeA := newRoute("A", a, 0)
	routeA.Model = "model-a"
	routeB := newRoute("B", b, 0)
	routeB.Model = "model-b"
	p := provider.NewRouter(routeA, routeB, newRoute("C", c, 0))

	req := provider.Request{TaskID: "task-1", Step: 3, Attempt: 2, Model: "caller", Prompt: "提示词", System: "系统词", MaxTokens: 9}
	if _, err := p.Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	wantModels := []struct {
		name string
		s    *scripted
		want string
	}{
		{"A", a, "model-a"},
		{"B", b, "model-b"},
		{"C", c, "caller"},
	}
	for _, w := range wantModels {
		got := w.s.Requests()[0]
		if got.Model != w.want {
			t.Fatalf("路由 %s 收到的 Model = %q，期望 %q", w.name, got.Model, w.want)
		}
		// 除 Model 外，请求原样传给每条路由。
		got.Model = req.Model
		if got.TaskID != req.TaskID || got.Step != req.Step || got.Attempt != req.Attempt ||
			got.Prompt != req.Prompt || got.System != req.System || got.MaxTokens != req.MaxTokens {
			t.Fatalf("路由 %s 收到的请求被改动: %+v", w.name, got)
		}
	}
}

func Test路由_NewRouter复制传入的路由切片(t *testing.T) {
	a := &scripted{results: []scriptedResult{okResult("A")}}
	routes := []provider.Route{newRoute("A", a, 0)}
	p := provider.NewRouter(routes...)
	routes[0].Compliant = false // 构造之后修改原切片，不应影响已创建的路由器
	routes[0].Provider = nil

	resp, err := p.Generate(context.Background(), provider.Request{Prompt: "x"})
	if err != nil || resp.Text != "A" {
		t.Fatalf("得到 %+v, %v，期望仍由 A 应答", resp, err)
	}
}
