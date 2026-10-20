package tradingcn

// UpstreamEdition 与提交共同标识本适配器核对的社区版；不是远端版本探测结果。
const UpstreamEdition = "community"

// BaselineInfo 只描述适配器协议参照，不读取部署配置、许可文件或任何凭据。
// 社区版无需软件激活，不意味着服务接口匿名开放或已取得商业使用许可。
type BaselineInfo struct {
	Scope                         string `json:"scope"`
	Edition                       string `json:"edition"`
	Commit                        string `json:"commit"`
	Contract                      string `json:"contract"`
	SoftwareActivationRequired    bool   `json:"software_activation_required"`
	ServiceAuthenticationRequired bool   `json:"service_authentication_required"`
	RemoteVersionVerified         bool   `json:"remote_version_verified"`
}

func Baseline() BaselineInfo {
	return BaselineInfo{
		Scope:                         "adapter_baseline",
		Edition:                       UpstreamEdition,
		Commit:                        UpstreamCommit,
		Contract:                      "v2_async_analysis",
		ServiceAuthenticationRequired: true,
	}
}
