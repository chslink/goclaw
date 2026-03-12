package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// allowedFiles 允许读写的工作区文件白名单
var allowedFiles = map[string]bool{
	"IDENTITY.md": true,
	"MEMORY.md":   true,
	"SOUL.md":     true,
	"USER.md":     true,
	"AGENTS.md":   true,
}

// getAgentWorkspace 从 agentMgr 或 swarmMgr 获取 agent workspace 路径
func (h *AdminHandler) getAgentWorkspace(id string) string {
	if h.agentMgr != nil {
		if ws := h.agentMgr.GetAgentWorkspace(id); ws != "" {
			return ws
		}
	}
	if h.swarmMgr != nil {
		if ws := h.swarmMgr.GetAgentWorkspace(id); ws != "" {
			return ws
		}
	}
	return ""
}

// handleListAgentFiles 列出 agent 工作区文件
func (h *AdminHandler) handleListAgentFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	workspace := h.getAgentWorkspace(id)
	if workspace == "" {
		writeError(w, http.StatusNotFound, "agent not found or no workspace: "+id)
		return
	}

	files := make([]map[string]interface{}, 0)
	for name := range allowedFiles {
		filePath := filepath.Join(workspace, name)
		// memory 目录下的文件
		if name == "MEMORY.md" {
			filePath = filepath.Join(workspace, "memory", name)
		}
		info, err := os.Stat(filePath)
		exists := err == nil
		entry := map[string]interface{}{
			"name":   name,
			"exists": exists,
		}
		if exists {
			entry["size"] = info.Size()
			entry["modified"] = info.ModTime()
		}
		files = append(files, entry)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id":  id,
		"workspace": workspace,
		"files":     files,
	})
}

// resolveFilePath 解析文件路径，MEMORY.md 在 memory/ 子目录下
func resolveFilePath(workspace, name string) string {
	if name == "MEMORY.md" {
		return filepath.Join(workspace, "memory", name)
	}
	return filepath.Join(workspace, name)
}

// handleGetAgentFile 读取 agent 工作区文件
func (h *AdminHandler) handleGetAgentFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	if id == "" || name == "" {
		writeError(w, http.StatusBadRequest, "agent id and file name are required")
		return
	}

	// 安全检查：只允许白名单文件
	if !allowedFiles[name] {
		writeError(w, http.StatusBadRequest, "file not allowed: "+name)
		return
	}

	workspace := h.getAgentWorkspace(id)
	if workspace == "" {
		writeError(w, http.StatusNotFound, "agent not found or no workspace: "+id)
		return
	}

	filePath := resolveFilePath(workspace, name)

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空内容（前端可以创建）
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"name":    name,
				"content": "",
				"exists":  false,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read file: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"content": string(content),
		"exists":  true,
	})
}

// handlePutAgentFile 写入 agent 工作区文件
func (h *AdminHandler) handlePutAgentFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	if id == "" || name == "" {
		writeError(w, http.StatusBadRequest, "agent id and file name are required")
		return
	}

	if !allowedFiles[name] {
		writeError(w, http.StatusBadRequest, "file not allowed: "+name)
		return
	}

	workspace := h.getAgentWorkspace(id)
	if workspace == "" {
		writeError(w, http.StatusNotFound, "agent not found or no workspace: "+id)
		return
	}

	// 读取请求体
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB 限制
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// 解析 content 字段（支持 JSON 格式 {"content":"..."} 和纯文本）
	content := string(body)
	if strings.HasPrefix(strings.TrimSpace(content), "{") {
		// 尝试作为 JSON 解析
		var req struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(body, &req); err == nil {
			content = req.Content
		}
	}

	filePath := resolveFilePath(workspace, name)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create directory: "+err.Error())
		return
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write file: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "file saved",
		"name":    name,
	})
}
