package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/smallnest/goclaw/agent"
	"github.com/smallnest/goclaw/agent/tools"
	"github.com/smallnest/goclaw/bus"
	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/memory"
	"github.com/smallnest/goclaw/providers"
	"github.com/smallnest/goclaw/session"
	"go.uber.org/zap"
)

// CorporateSwarmManager 公司化蜂群管理器
type CorporateSwarmManager struct {
	config     *config.CorporateSwarmConfig
	secretary  *agent.Agent
	hr         *agent.Agent
	pm         *agent.Agent
	taskBoard  *TaskBoard
	approval   *ApprovalManager
	provider   providers.Provider
	sessionMgr *session.Manager
	bus        *bus.MessageBus
	homeDir    string
	running    bool
	mu         sync.RWMutex
	cancelFunc context.CancelFunc
	messageLog *MessageLog

	// 动态 worker
	workers   map[string]*agent.Agent
	workersMu sync.RWMutex

	// IM 桥接字段
	channelBus   *bus.MessageBus     // 主总线（IM 通道用）
	lastInbound  *bus.InboundMessage // 最近入站消息上下文
	inboundMu    sync.Mutex         // 保护 lastInbound
	baseTools    []tools.Tool        // 基础工具列表
	skillsLoader *agent.SkillsLoader // 技能加载器
	memorySearchMgr memory.MemorySearchManager // 记忆搜索管理器
}

// LoadCorporateSwarmConfig 加载公司化蜂群配置
func LoadCorporateSwarmConfig(path string) (*config.CorporateSwarmConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read corporate swarm config: %w", err)
	}

	var cfg config.CorporateSwarmConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse corporate swarm config: %w", err)
	}

	if cfg.Mode != "corporate" {
		return nil, fmt.Errorf("invalid mode: expected 'corporate', got '%s'", cfg.Mode)
	}

	return &cfg, nil
}

// NewCorporateSwarmManager 创建公司化蜂群管理器
func NewCorporateSwarmManager(
	cfg *config.CorporateSwarmConfig,
	provider providers.Provider,
	sessionMgr *session.Manager,
	homeDir string,
) *CorporateSwarmManager {
	dataDir := filepath.Join(homeDir, ".goclaw", "workspaces", cfg.Name)
	timeoutMin := cfg.Approval.TimeoutMinutes
	if timeoutMin <= 0 {
		timeoutMin = 10
	}

	return &CorporateSwarmManager{
		config:     cfg,
		provider:   provider,
		sessionMgr: sessionMgr,
		bus:        bus.NewMessageBus(100),
		taskBoard:  NewTaskBoard(dataDir),
		approval:   NewApprovalManager(timeoutMin),
		homeDir:    homeDir,
		messageLog: NewMessageLog(500),
		workers:    make(map[string]*agent.Agent),
	}
}

// Start 启动公司化蜂群
func (m *CorporateSwarmManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("corporate swarm already running")
	}

	logger.Info("Starting corporate swarm", zap.String("name", m.config.Name))

	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	// 1. 准备 workspace
	if err := m.prepareWorkspaces(); err != nil {
		cancel()
		return fmt.Errorf("failed to prepare workspaces: %w", err)
	}

	// 2. 加载任务看板
	if err := m.taskBoard.LoadFromDisk(); err != nil {
		logger.Warn("Failed to load task board, starting fresh", zap.Error(err))
	}

	// 3. 设置回调
	m.setupCallbacks()

	// 4. 创建三个角色 Agent
	if err := m.createRoleAgents(ctx); err != nil {
		cancel()
		return fmt.Errorf("failed to create role agents: %w", err)
	}

	// 5. 启动消息路由
	go m.routeMessages(ctx)

	m.running = true
	logger.Info("Corporate swarm started",
		zap.String("name", m.config.Name),
		zap.String("secretary", m.config.Secretary.AgentID),
		zap.String("hr", m.config.HR.AgentID),
		zap.String("pm", m.config.PM.AgentID))

	return nil
}

// Stop 停止公司化蜂群
func (m *CorporateSwarmManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	logger.Info("Stopping corporate swarm", zap.String("name", m.config.Name))

	if m.cancelFunc != nil {
		m.cancelFunc()
	}

	// 停止各角色 Agent
	if m.secretary != nil {
		_ = m.secretary.Stop()
	}
	if m.hr != nil {
		_ = m.hr.Stop()
	}
	if m.pm != nil {
		_ = m.pm.Stop()
	}

	// 停止所有动态 worker
	m.workersMu.Lock()
	for id, w := range m.workers {
		if err := w.Stop(); err != nil {
			logger.Error("Failed to stop worker",
				zap.String("worker_id", id),
				zap.Error(err))
		}
	}
	m.workers = make(map[string]*agent.Agent)
	m.workersMu.Unlock()

	// 保存任务看板
	if err := m.taskBoard.SaveToDisk(); err != nil {
		logger.Error("Failed to save task board", zap.Error(err))
	}

	m.bus.Close()
	m.running = false

	logger.Info("Corporate swarm stopped", zap.String("name", m.config.Name))
	return nil
}

// HandleUserMessage 处理用户消息
func (m *CorporateSwarmManager) HandleUserMessage(ctx context.Context, content string) error {
	m.mu.RLock()
	if !m.running {
		m.mu.RUnlock()
		return fmt.Errorf("corporate swarm not running")
	}
	m.mu.RUnlock()

	// 1. 先检查是否为审批回复
	if resolved, approved, reqID := m.approval.TryResolve(content); resolved {
		logger.Info("Approval resolved via user message",
			zap.String("req_id", reqID),
			zap.Bool("approved", approved))
		return nil
	}

	// 2. 否则交给 Secretary 处理
	// 记录用户消息
	m.messageLog.Add(MessageLogEntry{
		ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		From:      "user",
		To:        m.config.Secretary.AgentID,
		Direction: DirectionRequest,
		Content:   content,
		SwarmName: m.config.Name,
	})
	return m.secretary.Prompt(ctx, content)
}

// GetTaskBoard 获取任务看板
func (m *CorporateSwarmManager) GetTaskBoard() *TaskBoard {
	return m.taskBoard
}

// GetApproval 获取审批管理器
func (m *CorporateSwarmManager) GetApproval() *ApprovalManager {
	return m.approval
}

// GetStatus 获取蜂群状态
func (m *CorporateSwarmManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"name":             m.config.Name,
		"mode":             "corporate",
		"running":          m.running,
		"secretary":        m.config.Secretary.AgentID,
		"hr":               m.config.HR.AgentID,
		"pm":               m.config.PM.AgentID,
		"tasks_total":      m.taskBoard.Count(),
		"pending_approvals": m.approval.PendingCount(),
	}
}

// prepareWorkspaces 准备各角色的工作区目录
func (m *CorporateSwarmManager) prepareWorkspaces() error {
	roles := []string{"secretary", "hr", "pm"}
	for _, role := range roles {
		dir := m.roleWorkspace(role)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create workspace for %s: %w", role, err)
		}
	}
	return nil
}

// roleWorkspace 获取角色工作区路径
func (m *CorporateSwarmManager) roleWorkspace(role string) string {
	return filepath.Join(m.homeDir, ".goclaw", "workspaces", m.config.Name, role)
}

// setupCallbacks 设置各组件的回调
func (m *CorporateSwarmManager) setupCallbacks() {
	// 任务状态变更回调
	m.taskBoard.SetOnStatusChange(func(task *Task, oldStatus TaskStatus) {
		logger.Info("Task status changed",
			zap.String("task_id", task.ID),
			zap.String("old_status", string(oldStatus)),
			zap.String("new_status", string(task.Status)))

		// 保存任务看板
		if err := m.taskBoard.SaveToDisk(); err != nil {
			logger.Error("Failed to save task board after status change", zap.Error(err))
		}
	})

	// 审批完成回调
	m.approval.SetOnResolved(func(req *ApprovalRequest) {
		logger.Info("Approval resolved, updating task",
			zap.String("req_id", req.ID),
			zap.String("task_id", req.TaskID),
			zap.String("status", string(req.Status)))

		if req.TaskID == "" {
			return
		}

		if req.Status == ApprovalStatusApproved {
			if err := m.taskBoard.UpdateStatus(req.TaskID, TaskStatusApproved); err != nil {
				logger.Error("Failed to update task status after approval", zap.Error(err))
			}
		} else if req.Status == ApprovalStatusRejected {
			if err := m.taskBoard.UpdateStatus(req.TaskID, TaskStatusRejected); err != nil {
				logger.Error("Failed to update task status after rejection", zap.Error(err))
			}
		}
	})
}

// createRoleAgents 创建三个角色的 Agent
func (m *CorporateSwarmManager) createRoleAgents(ctx context.Context) error {
	// 写入 IDENTITY 文件
	identities := map[string]string{
		"secretary": SecretaryIdentity(m.config.Name, m.config.HR.AgentID, m.config.PM.AgentID),
		"hr":        HRIdentity(m.config.Name, m.config.Secretary.AgentID, m.config.PM.AgentID),
		"pm":        PMIdentity(m.config.Name, m.config.Secretary.AgentID, m.config.HR.AgentID),
	}

	for role, identity := range identities {
		identityPath := filepath.Join(m.roleWorkspace(role), "IDENTITY.md")
		if err := os.WriteFile(identityPath, []byte(identity), 0644); err != nil {
			return fmt.Errorf("failed to write identity for %s: %w", role, err)
		}
	}

	// 创建 Secretary
	var err error
	secretaryWorkspace := m.roleWorkspace("secretary")
	m.secretary, err = m.createRoleAgent(ctx, m.config.Secretary.AgentID, secretaryWorkspace, m.config.Secretary)
	if err != nil {
		return fmt.Errorf("failed to create secretary agent: %w", err)
	}

	// 创建 HR
	hrWorkspace := m.roleWorkspace("hr")
	m.hr, err = m.createRoleAgent(ctx, m.config.HR.AgentID, hrWorkspace, m.config.HR)
	if err != nil {
		return fmt.Errorf("failed to create hr agent: %w", err)
	}

	// 创建 PM
	pmWorkspace := m.roleWorkspace("pm")
	m.pm, err = m.createRoleAgent(ctx, m.config.PM.AgentID, pmWorkspace, m.config.PM)
	if err != nil {
		return fmt.Errorf("failed to create pm agent: %w", err)
	}

	// 启动各 Agent
	if err := m.secretary.Start(ctx); err != nil {
		return fmt.Errorf("failed to start secretary: %w", err)
	}
	if err := m.hr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start hr: %w", err)
	}
	if err := m.pm.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pm: %w", err)
	}

	return nil
}

// createRoleAgent 创建角色 Agent
func (m *CorporateSwarmManager) createRoleAgent(
	ctx context.Context,
	agentID, workspace string,
	roleCfg config.CorporateRoleConfig,
) (*agent.Agent, error) {
	memoryStore := agent.NewMemoryStore(workspace)
	contextBuilder := agent.NewContextBuilder(memoryStore, workspace)
	toolRegistry := agent.NewToolRegistry()

	// 注册 agent_call 工具
	agentCallTool := agent.NewAgentCallTool(m.bus)
	agentCallTool.SetCallAgent(m.callAgentSync)
	agentCallTool.SetAgentExistsChecker(m.agentExists)
	agentCallTool.SetAgentConfigGetter(m.getAgentConfig)
	agentCallTool.SetAgentIDGetter(func(sessionKey string) string {
		return agentID
	})
	toolRegistry.RegisterAgentTool(agentCallTool)

	// 注册基础工具（按 DenyTools 过滤）
	if len(m.baseTools) > 0 {
		denySet := make(map[string]bool, len(roleCfg.DenyTools))
		for _, name := range roleCfg.DenyTools {
			denySet[name] = true
		}
		for _, tool := range m.baseTools {
			if denySet[tool.Name()] {
				logger.Debug("Skipping denied tool for role",
					zap.String("agent", agentID),
					zap.String("tool", tool.Name()))
				continue
			}
			if err := toolRegistry.RegisterExisting(tool); err != nil {
				logger.Warn("Failed to register base tool for role",
					zap.String("agent", agentID),
					zap.String("tool", tool.Name()),
					zap.Error(err))
			}
		}
		logger.Info("Base tools registered for role",
			zap.String("agent", agentID),
			zap.Int("total", len(m.baseTools)),
			zap.Int("denied", len(roleCfg.DenyTools)))
	}

	// 注册记忆工具（如果记忆管理器已设置）
	if m.memorySearchMgr != nil {
		searchTool := tools.NewMemoryTool(m.memorySearchMgr, agentID)
		if err := toolRegistry.RegisterExisting(searchTool); err != nil {
			logger.Warn("Failed to register memory_search tool", zap.String("agent", agentID), zap.Error(err))
		}

		addTool := tools.NewMemoryAddTool(m.memorySearchMgr, agentID)
		if err := toolRegistry.RegisterExisting(addTool); err != nil {
			logger.Warn("Failed to register memory_add tool", zap.String("agent", agentID), zap.Error(err))
		}

		// 共享记忆目录
		sharedDir := filepath.Join(m.homeDir, ".goclaw", "workspaces", m.config.Name, "shared")
		readSharedTool := tools.NewSharedMemoryReadTool(sharedDir)
		if err := toolRegistry.RegisterExisting(readSharedTool); err != nil {
			logger.Warn("Failed to register memory_read_shared tool", zap.String("agent", agentID), zap.Error(err))
		}

		writeSharedTool := tools.NewSharedMemoryWriteTool(sharedDir, agentID)
		if err := toolRegistry.RegisterExisting(writeSharedTool); err != nil {
			logger.Warn("Failed to register memory_write_shared tool", zap.String("agent", agentID), zap.Error(err))
		}
	}

	agentCfg := &agent.NewAgentConfig{
		Bus:          m.bus,
		Provider:     m.provider,
		SessionMgr:   m.sessionMgr,
		Tools:        toolRegistry,
		Context:      contextBuilder,
		Workspace:    workspace,
		MaxIteration: 15,
		SessionKey:   "corporate:" + m.config.Name + ":" + agentID,
		ToolTimeout:  10 * time.Minute,
		SkillsLoader: m.skillsLoader,
	}

	return agent.NewAgent(agentCfg)
}

// callAgentSync 同步调用目标 Agent
func (m *CorporateSwarmManager) callAgentSync(ctx context.Context, targetAgentID, message string) (string, error) {
	var target *agent.Agent
	var callerID string

	switch targetAgentID {
	case m.config.Secretary.AgentID:
		target = m.secretary
	case m.config.HR.AgentID:
		target = m.hr
	case m.config.PM.AgentID:
		target = m.pm
	default:
		// 查找动态 worker
		m.workersMu.RLock()
		w, ok := m.workers[targetAgentID]
		m.workersMu.RUnlock()
		if ok {
			target = w
		} else {
			return "", fmt.Errorf("unknown agent: %s", targetAgentID)
		}
	}

	if target == nil {
		return "", fmt.Errorf("agent %s is not running", targetAgentID)
	}

	// 推断调用者
	callerID = m.inferCaller(targetAgentID)

	// 记录请求
	m.messageLog.Add(MessageLogEntry{
		ID:        fmt.Sprintf("call-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		From:      callerID,
		To:        targetAgentID,
		Direction: DirectionRequest,
		Content:   message,
		SwarmName: m.config.Name,
	})

	// 使用 orchestrator 直接执行获取同步结果
	userMsg := agent.AgentMessage{
		Role:    agent.RoleUser,
		Content: []agent.ContentBlock{agent.TextContent{Text: message}},
	}

	orchestrator := target.GetOrchestrator()
	finalMessages, err := orchestrator.Run(ctx, []agent.AgentMessage{userMsg})
	if err != nil {
		return "", fmt.Errorf("agent %s execution failed: %w", targetAgentID, err)
	}

	// 提取最后一条 assistant 消息
	for i := len(finalMessages) - 1; i >= 0; i-- {
		if finalMessages[i].Role == agent.RoleAssistant {
			for _, block := range finalMessages[i].Content {
				if text, ok := block.(agent.TextContent); ok {
					// 记录响应
					m.messageLog.Add(MessageLogEntry{
						ID:        fmt.Sprintf("resp-%d", time.Now().UnixNano()),
						Timestamp: time.Now(),
						From:      targetAgentID,
						To:        callerID,
						Direction: DirectionResponse,
						Content:   text.Text,
						SwarmName: m.config.Name,
					})
					return text.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("agent %s returned no response", targetAgentID)
}

// agentExists 检查 agent 是否存在
func (m *CorporateSwarmManager) agentExists(agentID string) bool {
	if agentID == m.config.Secretary.AgentID ||
		agentID == m.config.HR.AgentID ||
		agentID == m.config.PM.AgentID {
		return true
	}

	m.workersMu.RLock()
	_, ok := m.workers[agentID]
	m.workersMu.RUnlock()
	return ok
}

// getAgentConfig 获取 agent 配置（用于权限检查）
func (m *CorporateSwarmManager) getAgentConfig(agentID string) *config.AgentConfig {
	var allowAgents []string

	switch agentID {
	case m.config.Secretary.AgentID:
		allowAgents = []string{m.config.HR.AgentID, m.config.PM.AgentID}
	case m.config.HR.AgentID:
		allowAgents = []string{m.config.Secretary.AgentID, m.config.PM.AgentID}
	case m.config.PM.AgentID:
		allowAgents = []string{m.config.Secretary.AgentID, m.config.HR.AgentID}
	default:
		// 动态 worker：允许调用所有管理层
		m.workersMu.RLock()
		_, isWorker := m.workers[agentID]
		m.workersMu.RUnlock()
		if !isWorker {
			return nil
		}
		allowAgents = []string{
			m.config.Secretary.AgentID,
			m.config.HR.AgentID,
			m.config.PM.AgentID,
		}
	}

	return &config.AgentConfig{
		ID: agentID,
		AgentCall: &config.AgentCallConfig{
			AllowAgents: allowAgents,
			Timeout:     300,
		},
	}
}

// routeMessages 路由消息
func (m *CorporateSwarmManager) routeMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := m.bus.ConsumeOutbound(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			if msg == nil {
				continue
			}

			logger.Debug("Corporate swarm outbound message",
				zap.String("channel", msg.Channel),
				zap.String("chat_id", msg.ChatID),
				zap.Int("content_length", len(msg.Content)))

			// 如果主通道总线已设置，将回复转发到 IM
			if m.channelBus != nil {
				m.inboundMu.Lock()
				lastMsg := m.lastInbound
				m.inboundMu.Unlock()

				if lastMsg != nil {
					outbound := &bus.OutboundMessage{
						Channel: lastMsg.Channel,
						ChatID:  lastMsg.ChatID,
						Content: msg.Content,
						ReplyTo: lastMsg.ID,
					}
					if err := m.channelBus.PublishOutbound(ctx, outbound); err != nil {
						logger.Error("Failed to forward message to IM channel bus",
							zap.String("channel", lastMsg.Channel),
							zap.Error(err))
					} else {
						logger.Info("Forwarded corporate swarm reply to IM",
							zap.String("channel", lastMsg.Channel),
							zap.String("chat_id", lastMsg.ChatID))
					}
				} else {
					logger.Warn("No inbound context available, cannot forward reply to IM")
				}
			}
		}
	}
}

// SendToUser 发送消息到用户（通过 IM 通道）
func (m *CorporateSwarmManager) SendToUser(ctx context.Context, content string) error {
	outbound := &bus.OutboundMessage{
		Channel: "corporate:" + m.config.Name,
		ChatID:  "user",
		Content: content,
	}
	return m.bus.PublishOutbound(ctx, outbound)
}

// GetAgent 获取指定角色的 Agent
func (m *CorporateSwarmManager) GetAgent(role string) (*agent.Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch role {
	case "secretary", m.config.Secretary.AgentID:
		return m.secretary, m.secretary != nil
	case "hr", m.config.HR.AgentID:
		return m.hr, m.hr != nil
	case "pm", m.config.PM.AgentID:
		return m.pm, m.pm != nil
	}

	// 查找动态 worker
	m.workersMu.RLock()
	defer m.workersMu.RUnlock()
	if w, ok := m.workers[role]; ok {
		return w, true
	}

	return nil, false
}

// ListAgents 列出所有角色
func (m *CorporateSwarmManager) ListAgents() []string {
	agents := []string{
		m.config.Secretary.AgentID,
		m.config.HR.AgentID,
		m.config.PM.AgentID,
	}

	m.workersMu.RLock()
	for id := range m.workers {
		agents = append(agents, id)
	}
	m.workersMu.RUnlock()

	return agents
}

// GetAgentWorkspace 获取agent的工作区路径
func (m *CorporateSwarmManager) GetAgentWorkspace(id string) string {
	ag, ok := m.GetAgent(id)
	if !ok {
		return ""
	}
	return ag.GetWorkspace()
}

// GetMessageLog 获取消息日志
func (m *CorporateSwarmManager) GetMessageLog() *MessageLog {
	return m.messageLog
}

// inferCaller 推断调用者（用于消息日志中标注 From）
func (m *CorporateSwarmManager) inferCaller(targetAgentID string) string {
	// corporate 模式中，调用通常来自其他角色或用户
	// 这里无法精确知道调用者，标记为 "swarm"
	return "swarm"
}

// SetChannelBus 设置主通道总线，用于回复 IM 消息
func (m *CorporateSwarmManager) SetChannelBus(mainBus *bus.MessageBus) {
	m.channelBus = mainBus
}

// SetBaseTools 设置基础工具列表，供角色 Agent 使用
func (m *CorporateSwarmManager) SetBaseTools(baseTools []tools.Tool) {
	m.baseTools = baseTools
}

// SetSkillsLoader 设置技能加载器，供角色 Agent 使用
func (m *CorporateSwarmManager) SetSkillsLoader(loader *agent.SkillsLoader) {
	m.skillsLoader = loader
}

// SetMemorySearchManager 设置记忆搜索管理器
func (m *CorporateSwarmManager) SetMemorySearchManager(mgr memory.MemorySearchManager) {
	m.memorySearchMgr = mgr
}

// HandleInboundMessage 处理来自 IM 的入站消息（保留通道上下文）
func (m *CorporateSwarmManager) HandleInboundMessage(ctx context.Context, msg *bus.InboundMessage) error {
	// 保存入站消息上下文，用于回复时定位 IM 通道
	m.inboundMu.Lock()
	m.lastInbound = msg
	m.inboundMu.Unlock()

	logger.Info("Corporate swarm received inbound message",
		zap.String("channel", msg.Channel),
		zap.String("chat_id", msg.ChatID),
		zap.String("sender", msg.SenderID),
		zap.Int("content_length", len(msg.Content)))

	// 交给 HandleUserMessage 处理
	return m.HandleUserMessage(ctx, msg.Content)
}

// AddWorker 动态添加 worker agent
func (m *CorporateSwarmManager) AddWorker(ctx context.Context, agentID string, roleCfg config.CorporateRoleConfig) error {
	m.mu.RLock()
	running := m.running
	m.mu.RUnlock()

	if !running {
		return fmt.Errorf("corporate swarm not running")
	}

	// 检查不与管理层 ID 冲突
	if agentID == m.config.Secretary.AgentID ||
		agentID == m.config.HR.AgentID ||
		agentID == m.config.PM.AgentID {
		return fmt.Errorf("cannot add worker with management role ID '%s'", agentID)
	}

	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	if _, exists := m.workers[agentID]; exists {
		return fmt.Errorf("worker '%s' already exists", agentID)
	}

	workspace := filepath.Join(m.homeDir, ".goclaw", "workspaces", m.config.Name, agentID)
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("failed to create worker workspace: %w", err)
	}

	if roleCfg.AgentID == "" {
		roleCfg.AgentID = agentID
	}

	ag, err := m.createRoleAgent(ctx, agentID, workspace, roleCfg)
	if err != nil {
		return fmt.Errorf("failed to create worker agent: %w", err)
	}

	if err := ag.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker agent: %w", err)
	}

	m.workers[agentID] = ag

	logger.Info("Worker dynamically added",
		zap.String("worker_id", agentID),
		zap.String("workspace", workspace))

	return nil
}

// RemoveWorker 移除 worker（不能移除 secretary/hr/pm）
func (m *CorporateSwarmManager) RemoveWorker(agentID string) error {
	// 检查不能移除管理层
	if agentID == m.config.Secretary.AgentID ||
		agentID == m.config.HR.AgentID ||
		agentID == m.config.PM.AgentID {
		return fmt.Errorf("cannot remove management role '%s'", agentID)
	}

	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	w, exists := m.workers[agentID]
	if !exists {
		return fmt.Errorf("worker '%s' not found", agentID)
	}

	if err := w.Stop(); err != nil {
		logger.Error("Failed to stop worker during removal",
			zap.String("worker_id", agentID),
			zap.Error(err))
	}

	delete(m.workers, agentID)

	logger.Info("Worker dynamically removed", zap.String("worker_id", agentID))

	return nil
}
