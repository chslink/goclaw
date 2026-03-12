package decision

import "context"

// DecisionResult 决策结果
type DecisionResult struct {
	Decision   bool           `json:"decision"`
	Confidence float64        `json:"confidence"`
	Reason     string         `json:"reason"`
	Details    map[string]any `json:"details,omitempty"`
	Source     string         `json:"source"`     // "llm" | "fallback"
	LatencyMs  int64          `json:"latency_ms"`
}

// Decider 跨包决策接口，解决 channels 包不能导入 agent 包的循环依赖
type Decider interface {
	// Decide 执行一次语义决策。decisionType 标识决策类型（如 "cron_oneshot"、"message_filter"），
	// input 是待判断的文本，ctx 是附加上下文信息。
	Decide(ctx context.Context, decisionType string, input string, extra map[string]string) (DecisionResult, error)
}
