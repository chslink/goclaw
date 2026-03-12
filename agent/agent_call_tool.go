package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/bus"
	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/internal/logger"
	"go.uber.org/zap"
)

// AgentCallTool agent间通信工具
type AgentCallTool struct {
	getAgentConfig   func(agentID string) *config.AgentConfig
	getAgentID       func(sessionKey string) string
	checkAgentExists func(agentID string) bool
	callAgent        func(ctx context.Context, targetAgentID string, message string) (string, error)
	bus              *bus.MessageBus
	pendingCalls     map[string]*PendingCall
	pendingCallsMu   sync.RWMutex
}

// PendingCall 待处理的调用
type PendingCall struct {
	ID         string
	ResponseCh chan *AgentCallResponse
	Timeout    time.Duration
	CreatedAt  time.Time
}

// AgentCallParams agent调用参数
type AgentCallParams struct {
	AgentName string `json:"agent_name"`
	Message   string `json:"message"`
	Timeout   int    `json:"timeout"`
}

// AgentCallResponse agent调用响应
type AgentCallResponse struct {
	Success   bool   `json:"success"`
	AgentName string `json:"agent_name"`
	Response  string `json:"response"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"duration_ms"`
}

// NewAgentCallTool 创建agent调用工具
func NewAgentCallTool(messageBus *bus.MessageBus) *AgentCallTool {
	return &AgentCallTool{
		bus:          messageBus,
		pendingCalls: make(map[string]*PendingCall),
	}
}

// SetAgentConfigGetter 设置agent配置获取器
func (t *AgentCallTool) SetAgentConfigGetter(getter func(agentID string) *config.AgentConfig) {
	t.getAgentConfig = getter
}

// SetAgentIDGetter 设置agent ID获取器
func (t *AgentCallTool) SetAgentIDGetter(getter func(sessionKey string) string) {
	t.getAgentID = getter
}

// SetAgentExistsChecker 设置agent存在检查器
func (t *AgentCallTool) SetAgentExistsChecker(checker func(agentID string) bool) {
	t.checkAgentExists = checker
}

// SetCallAgent 设置agent调用回调
func (t *AgentCallTool) SetCallAgent(callAgent func(ctx context.Context, targetAgentID string, message string) (string, error)) {
	t.callAgent = callAgent
}

// Name 返回工具名称
func (t *AgentCallTool) Name() string {
	return "agent_call"
}

// Description 返回工具描述
func (t *AgentCallTool) Description() string {
	return "Call another agent and get a response. Use this to delegate tasks to specialized agents."
}

// Label 返回工具标签
func (t *AgentCallTool) Label() string {
	return "Agent Communication"
}

// Parameters 返回工具参数定义
func (t *AgentCallTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_name": map[string]any{
				"type":        "string",
				"description": "The name of the agent to call (e.g., 'zhongshu', 'menxia', 'shangshu')",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "The message to send to the agent",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default: 300)",
			},
		},
		"required": []string{"agent_name", "message"},
	}
}

// Execute 执行工具
func (t *AgentCallTool) Execute(ctx context.Context, params map[string]any, onUpdate func(ToolResult)) (ToolResult, error) {
	callParams, err := t.parseParams(params)
	if err != nil {
		return ToolResult{Error: err}, nil
	}

	requesterAgentID := t.getRequesterAgentID(ctx)
	if requesterAgentID == "" {
		requesterAgentID = "default"
	}

	targetAgentID := callParams.AgentName

	if !t.checkPermission(requesterAgentID, targetAgentID) {
		errMsg := fmt.Sprintf("没有权限调用 agent '%s'", targetAgentID)
		return ToolResult{Error: fmt.Errorf("%s", errMsg)}, nil
	}

	if t.checkAgentExists != nil && !t.checkAgentExists(targetAgentID) {
		errMsg := fmt.Sprintf("agent '%s' 不存在或未启动", targetAgentID)
		return ToolResult{Error: fmt.Errorf("%s", errMsg)}, nil
	}

	timeout := time.Duration(callParams.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	startTime := time.Now()

	// 如果有 callAgent 回调，直接调用
	if t.callAgent != nil {
		// 创建带超时的 context
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		response, err := t.callAgent(callCtx, targetAgentID, callParams.Message)
		duration := time.Since(startTime).Milliseconds()

		if err != nil {
			return ToolResult{Error: fmt.Errorf("调用 agent '%s' 失败: %w", targetAgentID, err)}, nil
		}

		result := t.formatSuccess(targetAgentID, response, duration)
		return ToolResult{
			Content: []ContentBlock{
				TextContent{Text: result},
			},
		}, nil
	}

	// 否则使用 bus 发送消息
	callID := uuid.New().String()

	inboundMsg := &bus.InboundMessage{
		ID:        uuid.New().String(),
		Channel:   "agent_call",
		SenderID:  requesterAgentID,
		ChatID:    fmt.Sprintf("call:%s:%s", requesterAgentID, callID),
		AgentID:   targetAgentID,
		Content:   callParams.Message,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"call_id":         callID,
			"requester_agent": requesterAgentID,
		},
	}

	responseSub := t.bus.SubscribeOutbound()
	defer responseSub.Unsubscribe()

	if err := t.bus.PublishInbound(ctx, inboundMsg); err != nil {
		return ToolResult{Error: fmt.Errorf("发送消息失败: %w", err)}, nil
	}

	select {
	case <-ctx.Done():
		return ToolResult{Error: fmt.Errorf("调用被取消")}, nil
	case <-time.After(timeout):
		return ToolResult{Error: fmt.Errorf("调用超时 (%d秒)", callParams.Timeout)}, nil
	case outMsg := <-responseSub.Channel:
		duration := time.Since(startTime).Milliseconds()

		if outMsg == nil {
			return ToolResult{Error: fmt.Errorf("收到空响应")}, nil
		}

		response := t.formatSuccess(targetAgentID, outMsg.Content, duration)
		return ToolResult{
			Content: []ContentBlock{
				TextContent{Text: response},
			},
		}, nil
	}
}

func (t *AgentCallTool) parseParams(params map[string]any) (*AgentCallParams, error) {
	result := &AgentCallParams{
		Timeout: 300,
	}

	if agentName, ok := params["agent_name"].(string); ok {
		result.AgentName = strings.TrimSpace(agentName)
	} else {
		return nil, fmt.Errorf("缺少 agent_name 参数")
	}

	if message, ok := params["message"].(string); ok {
		result.Message = strings.TrimSpace(message)
	} else {
		return nil, fmt.Errorf("缺少 message 参数")
	}

	if timeout, ok := params["timeout"].(float64); ok {
		result.Timeout = int(timeout)
	} else if timeout, ok := params["timeout"].(int); ok {
		result.Timeout = timeout
	}

	return result, nil
}

func (t *AgentCallTool) getRequesterAgentID(ctx context.Context) string {
	// 1. 优先从 context 读取 "agent_id"（由 ParseAgentSessionKey 设置）
	if agentID := ctx.Value("agent_id"); agentID != nil {
		if id, ok := agentID.(string); ok && id != "" {
			logger.Info("getRequesterAgentID from context", zap.String("agent_id", id))
			return id
		}
	}
	// 2. Fallback: 通过 session key + getAgentID getter 获取
	//    corporate/swarm 模式的 session key 格式不被 ParseAgentSessionKey 识别，
	//    但 createRoleAgent 通过 SetAgentIDGetter 设置了正确的 getter
	if t.getAgentID != nil {
		if sessionKey, ok := ctx.Value(SessionKeyContextKey).(string); ok && sessionKey != "" {
			if id := t.getAgentID(sessionKey); id != "" {
				logger.Info("getRequesterAgentID from getAgentID getter", zap.String("agent_id", id), zap.String("session_key", sessionKey))
				return id
			}
		}
	}
	logger.Info("getRequesterAgentID: no agent_id found")
	return ""
}

func (t *AgentCallTool) checkPermission(requesterID, targetID string) bool {
	agentCfg := t.getAgentConfig(requesterID)
	if agentCfg == nil {
		logger.Warn("无法获取agent配置", zap.String("requester", requesterID))
		return false
	}

	if agentCfg.AgentCall == nil {
		return false
	}

	for _, allowed := range agentCfg.AgentCall.AllowAgents {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(allowed, targetID) {
			return true
		}
	}

	return false
}

func (t *AgentCallTool) formatSuccess(agentName, response string, durationMs int64) string {
	result := AgentCallResponse{
		Success:   true,
		AgentName: agentName,
		Response:  response,
		Duration:  durationMs,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data)
}
