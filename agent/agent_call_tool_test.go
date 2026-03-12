package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/smallnest/goclaw/bus"
	"github.com/smallnest/goclaw/config"
)

func TestAgentCallTool_Name(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))
	if tool.Name() != "agent_call" {
		t.Errorf("expected name 'agent_call', got '%s'", tool.Name())
	}
}

func TestAgentCallTool_Description(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestAgentCallTool_Label(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))
	label := tool.Label()
	if label == "" {
		t.Error("expected non-empty label")
	}
}

func TestAgentCallTool_Parameters(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("expected type 'object', got '%v'", params["type"])
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Error("expected properties to be a map")
		return
	}

	if _, ok := props["agent_name"]; !ok {
		t.Error("expected agent_name property")
	}

	if _, ok := props["message"]; !ok {
		t.Error("expected message property")
	}

	if _, ok := props["timeout"]; !ok {
		t.Error("expected timeout property")
	}
}

func TestAgentCallTool_ParseParams(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))

	tests := []struct {
		name     string
		input    map[string]any
		expected *AgentCallParams
		hasError bool
	}{
		{
			name: "valid params",
			input: map[string]any{
				"agent_name": "zhongshu",
				"message":   "你好",
				"timeout":   30,
			},
			expected: &AgentCallParams{
				AgentName: "zhongshu",
				Message:   "你好",
				Timeout:   30,
			},
			hasError: false,
		},
		{
			name: "missing agent_name",
			input: map[string]any{
				"message": "你好",
			},
			expected: nil,
			hasError: true,
		},
		{
			name: "missing message",
			input: map[string]any{
				"agent_name": "zhongshu",
			},
			expected: nil,
			hasError: true,
		},
		{
			name: "default timeout",
			input: map[string]any{
				"agent_name": "zhongshu",
				"message":   "你好",
			},
			expected: &AgentCallParams{
				AgentName: "zhongshu",
				Message:   "你好",
				Timeout:   300,
			},
			hasError: false,
		},
		{
			name: "timeout as float64",
			input: map[string]any{
				"agent_name": "zhongshu",
				"message":   "你好",
				"timeout":   30.0,
			},
			expected: &AgentCallParams{
				AgentName: "zhongshu",
				Message:   "你好",
				Timeout:   30,
			},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.parseParams(tt.input)

			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if result.AgentName != tt.expected.AgentName {
					t.Errorf("expected AgentName %q, got %q", tt.expected.AgentName, result.AgentName)
				}
				if result.Message != tt.expected.Message {
					t.Errorf("expected Message %q, got %q", tt.expected.Message, result.Message)
				}
				if result.Timeout != tt.expected.Timeout {
					t.Errorf("expected Timeout %d, got %d", tt.expected.Timeout, result.Timeout)
				}
			}
		})
	}
}

func TestAgentCallTool_CheckPermission(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))

	tests := []struct {
		name        string
		requesterID string
		targetID    string
		config      *config.AgentConfig
		expected    bool
	}{
		{
			name:        "allow all",
			requesterID: "taizi",
			targetID:    "zhongshu",
			config: &config.AgentConfig{
				AgentCall: &config.AgentCallConfig{
					AllowAgents: []string{"*"},
				},
			},
			expected: true,
		},
		{
			name:        "allow specific agent",
			requesterID: "taizi",
			targetID:    "zhongshu",
			config: &config.AgentConfig{
				AgentCall: &config.AgentCallConfig{
					AllowAgents: []string{"zhongshu", "menxia"},
				},
			},
			expected: true,
		},
		{
			name:        "deny specific agent",
			requesterID: "taizi",
			targetID:    "shangshu",
			config: &config.AgentConfig{
				AgentCall: &config.AgentCallConfig{
					AllowAgents: []string{"zhongshu", "menxia"},
				},
			},
			expected: false,
		},
		{
			name:        "no config",
			requesterID: "taizi",
			targetID:    "zhongshu",
			config:      nil,
			expected:    false,
		},
		{
			name:        "no agent_call config",
			requesterID: "taizi",
			targetID:    "zhongshu",
			config: &config.AgentConfig{
				AgentCall: nil,
			},
			expected: false,
		},
		{
			name:        "case insensitive",
			requesterID: "taizi",
			targetID:    "ZHONGSHU",
			config: &config.AgentConfig{
				AgentCall: &config.AgentCallConfig{
					AllowAgents: []string{"zhongshu"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool.SetAgentConfigGetter(func(agentID string) *config.AgentConfig {
				if agentID == tt.requesterID {
					return tt.config
				}
				return nil
			})

			result := tool.checkPermission(tt.requesterID, tt.targetID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAgentCallTool_FormatResponse(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))

	t.Run("format success", func(t *testing.T) {
		result := tool.formatSuccess("zhongshu", "你好", 1500)
		var resp AgentCallResponse
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if !resp.Success {
			t.Error("expected success to be true")
		}
		if resp.AgentName != "zhongshu" {
			t.Errorf("expected AgentName 'zhongshu', got '%s'", resp.AgentName)
		}
		if resp.Response != "你好" {
			t.Errorf("expected Response '你好', got '%s'", resp.Response)
		}
		if resp.Duration != 1500 {
			t.Errorf("expected Duration 1500, got %d", resp.Duration)
		}
	})
}

func TestAgentCallTool_GetRequesterAgentID(t *testing.T) {
	tool := NewAgentCallTool(bus.NewMessageBus(10))

	t.Run("with agent_id in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "agent_id", "taizi")
		result := tool.getRequesterAgentID(ctx)
		if result != "taizi" {
			t.Errorf("expected 'taizi', got '%s'", result)
		}
	})

	t.Run("without agent_id in context", func(t *testing.T) {
		ctx := context.Background()
		result := tool.getRequesterAgentID(ctx)
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})
}
