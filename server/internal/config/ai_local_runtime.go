package config

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// AILocalRuntime 仅为隔离进程启动边界；不授予资料、模型、定价或发布批准。
// 文件不允许密钥或DSN，真实调用在本版本必须显式为false。
type AILocalRuntime struct {
	Version        int    `json:"version"`
	Environment    string `json:"environment"`
	InstanceID     string `json:"instance_id"`
	Database       string `json:"database"`
	Listen         string `json:"listen"`
	RealCalls      bool   `json:"real_calls"`
	PollIntervalMS int    `json:"poll_interval_ms"`
	BatchSize      int    `json:"batch_size"`
}

func invalidLocalRuntime() error {
	return fmt.Errorf("本地AI装配配置无效或隔离边界不符（详情已隐藏）")
}

func ReadAILocalRuntime(path string) (*AILocalRuntime, error) {
	b, err := readAILocalFile(path, 8192)
	if err != nil {
		return nil, err
	}
	return decodeAILocalRuntime(b)
}

func readAILocalFile(path string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(path) || strings.HasPrefix(filepath.ToSlash(path), "//") {
		return nil, invalidLocalRuntime()
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, invalidLocalRuntime()
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, invalidLocalRuntime()
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, invalidLocalRuntime()
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, invalidLocalRuntime()
	}
	return b, nil
}

func decodeAILocalRuntime(b []byte) (*AILocalRuntime, error) {
	// 逐键读取，同时拒绝重复、大小写别名、未知字段、null和尾随JSON。
	d := json.NewDecoder(bytes.NewReader(b))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, invalidLocalRuntime()
	}
	allowed := map[string]bool{"version": true, "environment": true, "instance_id": true, "database": true, "listen": true, "real_calls": true, "poll_interval_ms": true, "batch_size": true}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, invalidLocalRuntime()
		}
		name, ok := key.(string)
		if !ok || !allowed[name] || seen[name] {
			return nil, invalidLocalRuntime()
		}
		seen[name] = true
		var raw json.RawMessage
		if d.Decode(&raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, invalidLocalRuntime()
		}
	}
	if _, err = d.Token(); err != nil || len(seen) != len(allowed) {
		return nil, invalidLocalRuntime()
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, invalidLocalRuntime()
	}
	var c AILocalRuntime
	if json.Unmarshal(b, &c) != nil || c.validate() != nil {
		return nil, invalidLocalRuntime()
	}
	return &c, nil
}

func (c *AILocalRuntime) validate() error {
	if c == nil || c.Version != 1 || c.Environment != "local-isolated" || len(c.InstanceID) != 32 || c.Database != "tm_ai_local_"+c.InstanceID || c.RealCalls || c.PollIntervalMS < 1000 || c.PollIntervalMS > 60000 || c.BatchSize < 1 || c.BatchSize > 100 {
		return invalidLocalRuntime()
	}
	if b, err := hex.DecodeString(c.InstanceID); err != nil || hex.EncodeToString(b) != c.InstanceID {
		return invalidLocalRuntime()
	}
	host, port, err := net.SplitHostPort(c.Listen)
	n, pErr := strconv.Atoi(port)
	if err != nil || host != "127.0.0.1" || pErr != nil || n < 1024 || n > 65535 {
		return invalidLocalRuntime()
	}
	return nil
}

// CheckDSN 只检查环境注入DSN的目标；不连接数据库，不输出用户名或口令。
// 实例专属库名不是实际数据库所有权证明，后续仍须结构及隔离标记核查。
func (c *AILocalRuntime) CheckDSN(dsn string) error {
	if c.validate() != nil {
		return invalidLocalRuntime()
	}
	d, err := mysql.ParseDSN(dsn)
	if err != nil || d.Net != "tcp" || d.DBName != c.Database || !d.ParseTime {
		return invalidLocalRuntime()
	}
	host, port, err := net.SplitHostPort(d.Addr)
	n, pErr := strconv.Atoi(port)
	if err != nil || host != "127.0.0.1" || pErr != nil || n < 1024 || n > 65535 {
		return invalidLocalRuntime()
	}
	return nil
}

func LoadAILocalRuntime(dsn string) (*AILocalRuntime, error) {
	path := os.Getenv("TM_AI_LOCAL_CONFIG")
	if path == "" {
		return nil, nil
	}
	c, err := ReadAILocalRuntime(path)
	if err != nil {
		return nil, err
	}
	if err = c.CheckDSN(dsn); err != nil {
		return nil, err
	}
	return c, nil
}
