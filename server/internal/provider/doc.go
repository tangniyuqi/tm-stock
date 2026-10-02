// Package provider 定义内部运营任务可使用的模型 Provider 抽象，
// 并提供 OpenAI 兼容适配层、可组合中间件与模型注册表。
//
// 该包不参与 C 端 HTTP 路由，调用方必须在内部任务边界内使用。
//
// # 组成
//
//   - Provider、Request、Response：最小调用契约；扩展字段全部可选，旧调用方不受影响。
//   - OpenAICompat（New）：自写的 net/http 适配层，只调用 /chat/completions，只依赖标准库。
//   - Middleware：日志（LoggingMiddleware）、预算（BudgetMiddleware）、重试（RetryMiddleware）、
//     熔断（Breaker）、路由（NewRouter），用 Chain 组合。
//   - Registry：模型注册表与能力表，缺备案号的模型不可启用于 C 端（AC-L4）。
//   - 子包 providertest：按请求出队的 mock 服务与可注入时钟，仅供测试使用。
//
// # 不变量
//
//   - 绝不隐式重试：适配层单次 Generate 至多向上游发一次请求；http.Client.Timeout 为 0，
//     请求体不设 GetBody，也不跟随重定向，所以 net/http 不会悄悄重发 POST。重试只发生在 Retry 层，
//     并且每次尝试都会把 Request.Attempt 加一，日志可以逐次区分。
//   - 错误对外只暴露种类、HTTP 状态与错误码：Error.Error() 不含供应商原文，
//     原文只保留在 Error.Detail（仅供脱敏后的内部日志使用）。
//   - 密钥不落地：适配层会把配置的 API 密钥从它交出的所有副本里抹掉（Response.Text、Response.Raw、
//     Error.Detail、日志捕获），Redact 再按 Authorization、api_key、Bearer 等形态做一遍兜底脱敏。
//   - 失败可分类：见 ErrKind。重试、熔断、回退都只看分类，不看文本。
//
// # 装配
//
// Chain 按“最先写的在最外层”组合。设计文档给出的顺序是 日志 → 预算 → 重试 → 熔断 → 路由：
//
//	p := provider.Chain(router,
//		provider.LoggingMiddleware(sink, nil),
//		provider.BudgetMiddleware(),
//		provider.RetryMiddleware(provider.DefaultRetryPolicy(), nil, nil),
//	)
//
// 熔断按（厂商，模型）统计，因此通常套在每条路由自己的 Provider 上，再交给 Router 做回退：
//
//	breakers := provider.NewBreaker(provider.DefaultBreakerConfig(), nil)
//	route := provider.Route{
//		Name: "vendor-a", Compliant: true, Caps: provider.CapJSONObject,
//		Provider: provider.Chain(adapterA, breakers.Middleware("vendor-a")),
//	}
//
// 两点需要留意：日志放在最外层时，一次 Generate 只记一条汇总（取最后一次尝试的原始报文）；
// 要“每次尝试一条日志”，把 LoggingMiddleware 放到 Retry 的内层。预算放在 Retry 外层时按
// 逻辑调用计数，放在内层则按每次上游请求计数，后者对成本的封顶更严。
package provider
