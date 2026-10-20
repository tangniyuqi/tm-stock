package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiTaskCreator interface {
	Quote(context.Context, int64, dto.AITaskQuoteInput) (dto.AITaskQuote, error)
	Create(context.Context, int64, string, dto.AITaskCreateInput) (dto.AITaskCreation, error)
	Receipt(context.Context, int64, string) (dto.AITaskCreation, error)
}
type AITaskCreateHandler struct{ svc aiTaskCreator }

func NewAITaskCreateHandler(svc aiTaskCreator) *AITaskCreateHandler { return &AITaskCreateHandler{svc} }

// Register 的只读核查与新建写入口独立；服务自身仍检查创建开关。
func (h *AITaskCreateHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler, writes bool) {
	register := func(pattern string, fn http.HandlerFunc) {
		next := auth(fn)
		mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Vary", "Authorization")
			next.ServeHTTP(w, r)
		}))
	}
	register("GET /api/v1/ai/task-receipt", h.receipt)
	if writes {
		register("POST /api/v1/ai/tasks/quote", h.quote)
		register("POST /api/v1/ai/tasks", h.create)
	}
}

func taskSubmissionBody(w http.ResponseWriter, r *http.Request, target any, allowed ...string) error {
	if r.URL.RawQuery != "" || r.URL.ForceQuery || len(r.Header.Values("Content-Encoding")) != 0 || r.Body == nil {
		return points.ErrInvalid
	}
	types := r.Header.Values("Content-Type")
	if len(types) != 1 {
		return points.ErrInvalid
	}
	media, params, err := mime.ParseMediaType(types[0])
	if err != nil || media != "application/json" || len(params) > 1 {
		return points.ErrInvalid
	}
	for k, v := range params {
		if k != "charset" || !strings.EqualFold(v, "utf-8") {
			return points.ErrInvalid
		}
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2048))
	if err != nil || task.CheckStageJSON(raw) != nil {
		return points.ErrInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(allowed) {
		return points.ErrInvalid
	}
	for _, name := range allowed {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return points.ErrInvalid
		}
	}
	if json.Unmarshal(raw, target) != nil {
		return points.ErrInvalid
	}
	return nil
}

func taskCreationKey(r *http.Request) (string, error) {
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !points.ValidKey(keys[0]) {
		return "", points.ErrInvalid
	}
	return keys[0], nil
}

func (h *AITaskCreateHandler) quote(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	var input dto.AITaskQuoteInput
	if err := taskSubmissionBody(w, r, &input, "analysis_type", "subject_type", "subject_id"); err != nil {
		taskSubmissionError(w, err)
		return
	}
	if h == nil || h.svc == nil {
		taskSubmissionError(w, task.ErrCreationDisabled)
		return
	}
	out, err := h.svc.Quote(r.Context(), uid, input)
	if err != nil {
		taskSubmissionError(w, err)
		return
	}
	ok(w, out)
}

func (h *AITaskCreateHandler) create(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	key, err := taskCreationKey(r)
	var input dto.AITaskCreateInput
	if err == nil {
		err = taskSubmissionBody(w, r, &input, "analysis_type", "subject_type", "subject_id", "price_version")
	}
	if err != nil {
		taskSubmissionError(w, err)
		return
	}
	if h == nil || h.svc == nil {
		taskSubmissionError(w, task.ErrCreationDisabled)
		return
	}
	out, err := h.svc.Create(r.Context(), uid, key, input)
	if err != nil {
		taskSubmissionError(w, err)
		return
	}
	ok(w, out)
}

func (h *AITaskCreateHandler) receipt(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	key, err := taskCreationKey(r)
	if err == nil && (r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 || len(r.TransferEncoding) != 0 || len(r.Header.Values("Content-Encoding")) != 0) {
		err = points.ErrInvalid
	}
	if err == nil && r.Body != nil {
		var raw []byte
		raw, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
		if err != nil || len(raw) != 0 {
			err = points.ErrInvalid
		}
	}
	if err != nil {
		taskSubmissionError(w, points.ErrInvalid)
		return
	}
	if h == nil || h.svc == nil {
		taskSubmissionError(w, task.ErrCreationDisabled)
		return
	}
	out, err := h.svc.Receipt(r.Context(), uid, key)
	if err != nil {
		taskSubmissionError(w, err)
		return
	}
	ok(w, out)
}

func taskSubmissionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, points.ErrInvalid):
		fail(w, 400, codeBadRequest, "任务请求参数无效")
	case errors.Is(err, points.ErrNotFound):
		fail(w, 404, 40403, "任务请求不存在")
	case errors.Is(err, points.ErrIdempotency):
		fail(w, 409, 40903, "原请求编号已用于不同任务，请先核查")
	case errors.Is(err, points.ErrPriceChanged):
		fail(w, 409, 40901, "价格版本已变化，请重新报价")
	case errors.Is(err, points.ErrInsufficient):
		fail(w, 422, 42201, "可用积分不足")
	case errors.Is(err, task.ErrAdmissionUnavailable):
		fail(w, 422, 42202, "该任务目前不可创建")
	default:
		fail(w, 503, 50305, "任务请求未确认，请先按原键核查")
	}
}
