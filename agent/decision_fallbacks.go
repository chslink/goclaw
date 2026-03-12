package agent

import (
	"strings"

	"github.com/smallnest/goclaw/decision"
)

// registerBuiltinFallbacks 注册所有内置决策类型的回退函数（从现有硬编码逻辑迁移）
func (da *DecisionAgent) registerBuiltinFallbacks() {
	da.fallbacks[DecisionCronOneShot] = fallbackCronOneShot
	da.fallbacks[DecisionMessageFilter] = fallbackMessageFilter
	da.fallbacks[DecisionToolDisable] = fallbackToolDisable
	da.fallbacks[DecisionIdentityValid] = fallbackIdentityValid
}

// fallbackCronOneShot 原 isCronOneShotRequest 逻辑
func fallbackCronOneShot(input string, _ map[string]string) decision.DecisionResult {
	if input == "" {
		return decision.DecisionResult{Decision: false, Confidence: 1.0, Reason: "empty input"}
	}
	normalized := strings.ToLower(strings.TrimSpace(input))
	if strings.Contains(normalized, "cron run") {
		return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "contains 'cron run'"}
	}
	keywords := []string{
		"执行一次定时任务",
		"只测试一次定时任务",
		"手工执行一次定时任务",
		"临时执行一次定时任务",
		"测试一次定时任务",
	}
	for _, kw := range keywords {
		if strings.Contains(normalized, kw) {
			return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "keyword match: " + kw}
		}
	}
	return decision.DecisionResult{Decision: false, Confidence: 1.0, Reason: "no keyword matched"}
}

// fallbackMessageFilter 原 isFilteredContent 逻辑
func fallbackMessageFilter(input string, _ map[string]string) decision.DecisionResult {
	if input == "" {
		return decision.DecisionResult{Decision: false, Confidence: 1.0, Reason: "empty input"}
	}

	rejectionPatterns := []string{
		"作为一个人工智能语言模型",
		"作为AI语言模型",
		"作为一个AI",
		"作为一个人工智能",
		"我还没有学习",
		"我还没学习",
		"我无法回答",
		"我不能回答",
		"I'm sorry, but I cannot",
		"As an AI language model",
		"As an AI assistant",
		"I cannot answer",
		"I'm not able to answer",
		"I cannot provide",
	}

	contentLower := strings.ToLower(input)
	for _, pattern := range rejectionPatterns {
		if strings.Contains(input, pattern) || strings.Contains(contentLower, strings.ToLower(pattern)) {
			return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "rejection pattern: " + pattern}
		}
	}

	if strings.Contains(input, "An unknown error occurred") {
		return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "unknown error message"}
	}

	if strings.Contains(input, "工具执行失败") ||
		strings.Contains(input, "Tool execution failed") ||
		(strings.Contains(input, "## ") && strings.Contains(input, "**错误**")) {
		return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "tool execution error"}
	}

	techErrorPatterns := []string{
		"context deadline exceeded",
		"context canceled",
		"connection refused",
		"network error",
	}
	for _, pattern := range techErrorPatterns {
		if strings.Contains(contentLower, pattern) {
			return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "tech error: " + pattern}
		}
	}

	return decision.DecisionResult{Decision: false, Confidence: 1.0, Reason: "no filter pattern matched"}
}

// fallbackToolDisable 原 context.go 中的工具禁用检测逻辑
func fallbackToolDisable(input string, _ map[string]string) decision.DecisionResult {
	if strings.Contains(input, "禁止使用任何工具") ||
		strings.Contains(input, "禁止使用工具") ||
		strings.Contains(input, "FORBIDDEN to use any tools") ||
		strings.Contains(input, "cannot use any tools") {
		return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "keyword match"}
	}
	return decision.DecisionResult{Decision: false, Confidence: 1.0, Reason: "no disable keyword found"}
}

// fallbackIdentityValid 原 context.go 中的身份有效性检测逻辑
func fallbackIdentityValid(input string, _ map[string]string) decision.DecisionResult {
	if input == "" {
		return decision.DecisionResult{Decision: false, Confidence: 1.0, Reason: "empty identity"}
	}
	if strings.Contains(input, "选一个你喜欢的") || strings.Contains(input, "在第一次对话中填写") {
		return decision.DecisionResult{Decision: false, Confidence: 1.0, Reason: "placeholder text detected"}
	}
	return decision.DecisionResult{Decision: true, Confidence: 1.0, Reason: "valid custom identity"}
}
