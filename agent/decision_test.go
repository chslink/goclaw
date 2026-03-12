package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/decision"
	"github.com/smallnest/goclaw/providers"
)

// mockProvider 模拟 LLM provider 用于测试
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, options ...providers.ChatOption) (*providers.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &providers.Response{
		Content: m.response,
	}, nil
}

func (m *mockProvider) ChatWithTools(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, options ...providers.ChatOption) (*providers.Response, error) {
	return m.Chat(ctx, messages, tools, options...)
}

func (m *mockProvider) Close() error { return nil }

// TestDecisionAgent_FallbackOnly 测试无 LLM 时走 fallback
func TestDecisionAgent_FallbackOnly(t *testing.T) {
	da := NewDecisionAgent(nil, nil)

	tests := []struct {
		name       string
		dtype      string
		input      string
		wantDecide bool
		wantSource string
	}{
		{"cron hit", string(DecisionCronOneShot), "执行一次定时任务", true, "fallback"},
		{"cron miss", string(DecisionCronOneShot), "帮我看下天气", false, "fallback"},
		{"cron run", string(DecisionCronOneShot), "cron run job-abc", true, "fallback"},
		{"message filter hit", string(DecisionMessageFilter), "作为一个AI我无法回答", true, "fallback"},
		{"message filter miss", string(DecisionMessageFilter), "今天天气不错", false, "fallback"},
		{"tool disable hit", string(DecisionToolDisable), "禁止使用任何工具", true, "fallback"},
		{"tool disable miss", string(DecisionToolDisable), "你是一只猫", false, "fallback"},
		{"identity valid", string(DecisionIdentityValid), "你是一个名叫小明的AI助手", true, "fallback"},
		{"identity placeholder", string(DecisionIdentityValid), "选一个你喜欢的身份", false, "fallback"},
		{"identity empty", string(DecisionIdentityValid), "", false, "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := da.Decide(context.Background(), tt.dtype, tt.input, nil)
			if err != nil {
				t.Fatalf("Decide() error: %v", err)
			}
			if result.Decision != tt.wantDecide {
				t.Errorf("Decision = %v, want %v", result.Decision, tt.wantDecide)
			}
			if result.Source != tt.wantSource {
				t.Errorf("Source = %q, want %q", result.Source, tt.wantSource)
			}
		})
	}
}

// TestDecisionAgent_LLMPath 测试 LLM 路径
func TestDecisionAgent_LLMPath(t *testing.T) {
	resp := decision.DecisionResult{
		Decision:   true,
		Confidence: 0.95,
		Reason:     "looks like cron request",
	}
	respJSON, _ := json.Marshal(resp)

	mock := &mockProvider{response: string(respJSON)}
	cfg := &config.DecisionAgentConfig{
		Enabled:        true,
		Model:          "test-model",
		TimeoutMs:      5000,
		MaxTokens:      256,
		Temperature:    0.1,
		FallbackOnFail: true,
	}

	da := NewDecisionAgent(cfg, mock)

	result, err := da.Decide(context.Background(), string(DecisionCronOneShot), "跑一下cron", nil)
	if err != nil {
		t.Fatalf("Decide() error: %v", err)
	}
	if !result.Decision {
		t.Error("expected Decision=true from LLM")
	}
	if result.Source != "llm" {
		t.Errorf("Source = %q, want %q", result.Source, "llm")
	}
	if result.Confidence < 0.9 {
		t.Errorf("Confidence = %f, expected > 0.9", result.Confidence)
	}
}

// TestDecisionAgent_LLMFailFallback 测试 LLM 失败时走 fallback
func TestDecisionAgent_LLMFailFallback(t *testing.T) {
	mock := &mockProvider{err: fmt.Errorf("network error")}
	cfg := &config.DecisionAgentConfig{
		Enabled:        true,
		Model:          "test-model",
		TimeoutMs:      5000,
		FallbackOnFail: true,
	}

	da := NewDecisionAgent(cfg, mock)

	result, err := da.Decide(context.Background(), string(DecisionCronOneShot), "执行一次定时任务", nil)
	if err != nil {
		t.Fatalf("Decide() should not error with fallback_on_fail=true, got: %v", err)
	}
	if !result.Decision {
		t.Error("expected Decision=true from fallback")
	}
	if result.Source != "fallback" {
		t.Errorf("Source = %q, want %q", result.Source, "fallback")
	}
}

// TestDecisionAgent_LLMFailNoFallback 测试 LLM 失败且 fallback_on_fail=false 时返回错误
func TestDecisionAgent_LLMFailNoFallback(t *testing.T) {
	mock := &mockProvider{err: fmt.Errorf("network error")}
	cfg := &config.DecisionAgentConfig{
		Enabled:        true,
		Model:          "test-model",
		TimeoutMs:      5000,
		FallbackOnFail: false,
	}

	da := NewDecisionAgent(cfg, mock)

	_, err := da.Decide(context.Background(), string(DecisionCronOneShot), "执行一次定时任务", nil)
	if err == nil {
		t.Fatal("expected error when LLM fails and fallback_on_fail=false")
	}
}

// TestDecisionAgent_TypeNotEnabled 测试类型未启用时走 fallback
func TestDecisionAgent_TypeNotEnabled(t *testing.T) {
	resp := decision.DecisionResult{Decision: true, Confidence: 1.0}
	respJSON, _ := json.Marshal(resp)
	mock := &mockProvider{response: string(respJSON)}

	cfg := &config.DecisionAgentConfig{
		Enabled:      true,
		Model:        "test-model",
		TimeoutMs:    5000,
		EnabledTypes: []string{"cron_oneshot"}, // 只启用 cron_oneshot
	}

	da := NewDecisionAgent(cfg, mock)

	// message_filter 未启用，应该走 fallback
	result, err := da.Decide(context.Background(), string(DecisionMessageFilter), "作为AI我无法回答", nil)
	if err != nil {
		t.Fatalf("Decide() error: %v", err)
	}
	if result.Source != "fallback" {
		t.Errorf("Source = %q, want %q for disabled type", result.Source, "fallback")
	}
	if !result.Decision {
		t.Error("expected Decision=true from fallback")
	}
}

// TestDecisionAgent_JSONInMarkdown 测试 LLM 返回被 markdown 包裹的 JSON
func TestDecisionAgent_JSONInMarkdown(t *testing.T) {
	mock := &mockProvider{
		response: "```json\n{\"decision\": true, \"confidence\": 0.9, \"reason\": \"test\"}\n```",
	}
	cfg := &config.DecisionAgentConfig{
		Enabled:        true,
		Model:          "test-model",
		TimeoutMs:      5000,
		FallbackOnFail: true,
	}

	da := NewDecisionAgent(cfg, mock)

	result, err := da.Decide(context.Background(), string(DecisionCronOneShot), "执行一次定时任务", nil)
	if err != nil {
		t.Fatalf("Decide() error: %v", err)
	}
	if !result.Decision {
		t.Error("expected Decision=true from markdown-wrapped JSON")
	}
	if result.Source != "llm" {
		t.Errorf("Source = %q, want %q", result.Source, "llm")
	}
}

// TestCronOneShotFallback_Compatibility 验证 fallback 与原始 isCronOneShotRequest 行为一致
func TestCronOneShotFallback_Compatibility(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "执行一次定时任务", want: true},
		{text: "只测试一次定时任务", want: true},
		{text: "手工执行一次定时任务", want: true},
		{text: "临时执行一次定时任务", want: true},
		{text: "测试一次定时任务", want: true},
		{text: "cron run job-abc123", want: true},
		{text: "CRON RUN job-xyz", want: true},
		{text: "帮我看下天气", want: false},
		{text: "创建一个定时任务", want: false},
		{text: "", want: false},
	}

	da := NewDecisionAgent(nil, nil) // fallback-only mode

	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			result, _ := da.Decide(context.Background(), string(DecisionCronOneShot), tc.text, nil)

			// 与原始函数行为对比
			original := isCronOneShotRequest(tc.text)
			if result.Decision != original {
				t.Errorf("fallback(%q)=%v but isCronOneShotRequest=%v", tc.text, result.Decision, original)
			}
			if result.Decision != tc.want {
				t.Errorf("fallback(%q)=%v, want %v", tc.text, result.Decision, tc.want)
			}
		})
	}
}

// TestMessageFilterFallback_Compatibility 验证 message filter fallback 与原始 isFilteredContent 行为一致
func TestMessageFilterFallback_Compatibility(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"作为一个人工智能语言模型，我无法回答这个问题", true},
		{"As an AI language model, I cannot do that", true},
		{"I'm sorry, but I cannot help with that", true},
		{"An unknown error occurred", true},
		{"工具执行失败: timeout", true},
		{"context deadline exceeded", true},
		{"connection refused", true},
		{"今天天气不错", false},
		{"这是你的报告...", false},
		{"", false},
	}

	da := NewDecisionAgent(nil, nil) // fallback-only mode

	for _, tc := range cases {
		t.Run(tc.input[:min(len(tc.input), 30)], func(t *testing.T) {
			result, _ := da.Decide(context.Background(), string(DecisionMessageFilter), tc.input, nil)
			if result.Decision != tc.want {
				t.Errorf("fallback(%q)=%v, want %v", tc.input, result.Decision, tc.want)
			}
		})
	}
}

// TestDecisionAgent_DeciderInterface 验证 DecisionAgent 实现了 decision.Decider 接口
func TestDecisionAgent_DeciderInterface(t *testing.T) {
	da := NewDecisionAgent(nil, nil)
	var _ decision.Decider = da // 编译时检查
}
