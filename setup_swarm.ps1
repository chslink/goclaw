# Setup sanshengliubu swarm
# Run this script manually in PowerShell

$homeDir = $env:USERPROFILE
$swarmDir = Join-Path $homeDir ".goclaw\swarms"

# Create directories
Write-Host "Creating directories..." -ForegroundColor Green
New-Item -ItemType Directory -Force -Path $swarmDir | Out-Null

# Create swarm config (simplified - only agent IDs)
$swarmConfig = @{
    name = "sanshengliubu"
    description = "三省六部协同工作蜂群"
    agent_ids = @("taizi", "zhongshu", "menxia", "shangshu")
    flows = @(
        @{
            name = "draft_edict"
            description = "起草诏书流程"
            from = "taizi"
            to = "zhongshu"
            condition = "contains('起草')"
        },
        @{
            name = "review_edict"
            description = "审核诏书流程"
            from = "zhongshu"
            to = "menxia"
            condition = "contains('审核')"
        },
        @{
            name = "approve_edict"
            description = "批准诏书流程"
            from = "menxia"
            to = "taizi"
            condition = "always"
        }
    )
}

$swarmPath = Join-Path $swarmDir "sanshengliubu.json"
$swarmConfig | ConvertTo-Json -Depth 10 | Set-Content $swarmPath -Encoding UTF8
Write-Host "Created: $swarmPath" -ForegroundColor Green

Write-Host ""
Write-Host "=== Setup Complete ===" -ForegroundColor Green
Write-Host "Swarm 'sanshengliubu' created"
Write-Host ""
Write-Host "Note: Make sure agents exist before starting the swarm." -ForegroundColor Yellow
Write-Host "Use 'goclaw agents add' to create agents if needed."
Write-Host ""
Write-Host "Usage:" -ForegroundColor Yellow
Write-Host "  .\goclaw.exe swarm sanshengliubu start"
Write-Host "  .\goclaw.exe swarm sanshengliubu status"
Write-Host "  .\goclaw.exe swarm sanshengliubu stop"
Write-Host "  .\goclaw.exe swarm send <agent> <message>"
