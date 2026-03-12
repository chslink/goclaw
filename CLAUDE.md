# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

GoClaw (狗爪) 是 Go 语言的 AI Agent 框架，模块名 `github.com/smallnest/goclaw`，Go 1.25.5。核心能力：多通道 IM 接入、多 LLM 提供商、工具系统、技能系统、蜂群协作。

## 常用命令

```bash
# 构建
make build                    # 编译 → ./goclaw
go build -o goclaw .          # 快速编译

# 测试
make test                     # 全量测试 (go test -v ./...)
make test-short               # 快速测试 (-short)
make test-race                # 竞态检测 (-race)
go test -v -run TestFoo ./agent/...  # 运行单个测试

# 代码质量
make lint                     # golangci-lint run ./...
make lint-fix                 # 自动修复
make fmt                      # gofmt -s -w .
make check                    # fmt-check + vet + lint

# 依赖
make deps                     # go mod download
make tidy                     # go mod tidy && verify
```

## 架构概览

### 消息流（核心数据通路）

```
IM 通道 → ChannelManager → MessageBus(InboundQueue) → AgentManager → Agent
    ↑                                                                  ↓
    └── ChannelManager ← MessageBus(OutboundQueue) ← Orchestrator(LLM Loop)
```

`AgentManager` 根据 Binding 规则将 InboundMessage 路由到对应 Agent。Agent 内部由 `Orchestrator` 驱动 LLM 循环：发送消息→获取响应→解析工具调用→执行工具→继续循环，直到无工具调用或达到 MaxIterations。

### 核心包职责

| 包 | 职责 |
|---|------|
| `agent/` | Agent 主体、Orchestrator 执行循环、ToolRegistry、ContextBuilder、SkillsLoader、SubagentRegistry |
| `agent/tools/` | 工具实现（filesystem, shell, web, browser, cron, memory, message, skill, subagent_spawn） |
| `bus/` | 异步消息总线，InboundQueue + OutboundQueue |
| `channels/` | 13+ IM 通道适配器，统一 Channel 接口 |
| `providers/` | LLM 提供商（OpenAI/Anthropic/OpenRouter），含熔断器和故障转移 |
| `config/` | 配置结构（schema.go）、加载（loader.go）、校验（validator.go） |
| `session/` | 会话持久化（JSONL），会话树，历史修剪 |
| `gateway/` | WebSocket 网关，OpenClaw 协议兼容 |
| `swarm/` | 蜂群模式：多 Agent 异步消息流转 |
| `memory/` | 向量存储 + QMD 记忆后端 |
| `acp/` | Agent Client Protocol 实现 |
| `cron/` | 定时任务调度器 |
| `cli/` | Cobra 命令行，所有 `goclaw` 子命令在此定义 |

### Agent 内部结构

```
Agent
├── Orchestrator          # 驱动 LLM 循环（Run → runLoop → callTool）
│   ├── LoopConfig        # 模型、Provider、最大迭代、技能列表
│   └── AgentState        # 消息历史、Steering/FollowUp 队列
├── ToolRegistry          # 包装 tools.Registry，适配 agent.Tool ↔ tools.Tool
├── ContextBuilder        # 构建 SystemPrompt（IDENTITY.md + Memory + Skills）
├── SkillsLoader          # 发现和加载 SKILL.md
├── MessageBus            # 异步收发消息
└── SessionManager        # 会话持久化
```

### Agent 管理层级

```
AgentManager（全局单例）
├── agents map[string]*Agent      # 按 ID 管理多个 Agent
├── bindings                       # Channel+AccountID → Agent 路由
├── SubagentRegistry               # 分身（子 Agent）注册表
└── SubagentAnnouncer              # 分身状态广播
```

### 蜂群模式 (swarm/)

SwarmManager 管理多个 Agent 实例的异步协作。配置文件在 `~/.goclaw/swarms/<name>.json`，定义 `agent_ids` 和 `flows`（条件路由规则）。消息通过 MessageBus 在 Agent 间流转，响应自动触发下一步流程。

## 关键设计模式

- **适配器模式**: `agent.ToolRegistry` 包装 `tools.Registry`，避免 agent↔tools 循环导入
- **消息总线**: 所有 Agent 通信走 `bus.MessageBus`，解耦通道和 Agent
- **多账号**: 每个通道支持 `Accounts map[string]ChannelAccountConfig` 多账号配置
- **Provider Profile**: 支持多个提供商配置 + failover 策略（round_robin/least_used/random）
- **Steering/FollowUp**: Agent 支持中途插入消息（Steer）和执行后追加消息（FollowUp）

## 配置体系

主配置文件加载顺序：`~/.goclaw/config.json` > `./config.json` > 环境变量。
Agent 独立配置：`~/.goclaw/agents/<id>.json`（含 identity、workspace、model 等）。
技能加载顺序：`~/.goclaw/skills/` → `${WORKSPACE}/skills/` → `./skills/`（后覆盖前）。

## 添加新功能的惯例

- **新工具**: 在 `agent/tools/` 实现 `Tool` 接口，在 `registry.go` 注册
- **新通道**: 在 `channels/` 实现 `Channel` 接口，在 `manager.go` 注册，在 `config/schema.go` 添加配置结构
- **新 CLI 命令**: 在 `cli/` 添加 cobra Command，在 `root.go` 注册
- **新提供商**: 在 `providers/` 实现 `Provider` 接口，在 `factory.go` 注册
