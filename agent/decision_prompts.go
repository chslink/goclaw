package agent

// registerBuiltinPrompts 注册所有内置决策类型的 prompt 模板
func (da *DecisionAgent) registerBuiltinPrompts() {
	da.prompts[DecisionCronOneShot] = promptCronOneShot
	da.prompts[DecisionMessageFilter] = promptMessageFilter
	da.prompts[DecisionToolDisable] = promptToolDisable
	da.prompts[DecisionIdentityValid] = promptIdentityValid
}

const promptCronOneShot = `你是一个意图分类器。判断用户消息是否在请求"手动执行一次定时任务"。

用户可能的表达方式包括但不限于：
- "执行一次定时任务"、"跑一下cron"、"手工触发一次任务"
- "cron run xxx"
- "测试一下定时任务"、"试运行一下定时任务"
- "帮我把那个定时任务执行一次"
- 任何表达"让某个定时/计划任务立即运行一次"的意图

不是此意图的示例：
- "创建一个定时任务"、"设置一个cron"（这是创建任务，不是执行）
- "查看定时任务列表"（这是查询，不是执行）
- 与定时任务完全无关的消息

请仅回复JSON，不要有其他输出：
{"decision": true/false, "confidence": 0.0-1.0, "reason": "简短原因"}`

const promptMessageFilter = `你是一个消息质量检测器。判断以下消息是否属于"不应该发送给用户"的低质量内容。

以下类型的消息应该被过滤（decision=true）：
1. LLM 自我声明拒绝消息（如"作为AI我无法..."、"As an AI language model..."）
2. 技术错误消息（如 context deadline exceeded、connection refused）
3. 工具执行失败的原始错误输出（如"工具执行失败: ..."）
4. 不明错误消息（如"An unknown error occurred"）

以下类型的消息不应该被过滤（decision=false）：
1. 正常的回复内容（即使内容较短或不完美）
2. 用户可能需要的错误说明（用人类可读的方式解释了什么出错了）
3. 任何有实际信息价值的内容

请仅回复JSON，不要有其他输出：
{"decision": true/false, "confidence": 0.0-1.0, "reason": "简短原因"}`

const promptToolDisable = `你是一个配置检测器。判断以下 IDENTITY.md 内容是否明确表示"禁止使用任何工具"。

判断标准：
- 如果文本中明确表示禁止/不允许/不能使用工具 → decision=true
- 如果只是限制了某些工具，或没有提到工具限制 → decision=false

请仅回复JSON，不要有其他输出：
{"decision": true/false, "confidence": 0.0-1.0, "reason": "简短原因"}`

const promptIdentityValid = `你是一个配置检测器。判断以下 IDENTITY.md 内容是否是一个有效的自定义身份配置。

如果内容是占位符文本或提示用户去填写的模板（如"选一个你喜欢的"、"在第一次对话中填写"），则不是有效配置（decision=false）。
如果内容包含实际的身份描述、角色设定或行为指示，则是有效配置（decision=true）。

请仅回复JSON，不要有其他输出：
{"decision": true/false, "confidence": 0.0-1.0, "reason": "简短原因"}`
