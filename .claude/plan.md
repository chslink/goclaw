# InternalDecisionAgent 改造计划

## 目标

把当前大量依赖 `strings.Contains`、硬编码 prompt 拼接、固定 if/switch 分支的“语言理解型决策”逐步迁移到一个专用的内部决策层：

- **代码负责**：状态、权限、冷却、持久化、消息总线、实际副作用执行
- **LLM 负责**：意图识别、歧义消解、对象匹配、流程选择、路由建议
- **框架定位**：框架本身是组织 LLM 与工具协作的编排层，而不是用大量硬编码规则模拟理解能力

本次实施采用 **渐进式迁移**，优先替换最脆弱、最明显依赖关键字匹配的流程，而不是一次性把全部逻辑改成 LLM。

## 当前代码中的关键问题

### 1. 最脆弱的关键字决策

1. `swarm/approval.go`
   - `isApproveIntent()` / `isRejectIntent()` 使用关键字数组判断审批意图
   - `matchByContext()` 用 `Contains(title)`/`Contains(id)` 匹配审批对象
   - 风险：自然语言变体多、误判率高、多审批场景容易匹配错

2. `agent/manager.go`
   - `isCronOneShotRequest()` 用关键字判断“是否手工执行一次定时任务”
   - `resolveCronJobIDForOneShot()` 只支持显式 job-id 或“只有一个任务”的情况
   - 风险：表达方式变化就识别不到，且多任务场景太僵硬

3. `agent/context.go` + `agent/orchestrator.go`
   - 通过 prompt 文本里是否包含“禁止使用工具”“没有工具可用”来禁用工具
   - 风险：依赖特定措辞，且把配置判断和语言判断混在一起

### 2. Prompt 组装过于代码硬编码

`agent/context.go` 里 `buildSystemPromptWithSkills()` 将 13 个 section 固定拼接：

- 身份
- 工具说明
- Tool Call Style
- 安全说明
- 错误处理
- CLI 参考
- 文档路径
- Bootstrap 文件
- Messaging
- Silent Replies
- Heartbeats
- Workspace
- Runtime

问题不是“不能拼接”，而是：
- 现在很多“是否需要某一段”的判断也是写死在代码里
- 将来如果要做内部专用 agent（比如 decision agent / summarizer agent / router agent），复用成本高

### 3. Provider 侧存在一个关键约束

当前 `providers.Provider` 虽然暴露了 `ChatOption` / `WithModel()`，但现有 `Orchestrator` 调用 `Provider.Chat()` 时 **没有按调用传模型**；而具体 provider（如 `openai.go` / `openrouter.go` / `anthropic.go`）也是在创建实例时就绑定了默认模型。

这意味着：
- **InternalDecisionAgent 不能依赖“同一个 provider 实例按调用切成 flash 模型”**
- 正确做法是：**为 internal decision 单独创建 provider 实例**，并从指定 profile / model 初始化

这个点是架构设计里的硬前提。

## 总体设计

## 一、引入新的顶层包：`decision/`

新增一个框架级包，专门承载“内部 LLM 决策工作流”：

- `decision/service.go`：对外统一入口
- `decision/runner.go`：轻量 tool-calling 循环（内部专用，不复用完整用户 Agent）
- `decision/tool.go`：内部 tool 抽象
- `decision/approval.go`：审批解析工作流
- `decision/cron_oneshot.go`：一次性 cron 请求工作流
- 后续可扩展：`decision/routing.go`、`decision/prompt_policy.go`

### 为什么不直接复用现有 `agent.Agent`

不建议 Phase 1 直接把内部决策做成完整 `agent.Agent`，原因：

1. 现有 `agent.Agent` 默认绑定完整 `ContextBuilder`
   - 会注入大量用户向的 system prompt、技能、workspace、silent reply 等内容
   - 对 decision 任务来说过重且干扰大

2. 现有 `Orchestrator` 对 system prompt 构建方式有较强耦合
   - 默认走 `PromptModeFull`
   - 有面向用户 agent 的工具禁用判断

3. internal decision 需要的是：
   - 极小 prompt
   - 极少工具
   - 极短超时
   - 最多 2~3 轮工具调用
   - 不需要会话记忆、技能、bootstrap 文件

因此 Phase 1 更合适的方式是：
- **单独做一个轻量 `decision.Runner`**
- 复用 `providers.Provider` 的 tool calling 能力
- 不引入完整 `Agent` 生命周期

后续如果 internal workflow 越来越多，再考虑把 `Orchestrator` 抽一层公共 tool-loop 内核。

## 二、配置设计：新增独立 internal agent 配置

在 `config/schema.go` 顶层 `Config` 中新增：

```go
type Config struct {
    ...
    InternalAgents InternalAgentsConfig `mapstructure:"internal_agents" json:"internal_agents"`
}

type InternalAgentsConfig struct {
    Decision DecisionAgentConfig `mapstructure:"decision" json:"decision"`
}

type DecisionAgentConfig struct {
    Enabled         bool    `mapstructure:"enabled" json:"enabled"`
    ProviderProfile string  `mapstructure:"provider_profile" json:"provider_profile"`
    Model           string  `mapstructure:"model" json:"model"`
    Temperature     float64 `mapstructure:"temperature" json:"temperature"`
    MaxTokens       int     `mapstructure:"max_tokens" json:"max_tokens"`
    TimeoutSeconds  int     `mapstructure:"timeout_seconds" json:"timeout_seconds"`
    MaxIterations   int     `mapstructure:"max_iterations" json:"max_iterations"`
    Fallback        string  `mapstructure:"fallback" json:"fallback"` // keyword | error
    Debug           bool    `mapstructure:"debug" json:"debug"`
}
```

建议默认值：
- `enabled=true`
- `temperature=0`
- `max_tokens=256`
- `timeout_seconds=3`
- `max_iterations=3`
- `fallback=keyword`

### 校验要求

在 `config/validator.go` 新增：
- `provider_profile` 必须存在于 `cfg.Providers.Profiles`
- `model` 不能为空
- `timeout_seconds` 范围 1~30
- `max_iterations` 范围 1~5
- `fallback` 只能是 `keyword` 或 `error`

## 三、Provider 扩展：支持从 Profile 创建独立 Provider

在 `providers/factory.go` 新增一个显式工厂：

```go
func NewProviderFromProfile(cfg *config.Config, profileName, model string, maxTokens int) (Provider, error)
```

实现方式：
- 根据 `cfg.Providers.Profiles` 找到对应 profile
- 复用现有 `createProviderByType(...)`
- 用 internal decision 指定的 `model` / `maxTokens` 创建独立 provider 实例

这样可确保：
- 主 Agent 仍使用默认 provider
- internal decision 使用单独 flash / mini / cheap profile
- 不依赖当前 `Orchestrator` 的 per-call model override 缺失能力

## 四、Decision Runner 设计

`decision.Runner` 是一个轻量工具循环：

```go
type Runner struct {
    provider      providers.Provider
    maxIterations int
    timeout       time.Duration
    debug         bool
}
```

执行模型：
1. 构造 `[system, user]` 消息
2. 调用 `provider.Chat(ctx, messages, toolDefs)`
3. 若 LLM 触发工具：
   - 执行内部工具
   - 把工具结果追加为 `tool` 消息
   - 继续下一轮
4. 若没有工具调用：返回错误（因为内部工作流必须通过终止工具返回结构化结果）
5. 超过最大迭代数：失败

### 关键约束

internal decision tool **不直接做外部副作用**。

只允许两类 tool：
1. **查询型工具**：读取当前上下文/候选对象
2. **终止型工具**：`return_*`，把结构化决策返回给代码

这样可以保证：
- LLM 负责理解和匹配
- 真正的状态修改/消息发送/cron 执行仍由 Go 代码完成
- 降低“让内部 agent 直接动系统”的风险

## 五、内部 tool 设计原则

### 不做“万能元工具”，先做 workflow-scoped tools

Phase 1 不做一个包打天下的 `intent_classify` 超级工具，而是：
- 一个工作流一组小工具
- 一个工作流一个 system prompt
- 一个工作流一个结构化返回类型

这样更稳、更可测。

### Approval 工作流工具

#### `list_pending_approvals`
无参数，返回待审批列表：
- `id`
- `title`
- `description`
- `requested_by`
- `created_at`

#### `return_approval_decision`
参数：
```json
{
  "matched_id": "abc123",
  "action": "approve|reject|none",
  "confidence": 0.0,
  "reason": "简短解释"
}
```

### Cron One-shot 工作流工具

#### `list_enabled_cron_jobs`
无参数，返回可执行 cron job 列表：
- `job_id`
- `summary`
- `schedule`
- `enabled`

#### `return_cron_decision`
参数：
```json
{
  "is_one_shot": true,
  "job_id": "job-abc",
  "confidence": 0.0,
  "reason": "简短解释"
}
```

## 六、代码与 LLM 的职责边界

### 适合迁移给 InternalDecisionAgent 的

1. 用户语言理解
   - “这个是在同意审批还是只是客气回复？”
   - “这句话是不是在请求手工跑一次 cron？”
   - “多个审批里用户在说哪一个？”
   - “多个 cron job 里更像哪一个？”

2. 轻量路由建议
   - 某条消息更适合哪个角色/agent 先处理

3. Prompt 模块选择策略（未来）
   - 某类 internal agent 需要哪些 prompt block

### 必须保留在代码里的

1. 权限检查
2. cooldown / rate limit
3. 状态机转换
4. 实际副作用执行（发消息 / 修改审批状态 / 执行 cron）
5. provider 实例创建
6. bus / session / persistence

结论：**不是“所有判断都 LLM 化”，而是“所有语言理解型判断 LLM 化，规则和状态型判断保留代码实现”。**

## 分阶段实施

## Phase 1：基础设施落地（不改业务行为）

### 目标
搭好 InternalDecisionAgent 的基础设施，但先不接入生产路径。

### 文件变更

#### 新增
- `decision/service.go`
- `decision/runner.go`
- `decision/tool.go`
- `decision/runner_test.go`

#### 修改
- `config/schema.go`
- `config/validator.go`
- `providers/factory.go`
- `cli/root.go`
- `cli/swarm_services.go`
- `agent/manager.go`
- `swarm/corporate.go`（只做注入预留，如果需要）

### 设计

#### `decision.Service`

```go
type Service struct {
    cfg      config.DecisionAgentConfig
    provider providers.Provider
}
```

职责：
- 持有 internal decision provider
- 对外暴露业务工作流接口
- 统一 fallback 策略
- 统一 debug 日志

#### 启动注入点

普通模式：
- `runStart()` 初始化 `decision.Service`
- 注入 `AgentManager`

swarm 模式：
- `initSwarmBaseServices()` 初始化 `decision.Service`
- 注入 `CorporateSwarmManager` / `ApprovalManager`

### 验证
- 新配置缺省时不影响现有启动
- 配置错误时 validator 给出明确报错
- `go build ./...`

## Phase 2：审批流迁移（第一批真实接入）

### 目标
替换 `swarm/approval.go` 中所有关键字识别逻辑。

### 文件变更

#### 新增
- `decision/approval.go`
- `decision/approval_test.go`

#### 修改
- `swarm/approval.go`
- `swarm/corporate.go`
- 对应测试文件（如果没有，则新增）

### 具体改法

#### 1. 修改 `ApprovalManager.TryResolve`

从：
```go
func (m *ApprovalManager) TryResolve(userMessage string) (bool, bool, string)
```

改为：
```go
func (m *ApprovalManager) TryResolve(ctx context.Context, userMessage string) (bool, bool, string)
```

原因：
- LLM 调用必须带 context / timeout

#### 2. 给 `ApprovalManager` 增加可选 resolver

```go
type ApprovalResolver interface {
    ResolveApproval(ctx context.Context, userMessage string, pending []*ApprovalRequest) (*ApprovalResolution, error)
}
```

`ApprovalManager` 内部：
- 若配置了 resolver，先走 resolver
- 若 resolver 失败且 `fallback=keyword`，回退到现有关键字逻辑
- 若未配置 resolver，保持现有行为

#### 3. Approval 决策流程

`decision.Service.ResolveApproval(...)`：
- 传入用户消息
- 通过 `list_pending_approvals` 给模型看候选项
- 模型调用 `return_approval_decision`
- Go 代码根据结构化结果执行状态变更

### 验证
- 单个审批： “同意”“可以”“驳回吧”“不通过”
- 多个审批： “批准方案A”“驳回 abc123”
- 模糊输入： “好的我知道了” 不应误判为 approve
- fallback 覆盖：模拟 provider 异常时仍可退回关键字逻辑

## Phase 3：Cron one-shot 迁移

### 目标
替换 `agent/manager.go` 中 `isCronOneShotRequest()` 和任务匹配逻辑。

### 文件变更

#### 新增
- `decision/cron_oneshot.go`
- `decision/cron_oneshot_test.go`

#### 修改
- `agent/manager.go`
- `agent/cron_oneshot_test.go`

### 具体改法

把这段逻辑：
- `isCronOneShotRequest()`
- `resolveCronJobIDForOneShot()`

替换为：
```go
resolution, err := m.decision.ResolveCronOneShot(ctx, content)
```

其中：
- 是否是 one-shot 请求 → 由 LLM 判断
- 匹配哪个 job → 由 LLM 在 `list_enabled_cron_jobs` 后决定
- cooldown 仍然走 `allowManualCronRun()`
- 真正 `cron run <job-id>` 仍然由 Go 代码执行

### 验证
- “帮我手工跑一次日报任务”
- “cron run job-abc”
- “测试一下那个定时同步”
- 多个启用任务时能够匹配正确候选；不确定时返回澄清或失败

## Phase 4：消息路由/角色路由迁移（第二阶段）

### 目标
把一部分路由判断从硬编码表查找 + 特判，升级为“静态绑定优先 + LLM 建议补充”。

### 方式

#### 不替换已有硬绑定
以下逻辑仍保留：
- `bindings[channel:accountID]`
- default agent
- ACP thread binding

#### 新增“建议式路由”
只在以下场景调用决策层：
- 没有显式 binding 时
- corporate 场景需要判断是否给 secretary 还是进入特定流程时
- 将来 swarm worker pool 选择时

### 文件候选
- `decision/routing.go`
- `agent/manager.go`
- `swarm/corporate.go`

### 原则
- LLM 路由只是推荐
- 代码仍做最终约束校验

## Phase 5：Prompt 模块化（不是直接全量 LLM 化）

### 目标
把 `agent/context.go` 从“巨型函数拼字符串”改为“模块 + 策略”。

### 关键说明
这一步不是让主 agent 每次都用 LLM 决定 prompt 拼接，而是先把结构整理好，为后续 internal agent / specialized agent 做准备。

### 重构方向

#### 1. 提取 Prompt 模块
新增：
- `agent/prompt_modules.go`
- `agent/prompt_policy.go`

把当前 13 个 section 抽象成：
```go
type PromptModuleID string
const (
    PromptModuleIdentity PromptModuleID = "identity"
    PromptModuleTooling PromptModuleID = "tooling"
    PromptModuleSafety PromptModuleID = "safety"
    ...
)
```

#### 2. 默认策略仍是代码策略
```go
type PromptPolicy interface {
    SelectModules(ctx PromptBuildContext) []PromptModuleID
}
```

- 主 agent：默认 deterministic policy
- internal decision agent：使用更小的 policy
- 后续如有必要，再引入 LLM-based policy

### 这一步的收益
- 降低 `context.go` 复杂度
- internal agent 不再被迫复用 full prompt
- 为“逐步 LLM 化策略选择”提供边界清晰的接入点

## 降级与兼容策略

### 总策略
每个工作流都遵守：

1. **优先 LLM 决策**
2. **失败则 fallback**（如果配置允许）
3. **fallback 仍失败，则走原有错误路径**

### 具体规则

#### 审批
- `fallback=keyword` 时：继续走 `isApproveIntent` / `isRejectIntent` / `matchByContext`
- 为避免一次性切断旧行为，Phase 2 保留原函数作为 fallback，不立即删除

#### Cron
- `fallback=keyword` 时：继续走 `isCronOneShotRequest` + `resolveCronJobIDForOneShot`
- 稳定后再移除旧逻辑

### 观测性

每次决策都记录：
- workflow 名称
- `source=llm|fallback`
- confidence
- latency
- matched object id
- 是否触发 fallback

在 `Debug=true` 时可额外记录：
- tool 调用序列
- 最终结构化结果

## 风险与应对

### 风险 1：把确定性规则也 LLM 化，系统会变得不可控

应对：
- 明确边界：只迁移语言理解，不迁移状态规则和权限

### 风险 2：internal decision 自己太重，反而增加时延

应对：
- 使用独立 flash profile
- timeout 3 秒
- max iterations 3
- 不加载技能/记忆/完整 context

### 风险 3：LLM 工具调用跑偏

应对：
- 工具仅允许查询 + `return_*`
- 实际副作用由 Go 代码执行

### 风险 4：Provider 复用方式不对导致实际仍走主模型

应对：
- 先实现 `providers.NewProviderFromProfile`
- internal decision 一律持有独立 provider 实例

## 实施顺序（建议严格按顺序）

1. `Phase 1` 基础设施：config + provider factory + decision runner + service 注入
2. `Phase 2` 审批流接入
3. `Phase 3` cron one-shot 接入
4. `Phase 4` 路由建议接入
5. `Phase 5` prompt 模块化

## 本次准备实施的最小切片

如果批准开始编码，我建议本轮只做 **Phase 1 + Phase 2**：

### 包含
- internal decision 配置
- 独立 provider profile 创建
- `decision.Service` / `decision.Runner`
- approval workflow tools
- `ApprovalManager.TryResolve(ctx, ...)` LLM 化 + fallback
- 对应单元测试

### 暂不包含
- cron one-shot 改造
- route_message
- context prompt 模块化

### 理由
- approval 是最典型、最脆弱、收益最高的关键词流
- 这一步能把骨架跑通：配置 → internal provider → tool loop → fallback
- 一旦 approval 方案稳定，cron 和 routing 可直接按同样模式复制

## 预计修改文件清单（按本轮最小切片）

### 新增
- `decision/service.go`
- `decision/runner.go`
- `decision/tool.go`
- `decision/approval.go`
- `decision/runner_test.go`
- `decision/approval_test.go`

### 修改
- `config/schema.go`
- `config/validator.go`
- `providers/factory.go`
- `cli/root.go`
- `cli/swarm_services.go`
- `swarm/approval.go`
- `swarm/corporate.go`
- 可能补充：`swarm/*_test.go`

## 验证方案

1. `go build -o goclaw .`
2. `go test ./decision/... -v`
3. `go test ./swarm/... -v`
4. 增加审批场景测试：
   - 单审批 approve/reject
   - 多审批按 title / id 匹配
   - 模糊语句不误判
   - provider 故障时 fallback 生效

## 结论

本次改造不应该理解成“把所有代码逻辑都交给 LLM”，而应该理解成：

- 把 **语言理解型硬编码流程** 逐步迁移成 **internal decision workflow**
- 每个 workflow 由：**专用 prompt + 少量内部 tool + 结构化返回** 组成
- 框架继续负责状态与执行，LLM 负责判断与选择

这是最符合你目标的做法，也能和现有 GoClaw 架构自然衔接。