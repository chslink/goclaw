package admin

import (
	"net/http"
)

// handleListSessions 列出所有会话
func (h *AdminHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if h.sessionMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sessions": []interface{}{},
			"count":    0,
		})
		return
	}

	keys, err := h.sessionMgr.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions: "+err.Error())
		return
	}

	sessions := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		sess, err := h.sessionMgr.GetOrCreate(key)
		if err != nil {
			continue
		}
		sessions = append(sessions, map[string]interface{}{
			"key":           sess.Key,
			"message_count": len(sess.Messages),
			"created_at":    sess.CreatedAt,
			"updated_at":    sess.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// handleGetSession 获取会话详情
func (h *AdminHandler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "session key is required")
		return
	}

	if h.sessionMgr == nil {
		writeError(w, http.StatusNotFound, "session manager not available")
		return
	}

	sess, err := h.sessionMgr.GetOrCreate(key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get session: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"key":        sess.Key,
		"messages":   sess.Messages,
		"created_at": sess.CreatedAt,
		"updated_at": sess.UpdatedAt,
		"metadata":   sess.Metadata,
	})
}

// handleDeleteSession 清空会话
func (h *AdminHandler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "session key is required")
		return
	}

	if h.sessionMgr == nil {
		writeError(w, http.StatusNotFound, "session manager not available")
		return
	}

	sess, err := h.sessionMgr.GetOrCreate(key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get session: "+err.Error())
		return
	}

	sess.Clear()

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "cleared",
		"key":    key,
	})
}
