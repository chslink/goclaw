package admin

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/smallnest/goclaw/channels"
	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/cron"
	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/session"
	"go.uber.org/zap"
)

//go:embed static/*
var staticFiles embed.FS

// AgentManagerAPI 定义 admin 对 AgentManager 的访问接口（避免循环依赖）
type AgentManagerAPI interface {
	ListAgents() []string
	GetAgentConfig(id string) *AgentInfo
	GetToolsInfo() (map[string]interface{}, error)
	GetAgentWorkspace(id string) string
}

// AgentInfo Agent 信息
type AgentInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Model  string `json:"model"`
	Status string `json:"status"`
}

// SwarmManagerAPI 定义 admin 对 Swarm 的访问接口（避免循环依赖）
type SwarmManagerAPI interface {
	GetMode() string                                   // "flat" 或 "corporate"
	GetStatus() map[string]interface{}                 // 蜂群状态
	ListAgents() []string                              // Agent 列表
	GetAgentWorkspace(id string) string                // Agent 工作区路径
	GetTasks() []map[string]interface{}                // 任务看板（corporate 模式）
	GetApprovals() (pending, resolved []map[string]interface{}) // 审批列表
	ApproveRequest(id string) error                    // 批准审批
	RejectRequest(id, reason string) error             // 驳回审批
	GetMessages(n int) []map[string]interface{}        // Agent 间沟通记录
	AddAgent(ctx context.Context, agentID, workspace, model string) error // 动态添加 Agent
	RemoveAgent(agentID string) error                  // 动态移除 Agent
}

// AdminHandler Admin 管理界面处理器
type AdminHandler struct {
	config     *config.Config
	configPath string
	sessionMgr *session.Manager
	channelMgr *channels.Manager
	cronSvc    *cron.Service
	agentMgr   AgentManagerAPI
	swarmMgr   SwarmManagerAPI
	logBuffer  *LogRingBuffer
}

// NewAdminHandler 创建 AdminHandler
func NewAdminHandler(cfg *config.Config, sessionMgr *session.Manager, channelMgr *channels.Manager, cronSvc *cron.Service) *AdminHandler {
	// 获取配置文件路径
	configPath := ""
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath = homeDir + "/.goclaw/config.json"
	}

	h := &AdminHandler{
		config:     cfg,
		configPath: configPath,
		sessionMgr: sessionMgr,
		channelMgr: channelMgr,
		cronSvc:    cronSvc,
		logBuffer:  NewLogRingBuffer(1000),
	}

	return h
}

// SetAgentManager 延迟注入 AgentManager
func (h *AdminHandler) SetAgentManager(mgr AgentManagerAPI) {
	h.agentMgr = mgr
}

// SetSwarmManager 延迟注入 SwarmManager
func (h *AdminHandler) SetSwarmManager(mgr SwarmManagerAPI) {
	h.swarmMgr = mgr
}

// GetLogBuffer 获取日志缓冲区（用于外部注入 logger hook）
func (h *AdminHandler) GetLogBuffer() *LogRingBuffer {
	return h.logBuffer
}

// RegisterRoutes 注册 admin 路由到 mux
func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	// 静态文件服务
	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		logger.Error("Failed to create static file sub-filesystem", zap.Error(err))
		return
	}
	fileServer := http.FileServer(http.FS(staticSub))

	// Admin API 路由（Go 1.22+ 支持方法匹配）
	mux.HandleFunc("GET /admin/api/dashboard", h.handleDashboard)
	mux.HandleFunc("GET /admin/api/config", h.handleGetConfig)
	mux.HandleFunc("PUT /admin/api/config", h.handlePutConfig)
	mux.HandleFunc("GET /admin/api/agents", h.handleListAgents)
	mux.HandleFunc("GET /admin/api/agents/{id}", h.handleGetAgent)
	mux.HandleFunc("GET /admin/api/agents/{id}/files", h.handleListAgentFiles)
	mux.HandleFunc("GET /admin/api/agents/{id}/files/{name}", h.handleGetAgentFile)
	mux.HandleFunc("PUT /admin/api/agents/{id}/files/{name}", h.handlePutAgentFile)
	mux.HandleFunc("GET /admin/api/sessions", h.handleListSessions)
	mux.HandleFunc("GET /admin/api/sessions/{key}", h.handleGetSession)
	mux.HandleFunc("DELETE /admin/api/sessions/{key}", h.handleDeleteSession)
	mux.HandleFunc("GET /admin/api/channels", h.handleListChannels)
	mux.HandleFunc("GET /admin/api/logs/stream", h.handleLogsStream)
	mux.HandleFunc("GET /admin/api/cron", h.handleListCron)
	mux.HandleFunc("POST /admin/api/cron", h.handleCreateCron)
	mux.HandleFunc("DELETE /admin/api/cron/{id}", h.handleDeleteCron)
	mux.HandleFunc("POST /admin/api/cron/{id}/run", h.handleRunCron)

	// Swarm API 路由
	mux.HandleFunc("GET /admin/api/swarms", h.handleListSwarms)
	mux.HandleFunc("GET /admin/api/swarms/active", h.handleActiveSwarm)
	mux.HandleFunc("GET /admin/api/swarms/messages", h.handleSwarmMessages)
	mux.HandleFunc("GET /admin/api/swarms/tasks", h.handleSwarmTasks)
	mux.HandleFunc("GET /admin/api/swarms/approvals", h.handleSwarmApprovals)
	mux.HandleFunc("POST /admin/api/swarms/approvals/{id}/approve", h.handleSwarmApprove)
	mux.HandleFunc("POST /admin/api/swarms/approvals/{id}/reject", h.handleSwarmReject)
	mux.HandleFunc("POST /admin/api/swarms/agents", h.handleSwarmAddAgent)
	mux.HandleFunc("DELETE /admin/api/swarms/agents/{id}", h.handleSwarmRemoveAgent)

	// 静态文件和 SPA fallback
	mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		// 去掉 /admin/ 前缀
		path := strings.TrimPrefix(r.URL.Path, "/admin/")
		if path == "" {
			path = "index.html"
		}

		// 尝试打开文件
		f, err := staticSub.(fs.ReadFileFS).ReadFile(path)
		if err != nil {
			// SPA fallback：所有未匹配路径返回 index.html
			f, err = staticSub.(fs.ReadFileFS).ReadFile("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(f)
			return
		}

		// 设置正确的 Content-Type
		if strings.HasSuffix(path, ".js") {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		} else if strings.HasSuffix(path, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		} else if strings.HasSuffix(path, ".html") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		w.Write(f)
		_ = fileServer // 保留引用
	})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
