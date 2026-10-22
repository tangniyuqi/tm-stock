// Package feedbackconsole 仅转接内部反馈接口；不读取模型配置或直接修改账本/反馈表。
package feedbackconsole

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
)

var (
	ErrInvalid     = errors.New("反馈参数无效")
	ErrForbidden   = errors.New("当前账号未登记为反馈处理人")
	ErrUnconfirmed = errors.New("操作未确认，请查询原受理单")
	ErrNotFound    = errors.New("反馈不存在")
	ErrConflict    = errors.New("反馈已变化，请重新查询")
)

type Record struct {
	ID         int64  `json:"id"`
	TaskID     int64  `json:"taskId"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	Version    int64  `json:"version"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}
type Event struct {
	Version    int64  `json:"version"`
	ActorID    int64  `json:"actorId"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	At         int64  `json:"at"`
}
type Detail struct {
	Item   Record  `json:"item"`
	Events []Event `json:"events"`
}
type Page struct {
	Items       []Record `json:"items"`
	NextAfterID int64    `json:"nextAfterId"`
	HasMore     bool     `json:"hasMore"`
}
type Decision struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Status          string `json:"status"`
	Resolution      string `json:"resolution"`
}

func ID(n int64) bool { return n > 0 && n <= 9007199254740991 }
func State(s, r string, v int64) bool {
	return s == "received" && r == "" && v == 1 || s == "processing" && r == "" && v == 2 || s == "resolved" && (r == "reviewed" || r == "referred") && v == 3 || s == "dismissed" && (r == "unconfirmed" || r == "not_applicable") && v == 3
}
func (r Record) Valid() bool {
	return ID(r.ID) && ID(r.TaskID) && (r.Category == "fact" || r.Category == "source" || r.Category == "display" || r.Category == "other") && State(r.Status, r.Resolution, r.Version) && r.CreatedAt > 0 && r.UpdatedAt >= r.CreatedAt && r.UpdatedAt <= 8640000000000000 && (r.Version != 1 || r.CreatedAt == r.UpdatedAt)
}
func (d Decision) Valid() bool {
	return d.ExpectedVersion >= 1 && d.ExpectedVersion <= 2 && State(d.Status, d.Resolution, d.ExpectedVersion+1)
}

// Decode 先拒绝所有层级重复键，再做类型/未知字段/尾随文档检查。
func Decode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		if tok == json.Delim('{') {
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return ErrInvalid
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		if tok == json.Delim('[') {
			for d.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := walk(); err != nil {
		return ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInvalid
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(out); err != nil {
		return ErrInvalid
	}
	return nil
}
func DecodeDecision(raw []byte) (Decision, error) {
	var d Decision
	if Decode(raw, &d) != nil {
		return d, ErrInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 3 {
		return d, ErrInvalid
	}
	for _, k := range []string{"expectedVersion", "status", "resolution"} {
		if fields[k] == nil || bytes.Equal(fields[k], []byte("null")) {
			return d, ErrInvalid
		}
	}
	if !d.Valid() {
		return d, ErrInvalid
	}
	return d, nil
}

type Identity interface {
	Active(context.Context, int64) (bool, error)
}
type Client struct {
	cfg      config.FeedbackConsoleConfig
	http     *http.Client
	identity Identity
}

func New(c config.FeedbackConsoleConfig, identity Identity) (*Client, error) {
	if !c.Enabled || c.Validate() != nil || identity == nil {
		return nil, config.ErrFeedbackConsoleConfig
	}
	c.Reviewers = append([]config.FeedbackConsoleReviewer(nil), c.Reviewers...)
	return &Client{c, &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 16384}}, identity}, nil
}
func (c *Client) Close() {
	if c != nil {
		c.http.CloseIdleConnections()
	}
}
func (c *Client) WebOrigin() string { return c.cfg.WebOrigin }
func (c *Client) reviewer(uid int64) (config.FeedbackConsoleReviewer, error) {
	for _, r := range c.cfg.Reviewers {
		if r.UserID == uid {
			return r, nil
		}
	}
	return config.FeedbackConsoleReviewer{}, ErrForbidden
}
func (c *Client) call(ctx context.Context, uid int64, method, path string, body []byte, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	r, err := c.reviewer(uid)
	if err != nil {
		return err
	}
	active, err := c.identity.Active(ctx, uid)
	if err != nil {
		return ErrUnconfirmed
	}
	if !active {
		return ErrForbidden
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.Upstream+path, bytes.NewReader(body))
	if err != nil {
		return ErrInvalid
	}
	req.Header.Set("Authorization", "Bearer "+r.Token)
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return ErrUnconfirmed
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return ErrUnconfirmed
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if Decode(raw, &envelope) != nil {
		return ErrUnconfirmed
	}
	if res.StatusCode != 200 || envelope.Code != 0 {
		switch {
		case res.StatusCode == 404 && envelope.Code == 40403:
			return ErrNotFound
		case res.StatusCode == 409 && envelope.Code == 40904:
			return ErrConflict
		case res.StatusCode == 401:
			return ErrForbidden
		default:
			return ErrUnconfirmed
		}
	}
	if len(envelope.Data) == 0 || bytes.Equal(envelope.Data, []byte("null")) || Decode(envelope.Data, out) != nil {
		return ErrUnconfirmed
	}
	return nil
}
func (c *Client) List(ctx context.Context, uid, after int64, limit int) (Page, error) {
	var p Page
	if after < 0 || after > 9007199254740991 || limit < 1 || limit > 100 {
		return p, ErrInvalid
	}
	path := "/internal/v1/ai-feedback?limit=" + strconv.Itoa(limit)
	if after > 0 {
		path += "&after_id=" + strconv.FormatInt(after, 10)
	}
	if err := c.call(ctx, uid, "GET", path, nil, &p); err != nil {
		return Page{}, err
	}
	if p.Items == nil || len(p.Items) > limit {
		return Page{}, ErrUnconfirmed
	}
	prev := after
	for _, r := range p.Items {
		if !r.Valid() || r.ID <= prev {
			return Page{}, ErrUnconfirmed
		}
		prev = r.ID
	}
	if p.HasMore && (len(p.Items) == 0 || p.NextAfterID != prev) || !p.HasMore && p.NextAfterID != 0 {
		return Page{}, ErrUnconfirmed
	}
	return p, nil
}
func (c *Client) Detail(ctx context.Context, uid, id int64) (Detail, error) {
	var d Detail
	if !ID(id) {
		return d, ErrInvalid
	}
	if err := c.call(ctx, uid, "GET", "/internal/v1/ai-feedback/"+strconv.FormatInt(id, 10), nil, &d); err != nil {
		return Detail{}, err
	}
	if !d.Item.Valid() || d.Item.ID != id || int64(len(d.Events)) != d.Item.Version {
		return Detail{}, ErrUnconfirmed
	}
	for i, e := range d.Events {
		if e.Version != int64(i+1) || !State(e.Status, e.Resolution, e.Version) || e.At < d.Item.CreatedAt || e.At > d.Item.UpdatedAt || i == 0 && (e.ActorID != 0 || e.At != d.Item.CreatedAt) || i > 0 && (!ID(e.ActorID) || e.At < d.Events[i-1].At) {
			return Detail{}, ErrUnconfirmed
		}
	}
	last := d.Events[len(d.Events)-1]
	if last.Status != d.Item.Status || last.Resolution != d.Item.Resolution || last.At != d.Item.UpdatedAt {
		return Detail{}, ErrUnconfirmed
	}
	return d, nil
}
func (c *Client) Decide(ctx context.Context, uid, id int64, d Decision) (Record, error) {
	var r Record
	if !ID(id) || !d.Valid() {
		return r, ErrInvalid
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return r, ErrInvalid
	}
	if err = c.call(ctx, uid, "POST", "/internal/v1/ai-feedback/"+strconv.FormatInt(id, 10)+"/decisions", raw, &r); err != nil {
		return Record{}, err
	}
	if !r.Valid() || r.ID != id || r.Version < d.ExpectedVersion+1 {
		return Record{}, ErrUnconfirmed
	}
	// 原接口将凭据绑定actor；用其不可变事件核对登记，不能把错误绑定的回执称为成功。
	current, err := c.Detail(ctx, uid, id)
	if err != nil || int64(len(current.Events)) <= d.ExpectedVersion {
		return Record{}, ErrUnconfirmed
	}
	actor, err := c.reviewer(uid)
	if err != nil {
		return Record{}, ErrUnconfirmed
	}
	e := current.Events[d.ExpectedVersion]
	if current.Item.TaskID != r.TaskID || current.Item.Category != r.Category || e.ActorID != actor.ActorID || e.Status != d.Status || e.Resolution != d.Resolution {
		return Record{}, ErrUnconfirmed
	}
	return current.Item, nil
}
