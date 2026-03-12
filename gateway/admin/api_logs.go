package admin

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// handleLogsStream SSE 实时日志流
func (h *AdminHandler) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	// 设置 SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// 获取日志级别过滤
	levelFilter := r.URL.Query().Get("level")

	// 先发送历史日志
	history := h.logBuffer.Recent(200)
	for _, entry := range history {
		if levelFilter != "" && entry.Level != levelFilter {
			continue
		}
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// 订阅新日志
	subID := uuid.New().String()
	ch := h.logBuffer.Subscribe(subID)
	defer h.logBuffer.Unsubscribe(subID)

	// 实时推送
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			if levelFilter != "" && entry.Level != levelFilter {
				continue
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
