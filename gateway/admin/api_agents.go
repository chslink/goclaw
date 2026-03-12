package admin

import (
	"net/http"
)

// handleListAgents 列出所有 Agent
func (h *AdminHandler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	// 1. 优先使用 AgentManager（常规模式）
	if h.agentMgr != nil {
		ids := h.agentMgr.ListAgents()
		agents := make([]map[string]interface{}, 0, len(ids))
		for _, id := range ids {
			info := h.agentMgr.GetAgentConfig(id)
			if info != nil {
				agents = append(agents, map[string]interface{}{
					"id":     info.ID,
					"name":   info.Name,
					"model":  info.Model,
					"status": info.Status,
				})
			} else {
				agents = append(agents, map[string]interface{}{
					"id":     id,
					"status": "unknown",
				})
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"agents": agents,
			"count":  len(agents),
		})
		return
	}

	// 2. Fallback: 使用 SwarmManager（swarm 模式）
	if h.swarmMgr != nil {
		ids := h.swarmMgr.ListAgents()
		// 尝试从 GetStatus 获取 agent 运行状态（flat 模式有 agents map）
		status := h.swarmMgr.GetStatus()
		agentStates, _ := status["agents"].(map[string]string)

		agents := make([]map[string]interface{}, 0, len(ids))
		for _, id := range ids {
			agentInfo := map[string]interface{}{
				"id":   id,
				"name": id,
			}
			if agentStates != nil {
				if s, ok := agentStates[id]; ok {
					agentInfo["status"] = s
				}
			}
			if agentInfo["status"] == nil {
				agentInfo["status"] = "running"
			}
			agents = append(agents, agentInfo)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"agents": agents,
			"count":  len(agents),
			"mode":   h.swarmMgr.GetMode(),
		})
		return
	}

	// 3. 都没有，返回空列表
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents": []interface{}{},
		"count":  0,
	})
}

// handleGetAgent 获取 Agent 详情
func (h *AdminHandler) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	// 1. 优先使用 AgentManager
	if h.agentMgr != nil {
		info := h.agentMgr.GetAgentConfig(id)
		if info == nil {
			writeError(w, http.StatusNotFound, "agent not found: "+id)
			return
		}

		result := map[string]interface{}{
			"id":     info.ID,
			"name":   info.Name,
			"model":  info.Model,
			"status": info.Status,
		}

		if toolsInfo, err := h.agentMgr.GetToolsInfo(); err == nil {
			result["tools_count"] = len(toolsInfo)
		}

		writeJSON(w, http.StatusOK, result)
		return
	}

	// 2. Fallback: SwarmManager
	if h.swarmMgr != nil {
		for _, agentID := range h.swarmMgr.ListAgents() {
			if agentID == id {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"id":     id,
					"name":   id,
					"status": "running",
					"mode":   h.swarmMgr.GetMode(),
				})
				return
			}
		}
	}

	writeError(w, http.StatusNotFound, "agent not found: "+id)
}
