package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type AITaskStreamHandler struct{ svc *service.AITaskStreamService }

func NewAITaskStreamHandler(svc *service.AITaskStreamService) *AITaskStreamHandler {
	return &AITaskStreamHandler{svc}
}

func (h *AITaskStreamHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ai/tasks/{id}/stream", auth(http.HandlerFunc(h.stream)))
}

func streamAfter(r *http.Request) (int64, error) {
	if r.Method != http.MethodGet || r.URL.ForceQuery || r.ContentLength != 0 || len(r.TransferEncoding) != 0 || len(r.Header.Values("Content-Encoding")) != 0 {
		return 0, points.ErrInvalid
	}
	q, err := queryTaskRead(r, "after_seq")
	if err != nil {
		return 0, err
	}
	values := r.Header.Values("Last-Event-ID")
	if len(values) > 1 {
		return 0, points.ErrInvalid
	}
	parse := func(raw string) (int64, error) {
		if raw == "0" {
			return 0, nil
		}
		n, err := positiveDecimal(raw)
		if err != nil || n > 9007199254740991 {
			return 0, points.ErrInvalid
		}
		return n, nil
	}
	after := int64(0)
	if raw, ok := q["after_seq"]; ok {
		after, err = parse(raw)
		if err != nil {
			return 0, err
		}
	}
	if len(values) == 1 {
		n, err := parse(values[0])
		if err != nil {
			return 0, err
		}
		if _, explicit := q["after_seq"]; explicit && n != after {
			return 0, points.ErrInvalid
		}
		after = n
	}
	return after, nil
}

func streamFailure(err error) (int, int, string) {
	switch {
	case errors.Is(err, points.ErrInvalid):
		return 400, 40001, "任务进度参数无效"
	case errors.Is(err, points.ErrNotFound):
		return 404, 40403, "任务不存在"
	case errors.Is(err, memberauth.ErrUnauthorized):
		return 401, 40100, "会员身份无效，请重新登录"
	case errors.Is(err, service.ErrAITaskStreamCapacity):
		return 429, 42901, "任务进度连接过多，请使用状态查询"
	default:
		return 503, 50302, "任务进度暂不可用，请使用状态查询"
	}
}

func (h *AITaskStreamHandler) stream(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, 40100, "需要会员身份")
		return
	}
	id, err := positiveDecimal(r.PathValue("id"))
	if err == nil && id > 9007199254740991 {
		err = points.ErrInvalid
	}
	after := int64(0)
	if err == nil {
		after, err = streamAfter(r)
	}
	if err != nil {
		status, code, msg := streamFailure(err)
		fail(w, status, code, msg)
		return
	}
	if h == nil || h.svc == nil {
		fail(w, 503, 50302, "任务进度暂不可用，请使用状态查询")
		return
	}
	controller := http.NewResponseController(w)
	// 没有可强制写期限的宿主不能开放长连接；不假设所有ResponseWriter都支持。
	if _, ok := w.(http.Flusher); !ok || controller.SetWriteDeadline(time.Time{}) != nil {
		fail(w, 503, 50302, "当前连接不支持进度推送，请使用状态查询")
		return
	}
	defer func() { _ = controller.SetWriteDeadline(time.Time{}) }() // 请求结束清除期限；失败不再写第二个响应。
	// Register强制经过原严格Bearer中间件，此处仅取同一已校验头供逐轮复核。
	_, token, _ := strings.Cut(r.Header.Get("Authorization"), " ")
	stream, err := h.svc.Open(r.Context(), token, uid, id, after)
	if err != nil {
		status, code, msg := streamFailure(err)
		fail(w, status, code, msg)
		return
	}
	defer stream.Close()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Accel-Buffering", "no")
	writeFailed := false
	writeFrame := func(frame string) error {
		deadline := time.Now().Add(3 * time.Second)
		if stream.Deadline().Before(deadline) {
			deadline = stream.Deadline()
		}
		if err := controller.SetWriteDeadline(deadline); err != nil {
			writeFailed = true
			return err
		}
		if _, err := fmt.Fprint(w, frame); err != nil {
			writeFailed = true
			return err
		}
		err := controller.Flush()
		writeFailed = err != nil
		return err
	}
	if err = writeFrame(": connected\n\n"); err != nil {
		return
	}
	err = stream.Run(func(event *dto.AITaskEvent) error {
		if event == nil {
			return writeFrame(": keep-alive\n\n")
		}
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return writeFrame("id: " + strconv.FormatInt(event.Seq, 10) + "\nevent: progress\ndata: " + string(raw) + "\n\n")
	})
	if err == nil || writeFailed || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || r.Context().Err() != nil {
		return
	}
	_, code, _ := streamFailure(err)
	// 已开始流之后只能给固定公开错误码，不回显错误、token或供应商文字。
	_ = writeFrame("event: error\ndata: {\"code\":" + strconv.Itoa(code) + "}\n\n") // 断连时无第二响应或日志输出。
}
