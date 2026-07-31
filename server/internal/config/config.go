// Package config 从环境变量读取运行配置。
//
// 🔴 铁律：任何密钥（DSN、口令、token）只从环境变量读，【不得】写进代码或配置文件。
// 仓库里出现明文密钥会被 scripts/check-secret-scan.sh 拦下（L1 hook / L2 pre-commit / CI）。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 运行配置。
type Config struct {
	Port string
	DSN  string

	// 行情开关。一期 QuoteEnabled=false（ADR-0006：涨跌幅不是差异化，
	// 且需行情商用授权，见 docs/上线前置授权清单.md §2）。
	QuoteEnabled  bool
	QuoteDelayMin int
	QuoteMock     bool

	// DevGrantPaid 仅供本地开发：无条件放行付费内容。
	//
	// ⚠️ 默认 false（fail-closed）。会员与试吃能力尚未开发（member-center 未实现），
	// 若默认放行，付费内容会在会员系统上线前就整体裸奔。
	// 宁可开发时多设一个环境变量，也不能让"忘了关"变成线上泄漏。
	DevGrantPaid bool
}

// Load 从环境变量装载。返回的错误里【不含】DSN 内容，避免密钥进日志。
func Load() (*Config, error) {
	c := &Config{
		Port:          envStr("TM_APP_PORT", "8080"),
		DSN:           os.Getenv("TM_DB_DSN"),
		QuoteEnabled:  envBool("TM_QUOTE_ENABLED", false),
		QuoteDelayMin: envInt("TM_QUOTE_DELAY_MIN", 15),
		QuoteMock:     envBool("TM_QUOTE_MOCK", false),
		DevGrantPaid:  envBool("TM_DEV_GRANT_PAID", false),
	}
	if c.DSN == "" {
		return nil, fmt.Errorf("缺少环境变量 TM_DB_DSN")
	}
	// datetime(3) 必须靠 parseTime 才能扫进 time.Time，
	// 缺了会在运行时报 "[]uint8 into *time.Time"——那种错很难一眼看出根因，
	// 不如启动就拦住。
	if !strings.Contains(c.DSN, "parseTime=true") {
		return nil, fmt.Errorf("TM_DB_DSN 必须包含 parseTime=true（否则时间字段无法扫描）")
	}
	if c.QuoteEnabled && c.QuoteDelayMin <= 0 {
		return nil, fmt.Errorf("启用行情时 TM_QUOTE_DELAY_MIN 必须为正数")
	}
	return c, nil
}

// Redacted 返回可安全打印的摘要 —— 刻意不含 DSN。
func (c *Config) Redacted() string {
	return fmt.Sprintf("port=%s quote=%v(delay=%dmin,mock=%v) devGrantPaid=%v dsn=<已隐藏>",
		c.Port, c.QuoteEnabled, c.QuoteDelayMin, c.QuoteMock, c.DevGrantPaid)
}

func envStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// envBool 只把明确的真值当 true。
// 刻意不把无法解析的值当 true —— 配置写错时应该退回安全的默认值，
// 而 DevGrantPaid 的安全默认值是 false。
func envBool(k string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
