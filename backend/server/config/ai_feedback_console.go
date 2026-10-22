package config

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
)

var ErrFeedbackConsoleConfig = errors.New("反馈后台配置无效")

type FeedbackConsoleReviewer struct {
	UserID  int64  `json:"userId"`
	ActorID int64  `json:"actorId"`
	Token   string `json:"token"`
}
type FeedbackConsoleConfig struct {
	Enabled             bool
	Upstream, WebOrigin string
	Reviewers           []FeedbackConsoleReviewer
}

func (FeedbackConsoleConfig) String() string       { return "反馈后台配置（凭据已隐藏）" }
func (c FeedbackConsoleConfig) GoString() string   { return c.String() }
func (FeedbackConsoleReviewer) String() string     { return "反馈处理身份（凭据已隐藏）" }
func (c FeedbackConsoleReviewer) GoString() string { return c.String() }
func (c FeedbackConsoleConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	u, err := url.Parse(c.Upstream)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Path != "" || u.String() != c.Upstream || u.Port() == "" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		return ErrFeedbackConsoleConfig
	}
	if port, err := net.LookupPort("tcp", u.Port()); err != nil || port < 1 || port > 65535 {
		return ErrFeedbackConsoleConfig
	}
	w, err := url.Parse(c.WebOrigin)
	if err != nil || w.Scheme != "https" && w.Scheme != "http" || w.Host == "" || w.User != nil || w.RawQuery != "" || w.ForceQuery || w.Fragment != "" || w.Path != "" || w.String() != c.WebOrigin || len(c.Reviewers) == 0 || len(c.Reviewers) > 100 {
		return ErrFeedbackConsoleConfig
	}
	if w.Scheme == "http" && (net.ParseIP(w.Hostname()) == nil || !net.ParseIP(w.Hostname()).IsLoopback()) {
		return ErrFeedbackConsoleConfig
	}
	users, actors, tokens := map[int64]bool{}, map[int64]bool{}, map[string]bool{}
	for _, r := range c.Reviewers {
		if r.UserID <= 0 || r.ActorID <= 0 || r.UserID > 9007199254740991 || r.ActorID > 9007199254740991 || users[r.UserID] || actors[r.ActorID] || tokens[r.Token] || len(r.Token) < 32 || len(r.Token) > 256 {
			return ErrFeedbackConsoleConfig
		}
		for _, b := range []byte(r.Token) {
			if b < 33 || b > 126 {
				return ErrFeedbackConsoleConfig
			}
		}
		users[r.UserID] = true
		actors[r.ActorID] = true
		tokens[r.Token] = true
	}
	return nil
}
func LoadFeedbackConsole() (FeedbackConsoleConfig, error) {
	var c FeedbackConsoleConfig
	switch os.Getenv("TM_AI_FEEDBACK_CONSOLE_ENABLED") {
	case "", "false":
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, ErrFeedbackConsoleConfig
	}
	c.Upstream = os.Getenv("TM_AI_FEEDBACK_CONSOLE_UPSTREAM")
	c.WebOrigin = os.Getenv("TM_AI_FEEDBACK_CONSOLE_WEB_ORIGIN")
	d := json.NewDecoder(strings.NewReader(os.Getenv("TM_AI_FEEDBACK_CONSOLE_REVIEWERS")))
	d.DisallowUnknownFields()
	if err := d.Decode(&c.Reviewers); err != nil {
		return FeedbackConsoleConfig{}, ErrFeedbackConsoleConfig
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return FeedbackConsoleConfig{}, ErrFeedbackConsoleConfig
	}
	if err := c.Validate(); err != nil {
		return FeedbackConsoleConfig{}, err
	}
	return c, nil
}
