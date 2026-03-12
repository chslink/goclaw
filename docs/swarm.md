# Swarm 蜂群模式

蜂群模式允许您同时运行多个 Agent，并通过异步消息传递实现 Agent 之间的协作。

## 概述

蜂群模式的核心特性：

- **异步消息传递** - 消息发送后立即返回，不阻塞调用方
- **回报处理** - 接收到回报后自动触发流程下一步
- **流程编排** - 支持条件判断和消息路由
- **复用 Agent 配置** - 直接使用 `goclaw agents` 创建的 Agent

## 安装设置

### 1. 编译项目

```bash
go build -o goclaw.exe .
```

### 2. 创建 Agent

使用 `goclaw agents add` 命令创建 Agent：

```bash
goclaw agents add taizi --model glm-5 --non-interactive
goclaw agents add zhongshu --model glm-5 --non-interactive
goclaw agents add menxia --model glm-5 --non-interactive
goclaw agents add shangshu --model glm-5 --non-interactive
```

### 3. 创建蜂群配置

运行设置脚本：

```powershell
.\setup_swarm.ps1
```

或手动创建配置文件 `~/.goclaw/swarms/sanshengliubu.json`：

```json
{
  "name": "sanshengliubu",
  "description": "三省六部协同工作蜂群",
  "agent_ids": ["taizi", "zhongshu", "menxia", "shangshu"],
  "flows": [
    {
      "name": "draft_edict",
      "description": "起草诏书流程",
      "from": "taizi",
      "to": "zhongshu",
      "condition": "contains('起草')"
    },
    {
      "name": "review_edict",
      "description": "审核诏书流程",
      "from": "zhongshu",
      "to": "menxia",
      "condition": "contains('审核')"
    },
    {
      "name": "approve_edict",
      "description": "批准诏书流程",
      "from": "menxia",
      "to": "taizi",
      "condition": "always"
    }
  ]
}
```

## CLI 命令

### 启动蜂群

```bash
goclaw swarm <name> start
```

选项：
- `-v, --verbose` - 详细输出
- `-t, --timeout int` - 蜂群超时时间（秒，默认 300）

示例：
```bash
goclaw swarm sanshengliubu start
goclaw swarm sanshengliubu start -v -t 600
```

### 查看状态

```bash
goclaw swarm <name> status
```

示例：
```bash
goclaw swarm sanshengliubu status
```

输出示例：
```json
{
  "name": "sanshengliubu",
  "running": true,
  "agents": {
    "taizi": "running",
    "zhongshu": "running",
    "menxia": "running",
    "shangshu": "running"
  },
  "pending_messages": 2
}
```

### 发送消息

```bash
goclaw swarm send <agent> <message>
```

示例：
```bash
goclaw swarm send taizi "请中书省起草一份减免赋税的诏书"
```

### 停止蜂群

```bash
goclaw swarm <name> stop
```

示例：
```bash
goclaw swarm sanshengliubu stop
```

### 列出所有蜂群

```bash
goclaw swarm list
```

输出示例：
```
Available swarms:
  - sanshengliubu (4 agents, 3 flows)
```

## 配置文件格式

蜂群配置文件位于 `~/.goclaw/swarms/<name>.json`：

```json
{
  "name": "string",           // 蜂群名称
  "description": "string",    // 蜂群描述
  "agent_ids": ["id1", ...],  // 要启动的 Agent ID 列表
  "flows": [...]              // 流程配置
}
```

### Flow 配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 流程名称 |
| `description` | string | 流程描述 |
| `from` | string | 消息来源 Agent ID |
| `to` | string | 消息目标 Agent ID |
| `condition` | string | 条件表达式 |

### 条件表达式

| 表达式 | 说明 |
|--------|------|
| `always` | 总是执行 |
| `contains('keyword')` | 消息包含指定关键字 |

## 架构设计

### 消息流转

```
┌─────────┐    异步发送    ┌─────────┐
│  Agent  │ ──────────────▶│  Agent  │
│  (发送方) │               │  (接收方) │
└─────────┘                └─────────┘
     ▲                          │
     │         回报消息          │
     └──────────────────────────┘
```

### 组件关系

```
┌─────────────────────────────────────────┐
│             SwarmManager                │
│  ┌─────────────────────────────────┐   │
│  │         MessageBus              │   │
│  │    (异步消息传递)                │   │
│  └─────────────────────────────────┘   │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐      │
│  │Agent│ │Agent│ │Agent│ │Agent│      │
│  │taizi│ │zhong│ │menxia│ │shang│      │
│  └─────┘ └─────┘ └─────┘ └─────┘      │
└─────────────────────────────────────────┘
```

## 示例场景

### 三省六部协作

```
太子 ──起草诏书──▶ 中书省 ──审核──▶ 门下省 ──批准──▶ 太子
                          │
                          ▼ 驳回
                        中书省
```

### 使用流程

1. 启动蜂群：
   ```bash
   goclaw swarm sanshengliubu start
   ```

2. 发送初始消息：
   ```bash
   goclaw swarm send taizi "请中书省起草一份减免赋税的诏书"
   ```

3. 消息自动流转：
   - 太子 → 中书省（起草诏书）
   - 中书省 → 门下省（审核）
   - 门下省 → 太子（最终批复）

4. 查看状态：
   ```bash
   goclaw swarm sanshengliubu status
   ```

5. 停止蜂群：
   ```bash
   goclaw swarm sanshengliubu stop
   ```

## 最佳实践

### 1. Agent 设计

- 每个 Agent 应有明确的职责
- IDENTITY.md 应详细描述 Agent 的角色和能力
- 避免职责重叠

### 2. 流程设计

- 流程步骤应简洁明了
- 条件判断应准确
- 考虑异常处理

### 3. 消息格式

- 使用结构化的消息格式
- 包含必要的上下文信息
- 避免过于冗长的消息

## 故障排除

### Agent 无法启动

检查：
- Agent 配置文件是否存在 (`~/.goclaw/agents/<id>.json`)
- 工作区目录是否创建
- IDENTITY.md 是否存在

### 消息未送达

检查：
- 目标 Agent 是否在运行
- 流程配置是否正确
- 条件表达式是否匹配

## 相关命令

- `goclaw agents` - 管理 Agent
- `goclaw agent` - 单个 Agent 交互
- `goclaw swarm` - 蜂群管理
