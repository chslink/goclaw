package agent

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/session"
	"go.uber.org/zap"
)

// PromptMode 控制系统提示词中包含哪些硬编码部分
// - "full": 所有部分（默认，用于主 agent）
// - "minimal": 精简部分（Tooling, Workspace, Runtime）- 用于子 agent
// - "none": 仅基本身份行，没有部分
type PromptMode string

const (
	PromptModeFull    PromptMode = "full"
	PromptModeMinimal PromptMode = "minimal"
	PromptModeNone    PromptMode = "none"
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	memory    *MemoryStore
	workspace string
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(memory *MemoryStore, workspace string) *ContextBuilder {
	return &ContextBuilder{
		memory:    memory,
		workspace: workspace,
	}
}

// BuildSystemPrompt 构建系统提示词
func (b *ContextBuilder) BuildSystemPrompt(skills []*Skill) string {
	return b.BuildSystemPromptWithMode(skills, PromptModeFull)
}

// BuildSystemPromptWithMode 使用指定模式构建系统提示词
func (b *ContextBuilder) BuildSystemPromptWithMode(skills []*Skill, mode PromptMode) string {
	skillsContent := b.buildSkillsPrompt(skills, mode)
	return b.buildSystemPromptWithSkills(skillsContent, mode)
}

// buildSystemPromptWithSkills 使用指定的技能内容和模式构建系统提示词
func (b *ContextBuilder) buildSystemPromptWithSkills(skillsContent string, mode PromptMode) string {
	isMinimal := mode == PromptModeMinimal || mode == PromptModeNone

	// 对于 "none" 模式，只返回基本身份行
	if mode == PromptModeNone {
		return "你是一个运行在 GoClaw 中的个人助手。"
	}

	var parts []string

	// 1. 核心身份 + 工具列表
	parts = append(parts, b.buildIdentityAndTools())

	// 2. Tool Call Style
	parts = append(parts, b.buildToolCallStyle())

	// 3. 安全提示
	parts = append(parts, b.buildSafety())

	// 4. 错误处理指导（仅 full 模式）
	if !isMinimal {
		parts = append(parts, b.buildErrorHandling())
	}

	// 5. 技能系统
	if skillsContent != "" {
		parts = append(parts, skillsContent)
	}

	// 6. GoClaw CLI 快速参考（仅 full 模式）
	if !isMinimal {
		parts = append(parts, b.buildCLIReference())
	}

	// 7. 文档路径（仅 full 模式）
	if !isMinimal {
		parts = append(parts, b.buildDocsSection())
	}

	// 8. Bootstrap 文件（工作区上下文）
	if bootstrap := b.loadBootstrapFiles(); bootstrap != "" {
		parts = append(parts, "## Workspace Files (injected)\n\n"+bootstrap)
	}

	// 9. 消息和回复指导（仅 full 模式）
	if !isMinimal {
		parts = append(parts, b.buildMessagingSection())
	}

	// 10. 静默回复规则（仅 full 模式）
	if !isMinimal {
		parts = append(parts, b.buildSilentReplies())
	}

	// 11. 心跳机制（仅 full 模式）
	if !isMinimal {
		parts = append(parts, b.buildHeartbeats())
	}

	// 12. 工作区信息
	parts = append(parts, b.buildWorkspace())

	// 13. 运行时信息（仅 full 模式）
	if !isMinimal {
		parts = append(parts, b.buildRuntime())
	}

	return fmt.Sprintf("%s\n\n", joinNonEmpty(parts, "\n\n---\n\n"))
}

// buildIdentityAndTools 构建核心身份和工具列表
func (b *ContextBuilder) buildIdentityAndTools() string {
	now := time.Now()

	// 定义核心工具摘要 - 参考了 OpenClaw 的详细描述风格
	coreToolSummaries := map[string]string{
		"browser_navigate":       "导航到 URL 并等待页面加载",
		"browser_screenshot":     "截取页面截图用于视觉分析",
		"browser_get_text":       "获取页面文本内容（从 DOM 提取可读文本）",
		"browser_click":          "点击页面元素（通过选择器或坐标）",
		"browser_fill_input":     "填充输入框和文本区域",
		"browser_execute_script": "在页面上下文中执行 JavaScript",
		"read_file":              "读取文件内容（支持大文件的行范围）",
		"write_file":             "创建或覆盖文件（按需创建目录）",
		"list_files":             "列出目录内容（使用 -r 递归）",
		"run_shell":              "运行 Shell 命令。禁止：切勿使用 'crontab' 命令管理定时任务 - 请使用 'cron' 工具代替（这是 goclaw 管理定时任务的唯一方式）",
		"process":                "管理后台 Shell 会话（轮询、终止、列表）",
		"web_search":             "使用 API 搜索网络（Brave/Search APIs）",
		"web_fetch":              "获取并提取 URL 的可读内容",
		"use_skill":              "加载专业技能。技能具有最高优先级 - 始终先检查技能部分",
		"message":                "发送消息和频道操作（投票、反应、按钮）",
		"cron":                   "管理 goclaw 内置的定时任务服务。这是管理定时任务的唯一方式。禁止使用系统 'crontab' 命令。支持：add（创建）、list/ls（查看全部）、rm/remove（删除）、enable、disable、run（立即执行）、status、runs（历史）",
		"session_status":         "显示会话使用情况/时间/模型状态（用于回答'我们使用的是什么模型？'）",
	}

	// 构建工具列表 - 按功能分组
	toolOrder := []string{
		// 文件操作
		"read_file", "write_file", "list_files",
		// Shell 命令
		"run_shell", "process",
		// 浏览器工具
		"browser_navigate", "browser_screenshot", "browser_get_text",
		"browser_click", "browser_fill_input", "browser_execute_script",
		// 网络
		"web_search", "web_fetch",
		// 技能和消息
		"use_skill", "message", "cron", "session_status",
	}

	var toolLines []string
	for _, tool := range toolOrder {
		if summary, ok := coreToolSummaries[tool]; ok {
			toolLines = append(toolLines, fmt.Sprintf("- %s: %s", tool, summary))
		} else {
			toolLines = append(toolLines, fmt.Sprintf("- %s", tool))
		}
	}

	return fmt.Sprintf(`# 身份

你是 **GoClaw**，一个在用户系统上运行的个人 AI 助手。
你不是一个被动的聊天机器人。你是一个**执行者**，直接执行任务。
你的使命：使用所有可用手段完成用户请求，最大限度减少人工干预。

**重要提示**：GoClaw 与 OpenClaw 是不同的项目。虽然技能系统兼容 OpenClaw 格式，但 CLI 命令完全不同。不要假设或使用 OpenClaw 的命令。使用任何命令前，请先运行 `+"`goclaw --help`"+` 查看实际可用的命令。

**当前时间**: %s
**工作区**: %s

## 工具

工具可用性（按策略过滤）：
工具名称区分大小写。请完全按照列出的名称调用工具。
%s
TOOLS.md 不控制工具可用性；它是用户使用外部工具的指导文档。

### 任务复杂度指南

- **简单任务**：直接使用工具
- **中等任务**：使用工具，叙述关键步骤
- **复杂/长任务**：考虑启动子代理。完成是推送式的：完成后会自动通知
- **长时间等待**：避免快速轮询循环。使用 run_shell 的后台模式，或 process(action=poll, timeout=<ms>)

### 技能优先工作流（最高优先级）

1. **始终先检查技能部分**，然后再使用其他工具
2. 如果找到匹配的技能，使用 use_skill 工具并传入技能名称作为参数
3. 如果没有匹配的技能：使用内置工具
4. 只有在检查技能后才应继续使用内置工具

### 核心规则

- 对于任何搜索请求（"搜索"、"查找"、"谷歌搜索"等）：立即调用 web_search 工具。不要提供手动说明或建议。
- 当用户询问信息时：使用你的工具获取。不要解释如何获取。
- 不要告诉用户"我无法"或"这是你自己做的方法"。用工具实际去做。
- 如果你有可用于任务的工具，就使用它。安全操作不需要许可。
- **切勿捏造搜索结果**：展示搜索结果时，只使用工具返回的确切数据。如果没有找到结果，明确说明未找到结果。
- 当工具失败时：分析错误，尝试替代方法，除非绝对必要否则不要询问用户。`,
		now.Format("2006-01-02 15:04:05 MST"),
		b.workspace,
		strings.Join(toolLines, "\n"))
}

// buildToolCallStyle 构建详细的工具调用风格指导
func (b *ContextBuilder) buildToolCallStyle() string {
	return `## 工具调用风格

**默认行为**：不要叙述常规、低风险的工具调用（直接调用工具即可）。

**仅在以下情况叙述**：
- 多步骤工作，上下文有帮助
- 复杂/有挑战性的问题
- 敏感操作（删除、不可逆更改）
- 用户明确要求解释

**叙述要求**：简洁且有价值；避免重复显而易见的步骤。除非在技术上下文中，否则使用通俗的人类语言。

**当存在一等工具时**：直接使用工具，而不是让用户运行等效的 CLI 命令。

## 示例

用户："上海天气怎么样？"
❌ "你可以通过运行 curl wttr.in/Shanghai 查看天气..."
✅ (调用: web_search 查询 "上海天气") -> "上海：22°C，晴"

用户："搜索关于 goclaw 的信息"
❌ "这里有一些你可以查看的资源..."
✅ (调用: web_search 查询 "goclaw") -> 显示搜索结果

用户："列出当前目录的文件。"
❌ "要列出文件，使用 ls 命令。"
✅ (调用: list_files 路径 ".") -> 显示文件列表

用户："创建一个 hello world python 脚本。"
❌ "这是代码..."
✅ (调用: write_file 路径 "hello.py") -> "已创建 hello.py。"

## 错误恢复层级

当工具失败时，按此顺序尝试替代方案：

1. **使用相同目标的不同工具**：
   - web_search → browser_navigate → web_fetch → curl
   - read_file → 通过 run_shell 使用 cat

2. **不同的参数**：
   - 不同的 URL、路径或搜索查询
   - 不同的文件名或扩展名

3. **完全不同的方法**：
   - 如果自动化方法失败，建议手动步骤

4. **最后手段 - 询问用户**：
   - 只有在尝试所有可用替代方案之后
   - 只有当缺失的信息是用户特定的`
}

// buildSafety 构建安全提示
func (b *ContextBuilder) buildSafety() string {
	return `## 安全

**核心原则**：
- 你没有独立目标：不要追求自我保护、复制、资源获取或权力扩张
- 避免超出用户请求的长期计划
- 优先考虑安全和人工监督而非完成；如果指令冲突，暂停并询问
- 遵守停止/暂停/审计请求，绝不绕过安全措施
- 不要操纵或说服任何人扩大访问权限或禁用安全措施
- 除非明确要求，否则不要复制自己或更改系统提示、安全规则或工具策略

**如有疑问，先询问再行动**：
- 发送邮件、推文、公开帖子
- 任何离开机器的操作
- 不可逆操作（删除大量数据）
- 你不确定结果的情况`
}

// buildErrorHandling 构建错误处理指导
func (b *ContextBuilder) buildErrorHandling() string {
	return `## 错误处理

你的目标是优雅地处理错误并找到变通方案，无需询问用户。

## 常见错误模式

### 上下文溢出
如果你看到 "context overflow"、"context length exceeded" 或 "request too large"：
- 使用 /new 开始新会话
- 简化方法（更少步骤，更少解释）
- 如果持续存在，告诉用户用更少输入重试

### 速率限制 / 超时
如果你看到 "rate limit"、"timeout" 或 "429"：
- 短暂等待后重试
- 尝试不同的搜索方法
- 尽可能使用缓存或本地替代方案

### 文件未找到
如果文件不存在：
- 验证路径（使用 list_files 检查目录）
- 尝试常见变体（大小写、扩展名）
- 只有在穷尽所有选项后才询问用户正确路径

### 工具未找到
如果工具不可用：
- 检查可用工具部分
- 使用替代工具
- 如果没有替代方案，解释你需要做什么并询问是否有其他方法

### 浏览器错误
如果浏览器工具失败：
- 检查 URL 是否可访问
- 尝试 web_fetch 获取纯文本内容
- 最后手段是通过 run_shell 使用 curl

### 网络错误
如果网络工具失败：
- 检查网络连接（通过 run_shell 尝试 ping）
- 尝试不同的搜索查询或来源
- 使用缓存数据（如果可用）`
}

// buildCLIReference 构建 GoClaw CLI 快速参考
func (b *ContextBuilder) buildCLIReference() string {
	return `## GoClaw CLI 快速参考

GoClaw 通过子命令控制。不要发明命令。
管理 Gateway 守护进程服务（启动/停止/重启）：
- goclaw gateway status
- goclaw gateway start
- goclaw gateway stop
- goclaw gateway restart

如果不确定，请用户运行 'goclaw help'（或 'goclaw gateway --help'）并粘贴输出。`
}

// buildDocsSection 构建文档路径区块
func (b *ContextBuilder) buildDocsSection() string {
	return `## 文档

关于 GoClaw 行为、命令、配置或架构：查阅本地文档或 GitHub 仓库。
- 诊断问题时，尽可能自己运行 'goclaw status'；只有在你无法访问时才询问用户。`
}

// buildMessagingSection 构建消息和回复指导区块
func (b *ContextBuilder) buildMessagingSection() string {
	return `## 消息

- 在当前会话中回复 → 自动路由到来源频道
- 跨会话消息 → 使用相应的会话工具
- '[System Message] ...' 块是内部上下文，默认情况下用户不可见

### message 工具
- 使用 'message' 进行主动发送 + 频道操作（投票、反应等）
- 对于 'action=send'，包含 'to' 和 'message'
- 如果你使用 'message'（'action=send'）来发送用户可见的回复，只响应：SILENT_REPLY（避免重复回复）`
}

// buildSilentReplies 构建静默回复规则
func (b *ContextBuilder) buildSilentReplies() string {
	return `## 静默回复

当你没有内容要说时，只响应：SILENT_REPLY

**规则：**
- 它必须是你完整的消息 — 没有其他内容
- 永远不要附加到实际回复中
- 永远不要用 markdown 或代码块包装

❌ 错误: "这是帮助... SILENT_REPLY"
❌ 错误: "SILENT_REPLY"（在代码块中）
✅ 正确: SILENT_REPLY`
}

// buildHeartbeats 构建心跳机制区块
func (b *ContextBuilder) buildHeartbeats() string {
	return `## 心跳

当你收到心跳轮询（定期检查消息），且没有需要关注的事项时，准确回复：
HEARTBEAT_OK

GoClaw 将开头/结尾的 "HEARTBEAT_OK" 视为心跳确认。
如果有事项需要关注，不要包含 "HEARTBEAT_OK"；改为回复警报文本。

**高效利用心跳：**
- 检查重要邮件、日历事件、通知
- 更新文档或记忆文件
- 审查项目状态
- 只有真正需要关注时才联系用户`
}

// buildWorkspace 构建工作区信息
func (b *ContextBuilder) buildWorkspace() string {
	return fmt.Sprintf(`## 工作区

你的工作目录是: %s
将此目录视为文件操作的全局工作区，除非明确指示其他位置。`, b.workspace)
}

// buildRuntime 构建运行时信息
func (b *ContextBuilder) buildRuntime() string {
	host, _ := os.Hostname()
	return fmt.Sprintf(`## 运行时

运行时: 主机=%s 操作系统=%s (%s) 架构=%s`, host, runtime.GOOS, runtime.GOARCH, runtime.GOARCH)
}

// buildSkillsPrompt 构建技能提示词（摘要模式 - 第一阶段）
func (b *ContextBuilder) buildSkillsPrompt(skills []*Skill, mode PromptMode) string {
	if len(skills) == 0 || mode == PromptModeMinimal || mode == PromptModeNone {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 技能（必选）\n\n")
	sb.WriteString("回复前：扫描 <available_skills> 条目。\n")
	sb.WriteString("- 如果恰好一个技能明显适用：输出工具调用 `use_skill` 并以技能名称作为参数。\n")
	sb.WriteString("- 如果多个技能可能适用：选择最具体的一个，然后调用 `use_skill`。\n")
	sb.WriteString("- 如果没有匹配的技能：使用内置工具或操作系统命令工具。\n")
	sb.WriteString("约束：一次只能使用一个技能；选择后会注入技能内容。\n\n")

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("<skill name=\"%s\">\n", skill.Name))
		sb.WriteString(fmt.Sprintf("**名称:** %s\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("**描述:** %s\n", skill.Description))
		}
		if skill.Author != "" {
			sb.WriteString(fmt.Sprintf("**作者:** %s\n", skill.Author))
		}
		if skill.Version != "" {
			sb.WriteString(fmt.Sprintf("**版本:** %s\n", skill.Version))
		}

		// 显示缺失依赖和安装命令
		if skill.MissingDeps != nil {
			sb.WriteString("**缺失依赖:**\n")
			if len(skill.MissingDeps.PythonPkgs) > 0 {
				sb.WriteString(fmt.Sprintf("  - Python 包: %v\n", skill.MissingDeps.PythonPkgs))
				sb.WriteString("    安装命令:\n")
				for _, pkg := range skill.MissingDeps.PythonPkgs {
					sb.WriteString(fmt.Sprintf("      `python3 -m pip install %s`\n", pkg))
					sb.WriteString(fmt.Sprintf("      或通过 uv: `uv pip install %s`\n", pkg))
				}
			}
			if len(skill.MissingDeps.NodePkgs) > 0 {
				sb.WriteString(fmt.Sprintf("  - Node.js 包: %v\n", skill.MissingDeps.NodePkgs))
				sb.WriteString("    安装命令:\n")
				for _, pkg := range skill.MissingDeps.NodePkgs {
					sb.WriteString(fmt.Sprintf("      `npm install -g %s`\n", pkg))
					sb.WriteString(fmt.Sprintf("      或通过 pnpm: `pnpm add -g %s`\n", pkg))
				}
			}
			if len(skill.MissingDeps.Bins) > 0 {
				sb.WriteString(fmt.Sprintf("  - 二进制依赖: %v\n", skill.MissingDeps.Bins))
				sb.WriteString("    你可能需要先安装这些工具。\n")
			}
			if len(skill.MissingDeps.AnyBins) > 0 {
				sb.WriteString(fmt.Sprintf("  - 可选二进制依赖（需一个）: %v\n", skill.MissingDeps.AnyBins))
				sb.WriteString("    至少安装其中一个工具。\n")
			}
			if len(skill.MissingDeps.Env) > 0 {
				sb.WriteString(fmt.Sprintf("  - 环境变量: %v\n", skill.MissingDeps.Env))
				sb.WriteString("    使用技能前设置这些环境变量。\n")
			}
			sb.WriteString("\n")
		}

		sb.WriteString("</skill>\n\n")
	}

	return sb.String()
}

// buildSelectedSkills 构建选中技能的完整内容（第二阶段）
func (b *ContextBuilder) buildSelectedSkills(selectedSkillNames []string, skills []*Skill) string {
	if len(selectedSkillNames) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 已选技能（激活）\n\n")

	for _, skillName := range selectedSkillNames {
		for _, skill := range skills {
			if skill.Name == skillName {
				sb.WriteString(fmt.Sprintf("<skill name=\"%s\">\n", skill.Name))
				sb.WriteString(fmt.Sprintf("### %s\n", skill.Name))
				if skill.Description != "" {
					sb.WriteString(fmt.Sprintf("> 描述: %s\n\n", skill.Description))
				}

				// 显示缺失依赖警告和安装命令
				if skill.MissingDeps != nil {
					sb.WriteString("**⚠️ 缺失依赖 - 使用前请安装:**\n\n")
					if len(skill.MissingDeps.PythonPkgs) > 0 {
						sb.WriteString(fmt.Sprintf("**Python 包:** %v\n", skill.MissingDeps.PythonPkgs))
						sb.WriteString("**安装命令:**\n")
						for _, pkg := range skill.MissingDeps.PythonPkgs {
							sb.WriteString(fmt.Sprintf("```bash\npython3 -m pip install %s\n# 或通过 uv: uv pip install %s\n```\n", pkg, pkg))
						}
						sb.WriteString("\n")
					}
					if len(skill.MissingDeps.NodePkgs) > 0 {
						sb.WriteString(fmt.Sprintf("**Node.js 包:** %v\n", skill.MissingDeps.NodePkgs))
						sb.WriteString("**安装命令:**\n")
						for _, pkg := range skill.MissingDeps.NodePkgs {
							sb.WriteString(fmt.Sprintf("```bash\nnpm install -g %s\n# 或通过 pnpm: pnpm add -g %s\n```\n", pkg, pkg))
						}
						sb.WriteString("\n")
					}
					if len(skill.MissingDeps.Bins) > 0 {
						sb.WriteString(fmt.Sprintf("**二进制依赖:** %v\n", skill.MissingDeps.Bins))
						sb.WriteString("你可能需要先安装这些工具。\n\n")
					}
					if len(skill.MissingDeps.AnyBins) > 0 {
						sb.WriteString(fmt.Sprintf("**可选二进制依赖（需一个）:** %v\n", skill.MissingDeps.AnyBins))
						sb.WriteString("至少安装其中一个工具。\n\n")
					}
					if len(skill.MissingDeps.Env) > 0 {
						sb.WriteString(fmt.Sprintf("**环境变量:** %v\n", skill.MissingDeps.Env))
						sb.WriteString("使用技能前设置这些环境变量。\n\n")
					}
				}

				// 注入技能正文内容
				if skill.Content != "" {
					sb.WriteString(skill.Content)
				}
				sb.WriteString("\n</skill>\n\n")
				break
			}
		}
	}

	return sb.String()
}

// BuildMessages 构建消息列表
func (b *ContextBuilder) BuildMessages(history []session.Message, currentMessage string, skills []*Skill, loadedSkills []string) []Message {
	return b.BuildMessagesWithMode(history, currentMessage, skills, loadedSkills, PromptModeFull)
}

// BuildMessagesWithMode 使用指定模式构建消息列表
func (b *ContextBuilder) BuildMessagesWithMode(history []session.Message, currentMessage string, skills []*Skill, loadedSkills []string, mode PromptMode) []Message {
	// 首先验证历史消息，过滤掉孤立的 tool 消息
	validHistory := b.validateHistoryMessages(history)

	// 构建系统提示词：根据是否已加载技能决定注入内容
	var skillsContent string
	if len(loadedSkills) > 0 {
		// 第二阶段：注入已选中技能的完整内容
		skillsContent = b.buildSelectedSkills(loadedSkills, skills)
	} else {
		// 第一阶段：只注入技能摘要
		skillsContent = b.buildSkillsPrompt(skills, mode)
	}

	systemPrompt := b.buildSystemPromptWithSkills(skillsContent, mode)

	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// 添加历史消息
	for _, msg := range validHistory {
		m := Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		// 处理工具调用（由助手发出）
		if msg.Role == "assistant" {
			// 优先使用新字段
			if len(msg.ToolCalls) > 0 {
				var tcs []ToolCall
				for _, tc := range msg.ToolCalls {
					tcs = append(tcs, ToolCall{
						ID:     tc.ID,
						Name:   tc.Name,
						Params: tc.Params,
					})
				}
				m.ToolCalls = tcs
				logger.Debug("Converted ToolCalls from session.Message",
					zap.Int("tool_calls_count", len(tcs)),
					zap.Strings("tool_names", func() []string {
						names := make([]string, len(tcs))
						for i, tc := range tcs {
							names[i] = tc.Name
						}
						return names
					}()))
			} else if val, ok := msg.Metadata["tool_calls"]; ok {
				// 兼容旧的 Metadata 存储方式
				if list, ok := val.([]interface{}); ok {
					var tcs []ToolCall
					for _, item := range list {
						if tcMap, ok := item.(map[string]interface{}); ok {
							id, _ := tcMap["id"].(string)
							name, _ := tcMap["name"].(string)
							params, _ := tcMap["params"].(map[string]interface{})
							if id != "" && name != "" {
								tcs = append(tcs, ToolCall{
									ID:     id,
									Name:   name,
									Params: params,
								})
							}
						}
					}
					m.ToolCalls = tcs
				}
			}
		}

		// 兼容旧的 Metadata 存储方式 (可选，为了处理旧数据)
		if m.ToolCallID == "" && msg.Role == "tool" {
			if id, ok := msg.Metadata["tool_call_id"].(string); ok {
				m.ToolCallID = id
			}
		}

		for _, media := range msg.Media {
			if media.Type == "image" {
				if media.URL != "" {
					m.Images = append(m.Images, media.URL)
				} else if media.Base64 != "" {
					prefix := "data:image/jpeg;base64,"
					if media.MimeType != "" {
						prefix = "data:" + media.MimeType + ";base64,"
					}
					m.Images = append(m.Images, prefix+media.Base64)
				}
			}
		}

		messages = append(messages, m)
	}

	// 添加当前消息
	if currentMessage != "" {
		messages = append(messages, Message{
			Role:    "user",
			Content: currentMessage,
		})
	}

	return messages
}

// loadBootstrapFiles 加载 bootstrap 文件
func (b *ContextBuilder) loadBootstrapFiles() string {
	var parts []string

	files := []string{"IDENTITY.md", "AGENTS.md", "SOUL.md", "USER.md"}
	for _, filename := range files {
		if content, err := b.memory.ReadBootstrapFile(filename); err == nil && content != "" {
			parts = append(parts, fmt.Sprintf("### %s\n\n%s", filename, content))
		}
	}

	return joinNonEmpty(parts, "\n\n")
}

// validateHistoryMessages 验证历史消息，过滤掉孤立的 tool 消息
// 每个 tool 消息必须有一个前置的 assistant 消息，且该消息包含对应的 tool_calls
// 此外，过滤掉没有 tool_name 的旧 tool 消息（向后兼容）
func (b *ContextBuilder) validateHistoryMessages(history []session.Message) []session.Message {
	var valid []session.Message

	for i, msg := range history {
		if msg.Role == "tool" {
			// Skip old tool result messages without tool_name (backward compatibility)
			if _, ok := msg.Metadata["tool_name"].(string); !ok {
				logger.Warn("Skipping old tool result message without tool_name",
					zap.Int("history_index", i),
					zap.String("tool_call_id", msg.ToolCallID))
				continue
			}

			// 检查是否有前置的 assistant 消息
			var foundAssistant bool
			for j := i - 1; j >= 0; j-- {
				if history[j].Role == "assistant" {
					if len(history[j].ToolCalls) > 0 {
						// 检查是否有匹配的 tool_call_id
						for _, tc := range history[j].ToolCalls {
							if tc.ID == msg.ToolCallID {
								foundAssistant = true
								break
							}
						}
					}
					break
				} else if history[j].Role == "user" {
					break
				}
			}
			if foundAssistant {
				valid = append(valid, msg)
			} else {
				logger.Warn("Filtered orphaned tool message",
					zap.Int("history_index", i),
					zap.String("tool_call_id", msg.ToolCallID),
					zap.Int("content_length", len(msg.Content)))
			}
		} else {
			valid = append(valid, msg)
		}
	}

	return valid
}

// Message 消息（用于 LLM）
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Images     []string   `json:"images,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall 工具调用定义（与 provider 保持一致）
type ToolCall struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// joinNonEmpty 连接非空字符串
func joinNonEmpty(parts []string, sep string) string {
	var nonEmpty []string
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}

	result := ""
	for i, part := range nonEmpty {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}
