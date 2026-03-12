package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// handleListSwarms 列出所有可用蜂群配置（从 ~/.goclaw/swarms/ 扫描）
func (h *AdminHandler) handleListSwarms(w http.ResponseWriter, r *http.Request) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get home directory")
		return
	}

	swarmDir := filepath.Join(homeDir, ".goclaw", "swarms")
	entries, err := os.ReadDir(swarmDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"swarms": []interface{}{},
				"count":  0,
				"active": h.swarmMgr != nil,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read swarms directory")
		return
	}

	swarms := make([]map[string]interface{}, 0)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		configPath := filepath.Join(swarmDir, entry.Name())

		data, err := os.ReadFile(configPath)
		if err != nil {
			swarms = append(swarms, map[string]interface{}{
				"name":  name,
				"error": "failed to read config",
			})
			continue
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			swarms = append(swarms, map[string]interface{}{
				"name":  name,
				"error": "invalid JSON",
			})
			continue
		}

		info := map[string]interface{}{
			"name":        name,
			"mode":        cfg["mode"],
			"description": cfg["description"],
		}

		// flat 模式额外信息
		if agentIDs, ok := cfg["agent_ids"].([]interface{}); ok {
			info["agent_count"] = len(agentIDs)
		}
		if flows, ok := cfg["flows"].([]interface{}); ok {
			info["flow_count"] = len(flows)
		}

		// corporate 模式额外信息
		if secretary, ok := cfg["secretary"].(map[string]interface{}); ok {
			info["secretary"] = secretary["agent_id"]
		}
		if hr, ok := cfg["hr"].(map[string]interface{}); ok {
			info["hr"] = hr["agent_id"]
		}
		if pm, ok := cfg["pm"].(map[string]interface{}); ok {
			info["pm"] = pm["agent_id"]
		}

		swarms = append(swarms, info)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"swarms": swarms,
		"count":  len(swarms),
		"active": h.swarmMgr != nil,
	})
}

// handleActiveSwarm 获取活跃蜂群状态
func (h *AdminHandler) handleActiveSwarm(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"active": false,
		})
		return
	}

	status := h.swarmMgr.GetStatus()
	status["active"] = true
	status["mode"] = h.swarmMgr.GetMode()
	status["agents"] = h.swarmMgr.ListAgents()

	writeJSON(w, http.StatusOK, status)
}

// handleSwarmTasks 获取蜂群任务看板（corporate 模式）
func (h *AdminHandler) handleSwarmTasks(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tasks": []interface{}{},
			"count": 0,
		})
		return
	}

	tasks := h.swarmMgr.GetTasks()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// handleSwarmApprovals 获取蜂群审批列表
func (h *AdminHandler) handleSwarmApprovals(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"pending":  []interface{}{},
			"resolved": []interface{}{},
		})
		return
	}

	pending, resolved := h.swarmMgr.GetApprovals()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pending":       pending,
		"resolved":      resolved,
		"pending_count": len(pending),
	})
}

// handleSwarmApprove 批准审批请求
func (h *AdminHandler) handleSwarmApprove(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "no active swarm")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "approval id is required")
		return
	}

	if err := h.swarmMgr.ApproveRequest(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "approved",
		"id":     id,
	})
}

// handleSwarmReject 驳回审批请求
func (h *AdminHandler) handleSwarmReject(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "no active swarm")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "approval id is required")
		return
	}

	// 读取可选的 reason
	reason := ""
	if r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		if len(body) > 0 {
			var req struct {
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(body, &req); err == nil {
				reason = req.Reason
			}
		}
	}

	if err := h.swarmMgr.RejectRequest(id, reason); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reject: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "rejected",
		"id":     id,
	})
}

// handleSwarmMessages 获取 Agent 间的沟通记录
func (h *AdminHandler) handleSwarmMessages(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"messages": []interface{}{},
			"count":    0,
		})
		return
	}

	// 支持 ?limit=N 参数，默认 100
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	messages := h.swarmMgr.GetMessages(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

// handleSwarmAddAgent 动态添加 agent 到活跃蜂群
func (h *AdminHandler) handleSwarmAddAgent(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "no active swarm")
		return
	}

	var req struct {
		AgentID   string `json:"agent_id"`
		Workspace string `json:"workspace"`
		Model     string `json:"model"`
	}

	if r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
				return
			}
		}
	}

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	if err := h.swarmMgr.AddAgent(r.Context(), req.AgentID, req.Workspace, req.Model); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add agent: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "added",
		"agent_id": req.AgentID,
		"agents":   h.swarmMgr.ListAgents(),
	})
}

// handleSwarmRemoveAgent 从活跃蜂群移除 agent
func (h *AdminHandler) handleSwarmRemoveAgent(w http.ResponseWriter, r *http.Request) {
	if h.swarmMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "no active swarm")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	if err := h.swarmMgr.RemoveAgent(agentID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove agent: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "removed",
		"agent_id": agentID,
		"agents":   h.swarmMgr.ListAgents(),
	})
}
