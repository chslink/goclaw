---
name: goclaw-agent
description: 创建和管理 GoClaw agents。当用户想要创建新 agent、设置 agent 身份、编辑 agent 个性文件、或管理 agent 工作区时使用此技能。
---

# GoClaw Agent 管理技能

此技能帮助你创建和管理 GoClaw agents。

## 关于 GoClaw Agents

GoClaw agents 是独立的智能体实例，每个 agent 拥有：
- 独立的工作区目录
- 独立的配置文件
- 独立的个性化文件（IDENTITY.md、SOUL.md 等）
- 独立的会话历史

## Agent 命令参考

### 创建 Agent

```bash
goclaw agents add <name> [options]
```

选项：
- `--workspace <path>` - 指定工作区目录
- `--model <model>` - 指定使用的模型（如 gpt-4、claude-3）
- `--bind <channel:id>` - 绑定到通道（如 telegram:123456）
- `--non-interactive` - 非交互模式

示例：
```bash
goclaw agents add myagent
goclaw agents add assistant --model claude-3 --bind telegram:123456
```

### 列出 Agents

```bash
goclaw agents list [options]
```

选项：
- `--json` - JSON 格式输出
- `--bindings` - 显示通道绑定详情

### 删除 Agent

```bash
goclaw agents delete <name> [options]
```

选项：
- `--force` - 跳过确认
- `--json` - JSON 格式输出

### 初始化工作区

```bash
goclaw agents bootstrap <name>
```

创建默认的模板文件：
- AGENTS.md - 工作区指南
- SOUL.md - 核心原则和价值观
- IDENTITY.md - 身份定义
- USER.md - 用户档案
- TOOLS.md - 工具使用笔记
- HEARTBEAT.md - 心跳行为配置
- BOOT.md - 启动指令
- BOOTSTRAP.md - 初始化指南

### 编辑个性化文件

```bash
goclaw agents identity <name>  # 编辑 IDENTITY.md
goclaw agents soul <name>      # 编辑 SOUL.md
goclaw agents user <name>      # 编辑 USER.md
goclaw agents tools <name>     # 编辑 TOOLS.md
goclaw agents memory <name>    # 编辑 MEMORY.md
```

## 个性化文件说明

### IDENTITY.md - 身份定义

定义 agent 的身份信息：
- 名称和称呼方式
- 生物类型（AI、机器人、使魔等）
- 气质和性格特点
- 表情符号和头像

示例：
```markdown
- **名称：** 小助手
- **生物类型：** AI 助手
- **气质：** 友好、耐心、专业
- **表情符号：** 🤖
```

### SOUL.md - 核心原则

定义 agent 的核心价值观和行为准则：
- 使命和目标
- 行为原则
- 价值观
- 工作方式

### USER.md - 用户档案

记录 agent 所服务的用户信息：
- 姓名和称呼
- 时区
- 偏好和习惯
- 项目背景

### TOOLS.md - 工具笔记

记录特定工具的使用说明：
- 本地工具配置
- API 密钥信息（不含实际密钥）
- 工作流程笔记

### MEMORY.md - 长期记忆

存储重要的事件、决策和上下文：
- 重要决策记录
- 学习到的经验
- 需要记住的事项

## 工作流程

### 创建新 Agent 的推荐流程

1. **创建 agent**
   ```bash
   goclaw agents add myagent
   ```

2. **初始化工作区（可选）**
   ```bash
   goclaw agents bootstrap myagent
   ```

3. **自定义身份**
   ```bash
   goclaw agents identity myagent
   goclaw agents soul myagent
   goclaw agents user myagent
   ```

4. **绑定通道（可选）**
   编辑配置文件添加绑定：
   ```bash
   # 配置文件位于 ~/.goclaw/agents/myagent.json
   ```

### Agent 配置文件

配置文件位于 `~/.goclaw/agents/<name>.json`：

```json
{
  "name": "myagent",
  "workspace": "~/.goclaw/workspaces/myagent",
  "model": "gpt-4",
  "bindings": ["telegram:123456"],
  "metadata": {}
}
```

## 注意事项

### GoClaw 与 OpenClaw 的区别

虽然技能系统兼容 OpenClaw 格式，但 CLI 命令完全不同：
- 使用 `goclaw` 而非 `openclaw` 命令
- 运行 `goclaw --help` 查看可用命令
- 不要假设 OpenClaw 的命令在 GoClaw 中可用

### 工作区结构

```
~/.goclaw/
├── agents/              # Agent 配置文件
│   ├── myagent.json
│   └── assistant.json
├── workspaces/          # Agent 工作区
│   ├── myagent/
│   │   ├── AGENTS.md
│   │   ├── SOUL.md
│   │   ├── IDENTITY.md
│   │   ├── USER.md
│   │   ├── TOOLS.md
│   │   ├── MEMORY.md
│   │   └── memory/      # 日常记忆
│   └── assistant/
└── sessions/            # 会话历史
```

## 常见问题

### 如何切换 Agent？

使用 `--agent` 参数指定 agent：
```bash
goclaw agent -m "你好" --agent myagent
```

### 如何查看 Agent 状态？

```bash
goclaw agents list
```

### 如何备份 Agent？

复制以下目录：
- `~/.goclaw/agents/<name>.json`
- `~/.goclaw/workspaces/<name>/`

## Swarm 动态 Agent 管理

蜂群运行时可以动态添加或移除 agent，无需重启。CLI 命令通过 Admin HTTP API 与运行中的蜂群进程通信。

### 前提条件

蜂群必须已经在运行（`goclaw swarm start <name>`），且 gateway 已启动。

### 向蜂群添加 Agent

```bash
goclaw swarm add-agent <agent-id> [options]
```

选项：
- `--workspace <path>` - 指定工作区目录（可选，默认 `~/.goclaw/workspaces/<agent-id>`）
- `--model <model>` - 指定使用的模型（可选）
- `--port <port>` - Admin API 端口（默认 28789）

两种添加方式：
1. **从磁盘配置加载**：仅指定 agent-id，配置从 `~/.goclaw/agents/<agent-id>.json` 加载
2. **运行时创建**：指定 `--workspace` 或 `--model`，无需磁盘配置文件

示例：
```bash
# 从磁盘配置加载已有 agent
goclaw swarm add-agent my-worker

# 运行时创建全新 agent
goclaw swarm add-agent temp-agent --workspace /tmp/temp --model gpt-4

# 指定自定义端口
goclaw swarm add-agent my-worker --port 18080
```

**Corporate 模式**：添加的 agent 作为 worker 角色加入，不能与管理层（secretary/hr/pm）ID 冲突。Worker 可以调用所有管理层 agent。

### 从蜂群移除 Agent

```bash
goclaw swarm remove-agent <agent-id> [options]
```

选项：
- `--port <port>` - Admin API 端口（默认 28789）

示例：
```bash
goclaw swarm remove-agent old-worker
goclaw swarm remove-agent old-worker --port 18080
```

**Corporate 模式**：不能移除管理层角色（secretary/hr/pm），只能移除动态 worker。

### Admin HTTP API

如果需要通过编程方式管理，可以直接调用 Admin API：

**添加 Agent**
```
POST http://localhost:28789/admin/api/swarms/agents
Content-Type: application/json

{
  "agent_id": "new-worker",
  "workspace": "/optional/path",
  "model": "gpt-4"
}
```
- `agent_id`（必填）：Agent ID
- `workspace`（可选）：工作区路径，为空则使用默认路径
- `model`（可选）：模型名称

**移除 Agent**
```
DELETE http://localhost:28789/admin/api/swarms/agents/{agent-id}
```

响应均返回操作结果和当前 agent 列表。

### Swarm 动态管理工作流示例

```bash
# 终端 A：启动蜂群
goclaw swarm start myswarm

# 终端 B：查看状态
goclaw swarm status myswarm

# 终端 B：动态添加 agent
goclaw swarm add-agent new-worker

# 终端 B：确认 agent 已加入
goclaw swarm status myswarm

# 终端 B：移除 agent
goclaw swarm remove-agent new-worker
```
