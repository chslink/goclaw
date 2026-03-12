package admin

import (
	"net/http"
)

// handleListChannels 列出所有通道及状态
func (h *AdminHandler) handleListChannels(w http.ResponseWriter, r *http.Request) {
	if h.channelMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"channels": []interface{}{},
			"count":    0,
		})
		return
	}

	names := h.channelMgr.List()
	channelsList := make([]map[string]interface{}, 0, len(names))

	for _, name := range names {
		status, _ := h.channelMgr.Status(name)
		if status != nil {
			channelsList = append(channelsList, status)
		} else {
			channelsList = append(channelsList, map[string]interface{}{
				"name":   name,
				"status": "unknown",
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"channels": channelsList,
		"count":    len(channelsList),
	})
}
