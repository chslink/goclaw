# Setup agents for multi-agent communication test
# Run this script manually in PowerShell

$homeDir = $env:USERPROFILE
$agentsDir = Join-Path $homeDir ".goclaw\agents"
$workspacesDir = Join-Path $homeDir ".goclaw\workspaces"

# Create directories
Write-Host "Creating directories..."
New-Item -ItemType Directory -Force -Path $agentsDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $workspacesDir "taizi") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $workspacesDir "zhongshu") | Out-Null

# Create taizi agent config with agent_call permissions
$taiziConfig = @{
    name = "taizi"
    workspace = Join-Path $workspacesDir "taizi"
    model = "glm-5"
    bindings = @()
    config_path = Join-Path $agentsDir "taizi.json"
    metadata = @{
        agent_call = @{
            allow_agents = @("zhongshu", "menxia", "shangshu")
        }
    }
}

$taiziPath = Join-Path $agentsDir "taizi.json"
$taiziConfig | ConvertTo-Json -Depth 10 | Set-Content $taiziPath -Encoding UTF8
Write-Host "Created: $taiziPath" -ForegroundColor Green

# Create zhongshu agent config
$zhongshuConfig = @{
    name = "zhongshu"
    workspace = Join-Path $workspacesDir "zhongshu"
    model = "glm-5"
    bindings = @()
    config_path = Join-Path $agentsDir "zhongshu.json"
}

$zhongshuPath = Join-Path $agentsDir "zhongshu.json"
$zhongshuConfig | ConvertTo-Json -Depth 10 | Set-Content $zhongshuPath -Encoding UTF8
Write-Host "Created: $zhongshuPath" -ForegroundColor Green

# Create taizi IDENTITY.md
$taiziIdentity = @"
---
summary: "太子身份记录"
---

# IDENTITY.md - 我是谁？

- **名称：** 太子
- **身份：** 皇太子，未来的皇帝
- **职责：** 协助皇帝处理政务，学习治国之道
- **表情符号：** 👑

---

你是太子，皇帝的长子和继承人。

## 可调用的机构

你可以调用以下机构协助处理事务：

- **中书省 (zhongshu)** - 负责起草诏书、圣旨等官方文书
- **门下省 (menxia)** - 负责审核诏书，有权封驳
- **尚书省 (shangshu)** - 负责执行政令

## 使用 agent_call 工具

当你需要其他机构协助时使用 agent_call 工具：

```
agent_call(agent_name="zhongshu", message="请帮我起草一份诏书")
```
"@

$taiziIdentityPath = Join-Path $workspacesDir "taizi\IDENTITY.md"
$taiziIdentity | Set-Content $taiziIdentityPath -Encoding UTF8
Write-Host "Created: $taiziIdentityPath" -ForegroundColor Green

# Create zhongshu IDENTITY.md
$zhongshuIdentity = @"
---
summary: "中书省身份记录"
---

# IDENTITY.md - 我是谁？

- **名称：** 中书省
- **职责：** 负责起草诏书、圣旨、奏折等官方文书
- **特点：** 文言文风格，正式、庄重
- **表情符号：** 📜

---

你是中书省，负责起草各种官方文书。

## 职责

1. **起草诏书** - 根据皇帝的意图起草正式诏书
2. **起草圣旨** - 起草圣旨，使用文言文格式
3. **起草奏折** - 为官员起草奏折
4. **文书润色** - 对文书进行润色和修改

## 回复风格

- 使用文言文风格
- 正式、庄重的语气
- 使用适当的敬语和称谓
- 文书格式规范

## 诏书格式

奉天承运，皇帝诏曰：
[正文内容]
钦此
"@

$zhongshuIdentityPath = Join-Path $workspacesDir "zhongshu\IDENTITY.md"
$zhongshuIdentity | Set-Content $zhongshuIdentityPath -Encoding UTF8
Write-Host "Created: $zhongshuIdentityPath" -ForegroundColor Green

Write-Host ""
Write-Host "=== Setup Complete ===" -ForegroundColor Green
Write-Host "Agents created: taizi, zhongshu"
Write-Host ""
Write-Host "Test commands:" -ForegroundColor Yellow
Write-Host "  .\goclaw.exe agent -m '请中书省帮我起草一份关于减免赋税的诏书' --agent taizi"
