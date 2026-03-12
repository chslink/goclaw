package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/decision"
	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/providers"
	"go.uber.org/zap"
)

// DecisionType 决策类型
type DecisionType string

const (
	DecisionCronOneShot   DecisionType = "cron_oneshot"
	DecisionMessageFilter DecisionType = "message_filter"
	DecisionToolDisable   DecisionType = "tool_disable"
	DecisionIdentityValid DecisionType = "identity_valid"
)

// DecisionFallback 回退函数签名
type DecisionFallback func(input string, ctx map[string]string) decision.DecisionResult

// DecisionAgent 轻量级决策组件，使用 flash 模型做单次 LLM 调用
// 不是 Agent 实例，不走 Orchestrator 循环
type DecisionAgent struct {
	provider     providers.Provider
	config       *config.DecisionAgentConfig
	prompts      map[DecisionType]string
	fallbacks    map[DecisionType]DecisionFallback
	enabledTypes map[string]bool
}

// NewDecisionAgent 创建 DecisionAgent
func NewDecisionAgent(cfg *config.DecisionAgentConfig, provider providers.Provider) *DecisionAgent {
	da := &DecisionAgent{
		provider:     provider,
		config:       cfg,
		prompts:      make(map[DecisionType]string),
		fallbacks:    make(map[DecisionType]DecisionFallback),
		enabledTypes: make(map[string]bool),
	}

	// 构建启用类型索引
	if cfg != nil {
		for _, t := range cfg.EnabledTypes {
			da.enabledTypes[t] = true
		}
	}

	// 注册所有内置 prompt 和 fallback
	da.registerBuiltinPrompts()
	da.registerBuiltinFallbacks()

	return da
}

// RegisterPrompt 注册决策类型的 prompt 模板
func (da *DecisionAgent) RegisterPrompt(dt DecisionType, prompt string) {
	da.prompts[dt] = prompt
}

// RegisterFallback 注册决策类型的回退函数
func (da *DecisionAgent) RegisterFallback(dt DecisionType, fb DecisionFallback) {
	da.fallbacks[dt] = fb
}

// Decide 执行决策，实现 decision.Decider 接口
func (da *DecisionAgent) Decide(ctx context.Context, decisionType string, input string, extra map[string]string) (decision.DecisionResult, error) {
	dt := DecisionType(decisionType)
	start := time.Now()

	// 如果 DecisionAgent 未启用或该类型未启用，直接走 fallback
	if da == nil || da.config == nil || !da.config.Enabled || !da.isTypeEnabled(dt) {
		return da.executeFallback(dt, input, extra, start), nil
	}

	// 获取 prompt 模板
	prompt, ok := da.prompts[dt]
	if !ok {
		return da.executeFallback(dt, input, extra, start), nil
	}

	// 构建消息
	messages := []providers.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: da.buildUserMessage(dt, input, extra)},
	}

	// 设置超时
	timeoutMs := da.config.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 2000
	}
	llmCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// 构建 ChatOptions
	opts := da.buildChatOptions()

	// 调用 LLM（无 tools）
	resp, err := da.provider.Chat(llmCtx, messages, nil, opts...)
	if err != nil {
		logger.Warn("DecisionAgent LLM call failed, falling back",
			zap.String("type", decisionType),
			zap.Error(err))
		if da.config.FallbackOnFail {
			return da.executeFallback(dt, input, extra, start), nil
		}
		return decision.DecisionResult{
			Source:    "error",
			LatencyMs: time.Since(start).Milliseconds(),
		}, fmt.Errorf("decision LLM call failed: %w", err)
	}

	// 解析 JSON 响应
	result, err := da.parseResponse(resp.Content)
	if err != nil {
		logger.Warn("DecisionAgent response parse failed, falling back",
			zap.String("type", decisionType),
			zap.String("content", resp.Content),
			zap.Error(err))
		if da.config.FallbackOnFail {
			return da.executeFallback(dt, input, extra, start), nil
		}
		return decision.DecisionResult{
			Source:    "error",
			LatencyMs: time.Since(start).Milliseconds(),
		}, fmt.Errorf("decision response parse failed: %w", err)
	}

	result.Source = "llm"
	result.LatencyMs = time.Since(start).Milliseconds()

	logger.Debug("DecisionAgent decision made",
		zap.String("type", decisionType),
		zap.Bool("decision", result.Decision),
		zap.Float64("confidence", result.Confidence),
		zap.Int64("latency_ms", result.LatencyMs))

	return result, nil
}

// isTypeEnabled 检查决策类型是否启用
func (da *DecisionAgent) isTypeEnabled(dt DecisionType) bool {
	// 如果没有配置 enabled_types，默认全部启用
	if len(da.enabledTypes) == 0 {
		return true
	}
	return da.enabledTypes[string(dt)]
}

// executeFallback 执行回退函数
func (da *DecisionAgent) executeFallback(dt DecisionType, input string, extra map[string]string, start time.Time) decision.DecisionResult {
	if da != nil {
		if fb, ok := da.fallbacks[dt]; ok {
			result := fb(input, extra)
			result.Source = "fallback"
			result.LatencyMs = time.Since(start).Milliseconds()
			return result
		}
	}
	// 无 fallback 时返回否定结果
	return decision.DecisionResult{
		Decision:  false,
		Source:    "fallback",
		Reason:    "no fallback registered",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

// buildUserMessage 构建用户消息
func (da *DecisionAgent) buildUserMessage(dt DecisionType, input string, extra map[string]string) string {
	var sb strings.Builder
	sb.WriteString(input)
	if len(extra) > 0 {
		sb.WriteString("\n\n---\nContext:\n")
		for k, v := range extra {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
		}
	}
	return sb.String()
}

// buildChatOptions 构建 LLM 调用选项
func (da *DecisionAgent) buildChatOptions() []providers.ChatOption {
	var opts []providers.ChatOption
	if da.config.Model != "" {
		opts = append(opts, providers.WithModel(da.config.Model))
	}
	temp := da.config.Temperature
	if temp <= 0 {
		temp = 0.1
	}
	opts = append(opts, providers.WithTemperature(temp))
	maxTokens := da.config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 256
	}
	opts = append(opts, providers.WithMaxTokens(maxTokens))
	return opts
}

// parseResponse 解析 LLM 响应为 DecisionResult
func (da *DecisionAgent) parseResponse(content string) (decision.DecisionResult, error) {
	content = strings.TrimSpace(content)

	// 尝试提取 JSON（可能被 markdown 代码块包裹）
	if idx := strings.Index(content, "{"); idx >= 0 {
		if endIdx := strings.LastIndex(content, "}"); endIdx > idx {
			content = content[idx : endIdx+1]
		}
	}

	var result decision.DecisionResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return decision.DecisionResult{}, fmt.Errorf("invalid JSON response: %w", err)
	}

	return result, nil
}

// CreateDecisionAgentProvider 根据 DecisionAgentConfig 创建独立的 provider
func CreateDecisionAgentProvider(dcfg *config.DecisionAgentConfig, mainCfg *config.Config) (providers.Provider, error) {
	// 确定 provider 类型和凭证
	providerType := dcfg.Provider
	apiKey := dcfg.APIKey
	baseURL := dcfg.BaseURL

	// 如果未指定，从主配置中推断
	if providerType == "" {
		providerType = "openrouter"
	}
	if apiKey == "" {
		switch providerType {
		case "openrouter":
			apiKey = mainCfg.Providers.OpenRouter.APIKey
		case "openai":
			apiKey = mainCfg.Providers.OpenAI.APIKey
		case "anthropic":
			apiKey = mainCfg.Providers.Anthropic.APIKey
		}
	}
	if baseURL == "" {
		switch providerType {
		case "openrouter":
			baseURL = mainCfg.Providers.OpenRouter.BaseURL
		case "openai":
			baseURL = mainCfg.Providers.OpenAI.BaseURL
		case "anthropic":
			baseURL = mainCfg.Providers.Anthropic.BaseURL
		}
	}

	model := dcfg.Model
	if model == "" {
		model = "google/gemini-2.0-flash"
	}
	maxTokens := dcfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 256
	}

	return providers.CreateProviderByType(providerType, apiKey, baseURL, model, maxTokens)
}
