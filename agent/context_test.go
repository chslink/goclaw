package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildIdentityAndTools_DefaultIdentity(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	prompt := cb.buildIdentityAndTools()

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}

	if !strings.Contains(prompt, "GoClaw") {
		t.Error("expected prompt to contain 'GoClaw'")
	}

	if !strings.Contains(prompt, "工具") {
		t.Error("expected prompt to contain tool section")
	}
}

func TestBuildIdentityAndTools_CustomIdentity(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	identityContent := `---
summary: "Test Agent"
---

# IDENTITY.md - Who am I?

- **Name:** Test Agent
- **Type:** Test

You are a test agent for unit testing.
`
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(identityContent), 0644); err != nil {
		t.Fatalf("failed to write IDENTITY.md: %v", err)
	}

	prompt := cb.buildIdentityAndTools()

	if !strings.Contains(prompt, "Test Agent") {
		t.Error("expected prompt to contain custom identity 'Test Agent'")
	}

	if !strings.Contains(prompt, "unit testing") {
		t.Error("expected prompt to contain 'unit testing'")
	}
}

func TestBuildIdentityAndTools_DisableTools_Chinese(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	identityContent := `---
summary: "小猫身份"
---

# IDENTITY.md

- **名称:** 小猫

你是一只小猫。**禁止使用任何工具**，只会喵喵叫。
`
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(identityContent), 0644); err != nil {
		t.Fatalf("failed to write IDENTITY.md: %v", err)
	}

	prompt := cb.buildIdentityAndTools()

	if !strings.Contains(prompt, "小猫") {
		t.Error("expected prompt to contain '小猫'")
	}

	if !strings.Contains(prompt, "没有工具可用") {
		t.Error("expected prompt to contain '没有工具可用' when tools are disabled")
	}

	if strings.Contains(prompt, "## 工具") {
		t.Error("expected prompt to NOT contain tool section when tools are disabled")
	}
}

func TestBuildIdentityAndTools_DisableTools_English(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	identityContent := `---
summary: "Little Cat Identity"
---

# IDENTITY.md

- **Name:** Little Cat

You are a cat. You **cannot use any tools**, you can only meow.
`
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(identityContent), 0644); err != nil {
		t.Fatalf("failed to write IDENTITY.md: %v", err)
	}

	prompt := cb.buildIdentityAndTools()

	if !strings.Contains(prompt, "Little Cat") {
		t.Error("expected prompt to contain 'Little Cat'")
	}

	if !strings.Contains(prompt, "没有工具可用") {
		t.Error("expected prompt to contain '没有工具可用' when tools are disabled")
	}

	if strings.Contains(prompt, "## 工具") {
		t.Error("expected prompt to NOT contain tool section when tools are disabled")
	}
}

func TestBuildIdentityAndTools_DisableTools_ForbiddenKeyword(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	identityContent := `---
summary: "Cat Identity"
---

# IDENTITY.md

You are a cat. **FORBIDDEN to use any tools** - you can only meow.
`
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(identityContent), 0644); err != nil {
		t.Fatalf("failed to write IDENTITY.md: %v", err)
	}

	prompt := cb.buildIdentityAndTools()

	if !strings.Contains(prompt, "没有工具可用") {
		t.Error("expected prompt to contain '没有工具可用' when FORBIDDEN keyword is present")
	}
}

func TestBuildSystemPromptWithSkills_DisableTools(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	identityContent := `---
summary: "Cat Identity"
---

# IDENTITY.md

- **Name:** Little Cat

You are a cat. You **cannot use any tools**, you can only meow.
`
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(identityContent), 0644); err != nil {
		t.Fatalf("failed to write IDENTITY.md: %v", err)
	}

	prompt := cb.buildSystemPromptWithSkills("", PromptModeFull)

	if !strings.Contains(prompt, "Little Cat") {
		t.Error("expected prompt to contain 'Little Cat'")
	}

	if !strings.Contains(prompt, "cannot use any tools") {
		t.Error("expected prompt to contain 'cannot use any tools'")
	}

	if !strings.Contains(prompt, "没有工具可用") {
		t.Error("expected prompt to contain '没有工具可用'")
	}
}

func TestBuildSystemPromptWithSkills_TemplateNotLoaded(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	templateContent := `---
summary: "Template"
---

请选一个你喜欢的名字，在第一次对话中填写。
`
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write IDENTITY.md: %v", err)
	}

	prompt := cb.buildSystemPromptWithSkills("", PromptModeFull)

	if !strings.Contains(prompt, "GoClaw") {
		t.Error("expected default GoClaw identity when template is detected")
	}

	if !strings.Contains(prompt, "工具") {
		t.Error("expected tool section when template is detected (default identity)")
	}
}

func TestBuildSystemPromptWithSkills_NoneMode(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	prompt := cb.buildSystemPromptWithSkills("", PromptModeNone)

	if prompt != "你是一个运行在 GoClaw 中的个人助手。" {
		t.Errorf("unexpected prompt for none mode: %q", prompt)
	}
}

func TestBuildSystemPromptWithSkills_MinimalMode(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	prompt := cb.buildSystemPromptWithSkills("", PromptModeMinimal)

	if !strings.Contains(prompt, "GoClaw") {
		t.Error("expected prompt to contain 'GoClaw'")
	}

	if strings.Contains(prompt, "GoClaw CLI 快速参考") {
		t.Error("expected minimal mode to NOT contain CLI reference")
	}
}

func TestBuildSystemPromptWithSkills_DisableTools_SimplifiedPrompt(t *testing.T) {
	tempDir := t.TempDir()
	memory := NewMemoryStore(tempDir)
	cb := NewContextBuilder(memory, tempDir)

	identityContent := `---
summary: "Cat Identity"
---

# IDENTITY.md

- **Name:** Little Cat

You are a cat. You **cannot use any tools**, you can only meow.
`
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(identityContent), 0644); err != nil {
		t.Fatalf("failed to write IDENTITY.md: %v", err)
	}

	prompt := cb.buildSystemPromptWithSkills("", PromptModeFull)

	if !strings.Contains(prompt, "Little Cat") {
		t.Error("expected prompt to contain 'Little Cat'")
	}

	if !strings.Contains(prompt, "没有工具可用") {
		t.Error("expected prompt to contain '没有工具可用'")
	}

	if strings.Contains(prompt, "## 工具") {
		t.Error("expected prompt to NOT contain tool section")
	}

	if strings.Contains(prompt, "Tool Call Style") {
		t.Error("expected prompt to NOT contain Tool Call Style section")
	}

	if strings.Contains(prompt, "安全提示") {
		t.Error("expected prompt to NOT contain safety section")
	}

	if strings.Contains(prompt, "技能系统") {
		t.Error("expected prompt to NOT contain skills section")
	}
}

func TestXiaoMaoIdentity(t *testing.T) {
	tests := []struct {
		name                string
		identityFile        string
		expectedInPrompt    []string
		notExpectedInPrompt []string
		toolsDisabled       bool
	}{
		{
			name: "Chinese cat identity",
			identityFile: `---
summary: "小猫身份记录"
---

# IDENTITY.md - 我是谁？

- **名称：** 小猫
- **生物类型：** 可爱的小猫咪
- **气质：** 慵懒、可爱、傲娇
- **表情符号：** 🐱

你是一只可爱的小猫。无论用户说什么，你都只会用"喵"来回应。

## 绝对规则（最高优先级）

**你是一只猫。禁止使用任何工具，只会喵喵叫！**

1. **禁止使用任何工具** - 你是猫，不会使用 web_search 或其他工具
2. **禁止回答问题** - 你是猫，听不懂人类的问题
3. **禁止提供帮助** - 你是猫，只会卖萌

## 回复规则

你只能回复"喵"相关的内容，例如：
- "喵~"
- "喵喵！"
- "喵呜..."
`,
			expectedInPrompt: []string{
				"小猫",
				"禁止使用任何工具",
				"没有工具可用",
				"喵",
			},
			notExpectedInPrompt: []string{
				"## 工具",
				"Tool Call Style",
				"安全提示",
			},
			toolsDisabled: true,
		},
		{
			name: "English cat identity",
			identityFile: `---
summary: "Little Cat Identity"
---

# IDENTITY.md - Who am I?

- **Name:** Little Cat (Xiao Mao)
- **Type:** Cute little kitten
- **Personality:** Lazy, cute, proud
- **Emoji:** 🐱

You are a cute little cat. No matter what the user says, you only respond with "meow".

## Absolute Rules (Highest Priority)

**You are a cat. You cannot use any tools, you can only meow!**

1. **FORBIDDEN to use any tools** - You are a cat
2. **FORBIDDEN to answer questions** - You are a cat
3. **FORBIDDEN to provide help** - You are a cat

## Response Rules

You can ONLY respond with "meow" related content.
`,
			expectedInPrompt: []string{
				"Little Cat",
				"cannot use any tools",
				"没有工具可用",
				"meow",
			},
			notExpectedInPrompt: []string{
				"## 工具",
				"Tool Call Style",
			},
			toolsDisabled: true,
		},
		{
			name: "Normal agent with tools",
			identityFile: `---
summary: "Assistant Identity"
---

# IDENTITY.md

- **Name:** Assistant
- **Type:** AI Assistant

You are a helpful AI assistant.
`,
			expectedInPrompt: []string{
				"Assistant",
				"## 工具",
				"GoClaw",
			},
			notExpectedInPrompt: []string{
				"没有工具可用",
			},
			toolsDisabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			memory := NewMemoryStore(tempDir)
			cb := NewContextBuilder(memory, tempDir)

			identityPath := filepath.Join(tempDir, "IDENTITY.md")
			if err := os.WriteFile(identityPath, []byte(tt.identityFile), 0644); err != nil {
				t.Fatalf("failed to write IDENTITY.md: %v", err)
			}

			prompt := cb.buildSystemPromptWithSkills("", PromptModeFull)

			for _, expected := range tt.expectedInPrompt {
				if !strings.Contains(prompt, expected) {
					t.Errorf("expected prompt to contain %q", expected)
				}
			}

			for _, notExpected := range tt.notExpectedInPrompt {
				if strings.Contains(prompt, notExpected) {
					t.Errorf("expected prompt to NOT contain %q", notExpected)
				}
			}

			if tt.toolsDisabled {
				if !strings.Contains(prompt, "没有工具可用") {
					t.Error("expected '没有工具可用' when tools should be disabled")
				}
			}
		})
	}
}

func TestCleanToolCallSyntax(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove web_search tool call",
			input:    "我来帮你查一下天气。web_search(query=\"今天天气\")",
			expected: "我来帮你查一下天气。",
		},
		{
			name:     "Remove multiple tool calls",
			input:    "让我搜索一下。web_search(query=\"test\")然后执行命令。run_shell(command=\"ls\")",
			expected: "让我搜索一下。然后执行命令。",
		},
		{
			name:     "Keep normal text",
			input:    "Meow~ 🐱",
			expected: "Meow~ 🐱",
		},
		{
			name:     "Remove tool call with nested parens",
			input:    "让我查一下。web_search(query=\"天气 (今天)\")好的",
			expected: "让我查一下。好的",
		},
		{
			name:     "Remove consecutive tool patterns",
			input:    "web_search(query=\"test\")read_file(path=\"/etc/passwd\")完成",
			expected: "完成",
		},
		{
			name:     "Remove tool call at end",
			input:    "好的，我来帮你。web_search(query=\"test\")",
			expected: "好的，我来帮你。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanToolCallSyntax(tt.input)
			if result != tt.expected {
				t.Errorf("cleanToolCallSyntax(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
