// Package handler 是 HTTP 传输层。
//
// 依赖方向：handler → service → repository（单向）。
// 本层【不得】import database/sql 或 internal/repository
// （check-architecture.sh 规则 1 与 2 会拦）。
//
// 只用标准库 net/http：Go 1.22+ 的 ServeMux 已支持
// "GET /api/v1/theme/{id}" 这种带方法与路径参数的模式，
// 本服务只有几个端点，没必要为此引入 Web 框架。
package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

// 统一响应信封 {code,msg,data}。
type envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// 业务错误码。与 docs/specs/theme-query/design.md §3 的错误码表一致。
const (
	codeOK              = 0
	codeBadRequest      = 40001
	codeUnauthorized    = 40100
	codeAccessDenied    = 40301 // 未订阅且试吃已用完
	codeThemeNotFound   = 40401
	codeEvidenceNoFound = 40402
	codeInternal        = 50001
)

func writeJSON(w http.ResponseWriter, httpStatus int, e envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// 接口返回的是结构化数据，不该被中间层猜类型
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(e); err != nil {
		log.Printf("[writeJSON] 写响应失败: %v", err)
	}
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Code: codeOK, Msg: "", Data: data})
}

func fail(w http.ResponseWriter, httpStatus, code int, msg string) {
	writeJSON(w, httpStatus, envelope{Code: code, Msg: msg})
}

// failInternal 对外只给一句通用提示，细节只进日志。
//
// 内部错误常带表名、列名、SQL 片段甚至连接串，直接回给客户端等于送情报。
func failInternal(w http.ResponseWriter, where string, err error) {
	log.Printf("[%s] 内部错误: %v", where, err)
	fail(w, http.StatusInternalServerError, codeInternal, "服务繁忙，请稍后重试")
}
