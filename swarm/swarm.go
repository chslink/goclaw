package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
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

// SwarmConfig 蜂群配置
type SwarmConfig struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Mode        string     `json:"mode"` // "flat"(默认) 或 "corporate"
	AgentIDs    []string   `json:"agent_ids"`
	Flows       []FlowStep `json:"flows"`
}

// FlowStep 流转步骤
type FlowStep struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	From        string `json:"from"`
	To          string `json:"to"`
	Condition   string `json:"condition"`
}

// SwarmManager 蜂群管理器
type SwarmManager struct {
	config         *SwarmConfig
	agents         map[string]*agent.Agent
	provider       providers.Provider
	sessionMgr     *session.Manager
	bus            *bus.MessageBus
	running        bool
	mu             sync.RWMutex
	pendingMsgs    map[string]*PendingMessage
	pendingMu      sync.RWMutex
	homeDir        string
	messageLog     *MessageLog
	memorySearchMgr memory.MemorySearchManager
}

// PendingMessage 待处理消息
type PendingMessage struct {
	ID        string
	FromAgent string
	ToAgent   string
	Content   string
	FlowName  string
	SentAt    time.Time
}

// LoadSwarmConfig 加载蜂群配置
func LoadSwarmConfig(path string) (*SwarmConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read swarm config: %w", err)
	}

	var cfg SwarmConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse swarm config: %w", err)
	}

	return &cfg, nil
}

// NewSwarmManager 创建蜂群管理器
func NewSwarmManager(cfg *SwarmConfig, provider providers.Provider, sessionMgr *session.Manager, homeDir string) *SwarmManager {
	return &SwarmManager{
		config:      cfg,
		agents:      make(map[string]*agent.Agent),
		provider:    provider,
		sessionMgr:  sessionMgr,
		bus:         bus.NewMessageBus(100),
		pendingMsgs: make(map[string]*PendingMessage),
		homeDir:     homeDir,
		messageLog:  NewMessageLog(500),
	}
}

// SetMemorySearchManager 设置记忆搜索管理器
func (m *SwarmManager) SetMemorySearchManager(mgr memory.MemorySearchManager) {
	m.memorySearchMgr = mgr
}

// loadAgentConfig 加载agent配置
func (m *SwarmManager) loadAgentConfig(agentID string) (*config.AgentConfig, error) {
	return config.LoadAgentByName(agentID)
}

// Start 启动蜂群
func (m *SwarmManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("swarm already running")
	}

	logger.Info("Starting swarm", zap.String("name", m.config.Name))

	for _, agentID := range m.config.AgentIDs {
		agentCfg, err := m.loadAgentConfig(agentID)
		if err != nil {
			logger.Error("Failed to load agent config",
				zap.String("agent_id", agentID),
				zap.Error(err))
			continue
		}

		workspace := agentCfg.Workspace
		if workspace == "" {
			workspace = filepath.Join(m.homeDir, ".goclaw", "workspaces", agentID)
		}

		ag, err := m.createAgent(ctx, agentID, workspace)
		if err != nil {
			logger.Error("Failed to create agent",
				zap.String("agent_id", agentID),
				zap.Error(err))
			continue
		}

		if err := ag.Start(ctx); err != nil {
			logger.Error("Failed to start agent",
				zap.String("agent_id", agentID),
				zap.Error(err))
			continue
		}

		m.agents[agentID] = ag
		logger.Info("Agent started", zap.String("agent_id", agentID))
	}

	go m.handleResponses(ctx)

	m.running = true
	logger.Info("Swarm started",
		zap.String("name", m.config.Name),
		zap.Int("agents", len(m.agents)))

	return nil
}

func (m *SwarmManager) createAgent(ctx context.Context, agentID, workspace string) (*agent.Agent, error) {
	memoryStore := agent.NewMemoryStore(workspace)
	contextBuilder := agent.NewContextBuilder(memoryStore, workspace)
	toolRegistry := agent.NewToolRegistry()

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
		SessionKey:   "swarm:" + m.config.Name + ":" + agentID,
	}

	return agent.NewAgent(agentCfg)
}

// Stop 停止蜂群
func (m *SwarmManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	logger.Info("Stopping swarm", zap.String("name", m.config.Name))

	for id, ag := range m.agents {
		if err := ag.Stop(); err != nil {
			logger.Error("Failed to stop agent",
				zap.String("agent_id", id),
				zap.Error(err))
		}
	}

	m.bus.Close()
	m.running = false
	m.agents = make(map[string]*agent.Agent)

	logger.Info("Swarm stopped", zap.String("name", m.config.Name))
	return nil
}

// SendMessage 异步发送消息
func (m *SwarmManager) SendMessage(ctx context.Context, fromAgent, toAgent, content string, flowName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return fmt.Errorf("swarm not running")
	}

	if _, ok := m.agents[toAgent]; !ok {
		return fmt.Errorf("agent '%s' not found", toAgent)
	}

	msgID := uuid.New().String()

	m.pendingMu.Lock()
	m.pendingMsgs[msgID] = &PendingMessage{
		ID:        msgID,
		FromAgent: fromAgent,
		ToAgent:   toAgent,
		Content:   content,
		FlowName:  flowName,
		SentAt:    time.Now(),
	}
	m.pendingMu.Unlock()

	inboundMsg := &bus.InboundMessage{
		ID:        uuid.New().String(),
		Channel:   "swarm:" + m.config.Name,
		SenderID:  fromAgent,
		ChatID:    msgID,
		AgentID:   toAgent,
		Content:   content,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"swarm_name": m.config.Name,
			"flow_name":  flowName,
		},
	}

	if err := m.bus.PublishInbound(ctx, inboundMsg); err != nil {
		m.pendingMu.Lock()
		delete(m.pendingMsgs, msgID)
		m.pendingMu.Unlock()
		return fmt.Errorf("failed to send message: %w", err)
	}

	// 记录发送消息
	m.messageLog.Add(MessageLogEntry{
		ID:        msgID,
		Timestamp: time.Now(),
		From:      fromAgent,
		To:        toAgent,
		Direction: DirectionRequest,
		Content:   content,
		FlowName:  flowName,
		SwarmName: m.config.Name,
	})

	logger.Info("Message sent",
		zap.String("msg_id", msgID),
		zap.String("from", fromAgent),
		zap.String("to", toAgent),
		zap.String("flow", flowName))

	return nil
}

func (m *SwarmManager) handleResponses(ctx context.Context) {
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
			m.processResponse(ctx, msg)
		}
	}
}

func (m *SwarmManager) processResponse(ctx context.Context, msg *bus.OutboundMessage) {
	m.pendingMu.Lock()
	pending, ok := m.pendingMsgs[msg.ChatID]
	if ok {
		delete(m.pendingMsgs, msg.ChatID)
	}
	m.pendingMu.Unlock()

	if !ok {
		return
	}

	logger.Info("Response received",
		zap.String("msg_id", pending.ID),
		zap.String("from", pending.ToAgent))

	// 记录响应消息
	m.messageLog.Add(MessageLogEntry{
		ID:        pending.ID + ":resp",
		Timestamp: time.Now(),
		From:      pending.ToAgent,
		To:        pending.FromAgent,
		Direction: DirectionResponse,
		Content:   msg.Content,
		FlowName:  pending.FlowName,
		SwarmName: m.config.Name,
	})

	flowName := pending.FlowName

	var flow *FlowStep
	for i := range m.config.Flows {
		if m.config.Flows[i].Name == flowName && m.config.Flows[i].From == pending.ToAgent {
			flow = &m.config.Flows[i]
			break
		}
	}

	if flow == nil {
		return
	}

	if err := m.SendMessage(ctx, flow.From, flow.To, msg.Content, flowName); err != nil {
		logger.Error("Failed to send next message",
			zap.String("flow", flowName),
			zap.String("from", flow.From),
			zap.String("to", flow.To),
			zap.Error(err))
	}
}

// GetStatus 获取蜂群状态
func (m *SwarmManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agentStatus := make(map[string]string)
	for id, ag := range m.agents {
		if ag != nil {
			agentStatus[id] = "running"
		} else {
			agentStatus[id] = "stopped"
		}
	}

	m.pendingMu.RLock()
	pendingCount := len(m.pendingMsgs)
	m.pendingMu.RUnlock()

	return map[string]interface{}{
		"name":             m.config.Name,
		"running":          m.running,
		"agents":           agentStatus,
		"pending_messages": pendingCount,
	}
}

// GetMessageLog 获取消息日志
func (m *SwarmManager) GetMessageLog() *MessageLog {
	return m.messageLog
}

// GetAgent 获取指定agent
func (m *SwarmManager) GetAgent(agentID string) (*agent.Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ag, ok := m.agents[agentID]
	return ag, ok
}

// ListAgents 列出所有agent
func (m *SwarmManager) ListAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agents := make([]string, 0, len(m.agents))
	for id := range m.agents {
		agents = append(agents, id)
	}
	return agents
}

// GetAgentWorkspace 获取agent的工作区路径
func (m *SwarmManager) GetAgentWorkspace(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ag, ok := m.agents[id]
	if !ok {
		return ""
	}
	return ag.GetWorkspace()
}

// AddAgent 动态添加已有磁盘配置的 agent
func (m *SwarmManager) AddAgent(ctx context.Context, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("swarm not running")
	}

	if _, exists := m.agents[agentID]; exists {
		return fmt.Errorf("agent '%s' already exists", agentID)
	}

	agentCfg, err := config.LoadAgentByName(agentID)
	if err != nil {
		return fmt.Errorf("failed to load agent config: %w", err)
	}

	workspace := agentCfg.Workspace
	if workspace == "" {
		workspace = filepath.Join(m.homeDir, ".goclaw", "workspaces", agentID)
	}

	ag, err := m.createAgent(ctx, agentID, workspace)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	if err := ag.Start(ctx); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	m.agents[agentID] = ag
	m.config.AgentIDs = append(m.config.AgentIDs, agentID)

	logger.Info("Agent dynamically added",
		zap.String("agent_id", agentID),
		zap.String("workspace", workspace))

	return nil
}

// AddAgentWithConfig 运行时动态创建 agent（无需磁盘配置）
func (m *SwarmManager) AddAgentWithConfig(ctx context.Context, agentID, workspace, model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("swarm not running")
	}

	if _, exists := m.agents[agentID]; exists {
		return fmt.Errorf("agent '%s' already exists", agentID)
	}

	if workspace == "" {
		workspace = filepath.Join(m.homeDir, ".goclaw", "workspaces", agentID)
	}

	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	ag, err := m.createAgent(ctx, agentID, workspace)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	if err := ag.Start(ctx); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	m.agents[agentID] = ag
	m.config.AgentIDs = append(m.config.AgentIDs, agentID)

	logger.Info("Agent dynamically created",
		zap.String("agent_id", agentID),
		zap.String("workspace", workspace),
		zap.String("model", model))

	return nil
}

// RemoveAgent 停止并移除 agent
func (m *SwarmManager) RemoveAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ag, exists := m.agents[agentID]
	if !exists {
		return fmt.Errorf("agent '%s' not found", agentID)
	}

	if err := ag.Stop(); err != nil {
		logger.Error("Failed to stop agent during removal",
			zap.String("agent_id", agentID),
			zap.Error(err))
	}

	delete(m.agents, agentID)

	// 从 config.AgentIDs 中移除
	newIDs := make([]string, 0, len(m.config.AgentIDs))
	for _, id := range m.config.AgentIDs {
		if id != agentID {
			newIDs = append(newIDs, id)
		}
	}
	m.config.AgentIDs = newIDs

	logger.Info("Agent dynamically removed", zap.String("agent_id", agentID))

	return nil
}
