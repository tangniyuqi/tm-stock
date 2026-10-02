// Package providertest 提供测试 provider 包用的确定性替身：
// 按请求出队的 mock 服务（NewServer）与可手动推进的时钟（FakeClock）。
//
// mock 服务按“场景脚本”工作：脚本是一串 Step，第 N 个到达的请求由第 N 个 Step 应答，
// 脚本耗尽后再来的请求会得到 500，并让测试记录一次错误，这样测试不可能悄悄多发请求。
// Count 与 Requests 反映上游实际收到的请求，用来精确断言“没有隐式重试”。
//
// 该包只依赖 provider 包与标准库，仅供测试使用，不得被生产代码引用。
package providertest
