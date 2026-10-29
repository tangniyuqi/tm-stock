//go:build integration

package points_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

// 仅测试：只复制原handler已经写出的完整公开帧，不构造任务/阶段/终态。
type h5StreamReplay struct {
	mu           sync.Mutex
	cursors      []int64
	frames       map[int64][]byte
	first        int64
	copies, cuts int
	invalid      bool
}

type h5StreamReplayWriter struct {
	http.ResponseWriter
	state           *h5StreamReplay
	after           int64
	first           bool
	buffer          []byte
	stop, connected func()
	cut             bool
}

func h5StreamReplayConfigurationValid(mode, origin, login, faults string) bool {
	if mode == "" {
		return true
	}
	_, err := h5TestOrigin(origin)
	return mode == "local-fixture-only" && err == nil && login == "local-fixture-only" && faults == ""
}

func TestH5StreamReplayConfiguration(t *testing.T) {
	for _, row := range []struct {
		name, mode, origin, login, faults string
		want                              bool
	}{
		{"默认关闭", "", "", "", "", true},
		{"显式隔离", "local-fixture-only", "http://127.0.0.1:5173", "local-fixture-only", "", true},
		{"未知模式", "on", "http://127.0.0.1:5173", "local-fixture-only", "", false},
		{"缺少源", "local-fixture-only", "", "local-fixture-only", "", false},
		{"外部源", "local-fixture-only", "https://example.test", "local-fixture-only", "", false},
		{"源带查询", "local-fixture-only", "http://127.0.0.1:5173?x=1", "local-fixture-only", "", false},
		{"缺原登录", "local-fixture-only", "http://127.0.0.1:5173", "", "", false},
		{"混合故障", "local-fixture-only", "http://127.0.0.1:5173", "local-fixture-only", "local-fixture-only", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if h5StreamReplayConfigurationValid(row.mode, row.origin, row.login, row.faults) != row.want {
				t.Fatal("复放测试配置没有失败关闭")
			}
		})
	}
}

func newH5StreamReplayWriter(w http.ResponseWriter, s *h5StreamReplay, after int64, stop, connected func()) *h5StreamReplayWriter {
	s.mu.Lock()
	s.cursors = append(s.cursors, after)
	first := len(s.cursors) == 1
	s.mu.Unlock()
	return &h5StreamReplayWriter{ResponseWriter: w, state: s, after: after, first: first, stop: stop, connected: connected}
}

func (w *h5StreamReplayWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *h5StreamReplayWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err == nil {
		if len(w.buffer)+n > 32768 {
			w.state.mu.Lock()
			w.state.invalid = true
			w.state.mu.Unlock()
			return n, errCheckTest
		}
		w.buffer = append(w.buffer, p...)
	}
	return n, err
}

func (w *h5StreamReplayWriter) copyFrame(raw []byte) bool {
	n, err := w.ResponseWriter.Write(raw)
	if err != nil || n != len(raw) {
		w.state.mu.Lock()
		w.state.invalid = true
		w.state.mu.Unlock()
		return false
	}
	w.state.mu.Lock()
	w.state.copies++
	w.state.mu.Unlock()
	return true
}

func (w *h5StreamReplayWriter) Flush() {
	raw := w.buffer
	w.buffer = nil
	if bytes.Equal(raw, []byte(": connected\n\n")) {
		if !w.first {
			w.state.mu.Lock()
			cached := bytes.Clone(w.state.frames[w.after])
			w.state.mu.Unlock()
			if len(cached) == 0 || !w.copyFrame(cached) {
				w.state.mu.Lock()
				w.state.invalid = true
				w.state.mu.Unlock()
				w.stop()
				return
			}
		}
		w.ResponseWriter.(http.Flusher).Flush()
		w.connected()
		return
	}
	if bytes.HasPrefix(raw, []byte("id: ")) {
		lines := strings.Split(string(raw), "\n")
		var event dto.AITaskEvent
		valid := len(lines) == 5 && lines[1] == "event: progress" && strings.HasPrefix(lines[2], "data: ") && lines[3] == "" && lines[4] == ""
		if valid {
			valid = json.Unmarshal([]byte(strings.TrimPrefix(lines[2], "data: ")), &event) == nil && event.Seq > w.after && lines[0] == "id: "+strconv.FormatInt(event.Seq, 10)
		}
		if !valid {
			w.state.mu.Lock()
			w.state.invalid = true
			w.state.mu.Unlock()
			w.stop()
			return
		}
		w.state.mu.Lock()
		if w.state.frames == nil {
			w.state.frames = map[int64][]byte{}
		}
		if previous, ok := w.state.frames[event.Seq]; ok && !bytes.Equal(previous, raw) {
			w.state.invalid = true
		}
		w.state.frames[event.Seq] = bytes.Clone(raw)
		if w.first && w.state.first == 0 {
			w.state.first = event.Seq
		}
		w.state.mu.Unlock()
		if w.first && !w.cut {
			if !w.copyFrame(raw) {
				w.stop()
				return
			}
			w.ResponseWriter.(http.Flusher).Flush()
			w.cut = true
			w.state.mu.Lock()
			w.state.cuts++
			w.state.mu.Unlock()
			w.stop()
			return
		}
	}
	w.ResponseWriter.(http.Flusher).Flush()
}

func TestH5StreamReplayWriter(t *testing.T) {
	s := &h5StreamReplay{}
	frame := func(seq int64) []byte {
		raw, err := json.Marshal(dto.AITaskEvent{Seq: seq, Type: "claimed", Status: 2, At: 1})
		if err != nil {
			t.Fatal(err)
		}
		return []byte("id: " + strconv.FormatInt(seq, 10) + "\nevent: progress\ndata: " + string(raw) + "\n\n")
	}
	connected := []byte(": connected\n\n")
	event := frame(2)
	next := frame(3)
	stops, starts, finishes := 0, 0, 0
	one := httptest.NewRecorder()
	first := newH5StreamReplayWriter(one, s, 1, func() { stops++ }, func() { starts++ })
	for _, raw := range [][]byte{connected, event} {
		if _, err := first.Write(raw); err != nil {
			t.Fatal(err)
		}
		first.Flush()
	}
	two := httptest.NewRecorder()
	second := newH5StreamReplayWriter(two, s, 2, func() { stops++ }, func() { finishes++ })
	for _, raw := range [][]byte{connected, next} {
		if _, err := second.Write(raw); err != nil {
			t.Fatal(err)
		}
		second.Flush()
	}
	if first.Unwrap() != one || second.Unwrap() != two || !bytes.Equal(one.Body.Bytes(), bytes.Join([][]byte{connected, event, event}, nil)) || !bytes.Equal(two.Body.Bytes(), bytes.Join([][]byte{connected, event, next}, nil)) || stops != 1 || starts != 1 || finishes != 1 || s.copies != 2 || s.cuts != 1 || s.invalid || s.first != 2 || len(s.frames) != 2 || len(s.cursors) != 2 || s.cursors[1] != 2 {
		t.Fatal("测试包装没有逐字复放原帧、保留宿主能力或按原确认游标恢复")
	}
	for _, row := range []struct {
		name string
		raw  []byte
	}{
		{"不完整原帧", event[:len(event)-1]},
		{"错误ID", bytes.Replace(event, []byte("id: 2"), []byte("id: 3"), 1)},
	} {
		t.Run(row.name, func(t *testing.T) {
			bad := &h5StreamReplay{}
			ended := 0
			writer := newH5StreamReplayWriter(httptest.NewRecorder(), bad, 1, func() { ended++ }, func() {})
			if _, err := writer.Write(row.raw); err != nil {
				t.Fatal(err)
			}
			writer.Flush()
			if !bad.invalid || ended != 1 || bad.copies != 0 {
				t.Fatal("无效原帧被冒充为复放证据")
			}
		})
	}
	t.Run("未缓存游标", func(t *testing.T) {
		bad := &h5StreamReplay{cursors: []int64{1}}
		ended := 0
		writer := newH5StreamReplayWriter(httptest.NewRecorder(), bad, 2, func() { ended++ }, func() { t.Error("未有原帧不能推进Agent") })
		if _, err := writer.Write(connected); err != nil {
			t.Fatal(err)
		}
		writer.Flush()
		if !bad.invalid || ended != 1 || bad.copies != 0 {
			t.Fatal("没有原帧却产生复放")
		}
	})
}
