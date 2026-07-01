package httpapi

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// sseWriter 封装 SSE 事件写出：每事件后 flush（ARCH-BACKEND-005）。
// 具体事件协议与续传在 M1 落地；此处仅提供底层写出原语。
type sseWriter struct {
	w       io.Writer
	flusher http.Flusher
}

// newSSEWriter 校验 ResponseWriter 是否支持 flush，并设置 SSE 响应头。
func newSSEWriter(c *gin.Context) (*sseWriter, error) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported")
	}
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	return &sseWriter{w: c.Writer, flusher: flusher}, nil
}

func (s *sseWriter) event(id, event, data string) error {
	if id != "" {
		if _, err := fmt.Fprintf(s.w, "id: %s\n", id); err != nil {
			return err
		}
	}
	if event != "" {
		if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// heartbeat 发送注释行心跳，保持连接活性（ARCH-BACKEND-005）。
func (s *sseWriter) heartbeat() error {
	if _, err := fmt.Fprint(s.w, ": ping\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
