package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type aiLibraryService interface {
	RegisterSource(context.Context, int64, sources.Source) (sources.Source, error)
	SourceByID(context.Context, int64) (sources.Source, error)
	EnableSource(context.Context, int64, int64, bool) error
	ImportText(context.Context, int64, int64, string, string, []byte) (sources.Snapshot, error)
	FetchSnapshot(context.Context, int64, int64, string) (sources.Snapshot, error)
	SourceAudits(context.Context, int64, int64, int) ([]sources.SourceAudit, error)
	SnapshotByID(context.Context, int64) (sources.Snapshot, error)
}
type aiL0Service interface {
	Extract(context.Context, int64, pipeline.RunRequest) (pipeline.RunRecord, error)
	RunByRequest(context.Context, int64, string) (pipeline.RunRecord, error)
}
type AILibraryHandler struct {
	library    aiLibraryService
	extraction aiL0Service
}

func NewAILibraryHandler(library aiLibraryService, extraction aiL0Service) *AILibraryHandler {
	return &AILibraryHandler{library: library, extraction: extraction}
}

func (h *AILibraryHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/v1/ai-sources", h.register)
	mux.HandleFunc("GET /internal/v1/ai-sources/{id}", h.source)
	mux.HandleFunc("GET /internal/v1/ai-sources/{id}/audits", h.sourceAudits)
	mux.HandleFunc("POST /internal/v1/ai-sources/{id}/state", h.sourceState)
	mux.HandleFunc("POST /internal/v1/ai-snapshot-imports", h.importText)
	mux.HandleFunc("POST /internal/v1/ai-snapshot-fetches", h.fetch)
	mux.HandleFunc("GET /internal/v1/ai-snapshots/{id}", h.snapshot)
	mux.HandleFunc("POST /internal/v1/ai-extractions", h.extract)
	mux.HandleFunc("GET /internal/v1/ai-extractions/{request_id}", h.extractionRun)
}
func (h *AILibraryHandler) authorized(w http.ResponseWriter, r *http.Request) bool {
	if middleware.AIAdminActor(r.Context()) <= 0 {
		fail(w, 401, codeUnauthorized, "需要内部管理身份")
		return false
	}
	return true
}
func (h *AILibraryHandler) decode(w http.ResponseWriter, r *http.Request, value any, limit int64) bool {
	if !h.authorized(w, r) {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		h.failure(w, service.ErrInvalidLibraryRequest)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		h.failure(w, service.ErrInvalidLibraryRequest)
		return false
	}
	return true
}
func (h *AILibraryHandler) id(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if !h.authorized(w, r) {
		return 0, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.failure(w, service.ErrInvalidLibraryRequest)
		return 0, false
	}
	return id, true
}
func (h *AILibraryHandler) register(w http.ResponseWriter, r *http.Request) {
	var request dto.AISourceRegistration
	if !h.decode(w, r, &request, 8192) {
		return
	}
	if request.MinIntervalMS < 0 || request.MinIntervalMS > 86400000 || (request.LicenseExpiresAt != nil && (*request.LicenseExpiresAt <= 0 || *request.LicenseExpiresAt > 253402300799999)) {
		h.failure(w, service.ErrInvalidLibraryRequest)
		return
	}
	s := sources.Source{Domain: request.Domain, Category: request.Category, LicenseTag: request.LicenseTag, LicenseRef: request.LicenseRef, MinInterval: time.Duration(request.MinIntervalMS) * time.Millisecond}
	if request.LicenseExpiresAt != nil {
		value := time.UnixMilli(*request.LicenseExpiresAt).UTC()
		s.LicenseExpires = &value
	}
	created, err := h.library.RegisterSource(r.Context(), middleware.AIAdminActor(r.Context()), s)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAISourceRecord(created))
}
func (h *AILibraryHandler) source(w http.ResponseWriter, r *http.Request) {
	id, valid := h.id(w, r)
	if !valid {
		return
	}
	s, err := h.library.SourceByID(r.Context(), id)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAISourceRecord(s))
}
func (h *AILibraryHandler) sourceState(w http.ResponseWriter, r *http.Request) {
	id, valid := h.id(w, r)
	if !valid {
		return
	}
	var request struct {
		Enabled *bool `json:"enabled"`
	}
	if !h.decode(w, r, &request, 1024) {
		return
	}
	if request.Enabled == nil {
		h.failure(w, service.ErrInvalidLibraryRequest)
		return
	}
	if err := h.library.EnableSource(r.Context(), middleware.AIAdminActor(r.Context()), id, *request.Enabled); err != nil {
		h.failure(w, err)
		return
	}
	ok(w, map[string]any{"id": id, "enabled": *request.Enabled})
}
func (h *AILibraryHandler) importText(w http.ResponseWriter, r *http.Request) {
	var request dto.AISnapshotImport
	if !h.decode(w, r, &request, 3<<20) {
		return
	}
	s, err := h.library.ImportText(r.Context(), middleware.AIAdminActor(r.Context()), request.SourceID, request.URL, request.MediaType, request.Body)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAISnapshotRecord(s))
}
func (h *AILibraryHandler) fetch(w http.ResponseWriter, r *http.Request) {
	var request dto.AISnapshotFetch
	if !h.decode(w, r, &request, 8192) {
		return
	}
	s, err := h.library.FetchSnapshot(r.Context(), middleware.AIAdminActor(r.Context()), request.SourceID, request.URL)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAISnapshotRecord(s))
}
func (h *AILibraryHandler) snapshot(w http.ResponseWriter, r *http.Request) {
	id, valid := h.id(w, r)
	if !valid {
		return
	}
	s, err := h.library.SnapshotByID(r.Context(), id)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAISnapshotRecord(s))
}

func (h *AILibraryHandler) sourceAudits(w http.ResponseWriter, r *http.Request) {
	id, valid := h.id(w, r)
	if !valid {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		h.failure(w, service.ErrInvalidLibraryRequest)
		return
	}
	for key, values := range query {
		if (key != "after_id" && key != "limit") || len(values) != 1 || values[0] == "" {
			h.failure(w, service.ErrInvalidLibraryRequest)
			return
		}
	}
	after, limit := int64(0), 100
	if query.Has("after_id") {
		after, err = strconv.ParseInt(query.Get("after_id"), 10, 64)
		if err != nil || after < 0 {
			h.failure(w, service.ErrInvalidLibraryRequest)
			return
		}
	}
	if query.Has("limit") {
		limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil || limit < 1 || limit > 200 {
			h.failure(w, service.ErrInvalidLibraryRequest)
			return
		}
	}
	events, err := h.library.SourceAudits(r.Context(), id, after, limit)
	if err != nil {
		h.failure(w, err)
		return
	}
	out := dto.AISourceAudits{Items: []dto.AISourceAudit{}, NextAfterID: after}
	for _, event := range events {
		out.Items = append(out.Items, dto.NewAISourceAudit(event))
		out.NextAfterID = event.ID
	}
	ok(w, out)
}
func (h *AILibraryHandler) extract(w http.ResponseWriter, r *http.Request) {
	var request pipeline.RunRequest
	if !h.decode(w, r, &request, 8192) {
		return
	}
	record, err := h.extraction.Extract(r.Context(), middleware.AIAdminActor(r.Context()), request)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAIL0RunRecord(record))
}
func (h *AILibraryHandler) extractionRun(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	record, err := h.extraction.RunByRequest(r.Context(), middleware.AIAdminActor(r.Context()), r.PathValue("request_id"))
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAIL0RunRecord(record))
}
func (h *AILibraryHandler) failure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidLibraryRequest), errors.Is(err, pipeline.ErrInvalidRun):
		fail(w, 400, codeBadRequest, "内部资料参数无效")
	case errors.Is(err, sources.ErrSourceUnavailable), errors.Is(err, pipeline.ErrRunNotFound):
		fail(w, 404, 40404, "内部资料或抽取记录不存在")
	case errors.Is(err, pipeline.ErrDisabled):
		fail(w, 503, 50301, "内部抽取尚未获得启用配置")
	case errors.Is(err, pipeline.ErrRunConflict), errors.Is(err, pipeline.ErrRunBusy):
		fail(w, 409, 40902, "抽取请求冲突或结果待核对，请保留原请求编号")
	case errors.Is(err, sources.ErrUnauthorized), errors.Is(err, sources.ErrUnsafeURL), errors.Is(err, sources.ErrUnsupportedMedia):
		fail(w, 422, 42202, "来源授权、链接或资料类型未通过校验")
	case errors.Is(err, sources.ErrBodyTooLarge):
		fail(w, 413, 41301, "资料正文超过大小限制")
	default:
		fail(w, 500, codeInternal, "内部资料服务暂不可用")
	}
}
