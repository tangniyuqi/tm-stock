package provider

import (
	"context"
	"slices"
)

// Route 是路由器可选的一条出路。
type Route struct {
	// Name 是路由名（通常是厂商名），仅用于识别与调试。
	Name     string
	Provider Provider
	// Caps 是该路由能满足的能力。
	Caps Caps
	// Compliant 表示该路由在合规白名单内。为 false 的路由永远不会被选中。
	Compliant bool
	// Model 非空时覆盖 Request.Model。各厂商的模型名不同，回退到另一家时需要换名。
	Model string
}

// NewRouter 返回按顺序选路并回退的 Provider。
//
// 选路：按给定顺序，取第一个“合规且能力满足请求需要”的路由。请求要 json_schema 就必须有
// CapJSONSchema，不会退到只支持 json_object 的路由；要 json_object 必须有 CapJSONObject。
//
// 回退：只有当前路由返回 Overloaded、Timeout、CircuitOpen、Auth、Billing 才换下一条符合条件的路由
// （路由内若套了 Retry，指的是重试耗尽之后的结果）。BadRequest、ContentFilter、ContextLen、Canceled
// 属于请求或调用方的问题，换一条路由多半没用还会多花钱；RateLimit、Malformed、BudgetExceeded 与
// 未分类错误不在上述五种之内，同样原样返回（RateLimit 保留 RetryAfter，便于上层回排队）。
// 调用方 ctx 已结束时也不再尝试后面的路由。
//
// 没有任何路由符合条件时返回 *Error{Kind: BadRequest, Code: "no_capable_route"}，且不向任何上游发请求；
// 所有符合条件的路由都失败时返回最后一个错误。
func NewRouter(routes ...Route) Provider {
	return &router{routes: slices.Clone(routes)}
}

type router struct {
	routes []Route
}

func (r *router) Generate(ctx context.Context, req Request) (Response, error) {
	need := requiredCaps(req)
	var last error
	for _, route := range r.routes {
		if route.Provider == nil || !route.Compliant || !route.Caps.Has(need) {
			continue
		}
		if last != nil && ctx.Err() != nil {
			break // 调用方已不再等待，别再去试后面的路由
		}
		routed := req
		if route.Model != "" {
			routed.Model = route.Model
		}
		resp, err := route.Provider.Generate(ctx, routed)
		if err == nil {
			return resp, nil
		}
		last = err
		if !shouldFallback(err) {
			return Response{}, err
		}
	}
	if last == nil {
		return Response{}, &Error{Kind: BadRequest, Code: "no_capable_route"}
	}
	return Response{}, last
}

// shouldFallback 判断一个失败是否值得换路由重试。
func shouldFallback(err error) bool {
	kind, ok := KindOf(err)
	if !ok {
		return false
	}
	switch kind {
	case Overloaded, Timeout, CircuitOpen, Auth, Billing:
		return true
	}
	return false
}
