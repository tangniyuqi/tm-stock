//go:build integration

package points_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// 仅测试：原handler写完并flush真实连接帧后取消本次读取，不改帧/任务/事件。
// Unwrap保留原ResponseController的写期限能力，不能用Recorder冒充流式宿主。
type h5EarlyEOFWriter struct {
	http.ResponseWriter
	stop func()
	once sync.Once
	cut  bool
}

func (w *h5EarlyEOFWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *h5EarlyEOFWriter) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
	w.once.Do(func() { w.cut = true; w.stop() })
}

func TestH5EarlyEOFWriter(t *testing.T) {
	original := httptest.NewRecorder()
	stops := 0
	w := &h5EarlyEOFWriter{ResponseWriter: original, stop: func() { stops++ }}
	frame := ": connected\n\n"
	if _, err := w.Write([]byte(frame)); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	w.Flush()
	if w.Unwrap() != original || original.Body.String() != frame || !original.Flushed || !w.cut || stops != 1 {
		t.Fatal("测试传输包装改写了原帧、未flush或重复中断")
	}
}
