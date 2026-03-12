package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/smallnest/goclaw/config"
	workspace2 "github.com/smallnest/goclaw/internal/workspace"
	"github.com/smallnest/goclaw/swarm"
	"github.com/spf13/cobra"
)

var swarmDeployCmd = &cobra.Command{
	Use:   "deploy <config-file>",
	Short: "Deploy a swarm from config file (create agents + install config)",
	Long: `Deploy a swarm by creating all required agents and installing the swarm config.

This command automates the full setup process:
  1. Reads the swarm configuration file
  2. Creates agent entries for each role (with workspace and IDENTITY)
  3. Installs the swarm config to ~/.goclaw/swarms/<name>.json
  4. Reports deployment status

After deployment, start the swarm with:
  goclaw swarm start <name>

Examples:
  goclaw swarm deploy examples/swarm/cpp2go-refactor.json
  goclaw swarm deploy examples/swarm/corporate.json --identity cpp2go
  goclaw swarm deploy myswarm.json --force`,
	Args: cobra.ExactArgs(1),
	Run:  runSwarmDeploy,
}

var (
	deployIdentityVariant string
	deployForce           bool
	deployJSON            bool
)

func init() {
	swarmCmd.AddCommand(swarmDeployCmd)
	swarmDeployCmd.Flags().StringVar(&deployIdentityVariant, "identity", "base", "Identity variant for management roles (base, cpp2go)")
	swarmDeployCmd.Flags().BoolVar(&deployForce, "force", false, "Overwrite existing agents and config")
	swarmDeployCmd.Flags().BoolVar(&deployJSON, "json", false, "Output in JSON format")
}

// deployResult 部署结果
type deployResult struct {
	SwarmName     string   `json:"swarm_name"`
	Mode          string   `json:"mode"`
	ConfigPath    string   `json:"config_path"`
	AgentsCreated []string `json:"agents_created"`
	AgentsSkipped []string `json:"agents_skipped,omitempty"`
	WorkerRoles   []string `json:"worker_roles,omitempty"`
	Success       bool     `json:"success"`
}

func runSwarmDeploy(cmd *cobra.Command, args []string) {
	configFile := args[0]

	// 1. 检查配置文件存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 配置文件不存在: %s\n", configFile)
		os.Exit(1)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法获取用户目录: %v\n", err)
		os.Exit(1)
	}

	// 2. 读取配置文件原始 JSON，判断 mode
	rawData, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法读取配置文件: %v\n", err)
		os.Exit(1)
	}

	var rawCfg struct {
		Name string `json:"name"`
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(rawData, &rawCfg); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 配置文件格式无效: %v\n", err)
		os.Exit(1)
	}

	if rawCfg.Name == "" {
		fmt.Fprintf(os.Stderr, "错误: 配置文件缺少 name 字段\n")
		os.Exit(1)
	}

	// 3. 安装 swarm 配置到 ~/.goclaw/swarms/
	swarmDir := filepath.Join(homeDir, ".goclaw", "swarms")
	if err := os.MkdirAll(swarmDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法创建 swarms 目录: %v\n", err)
		os.Exit(1)
	}

	targetConfigPath := filepath.Join(swarmDir, rawCfg.Name+".json")
	if !deployForce {
		if _, err := os.Stat(targetConfigPath); err == nil {
			fmt.Fprintf(os.Stderr, "错误: 蜂群 '%s' 已存在。使用 --force 覆盖\n", rawCfg.Name)
			os.Exit(1)
		}
	}

	if err := os.WriteFile(targetConfigPath, rawData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法安装蜂群配置: %v\n", err)
		os.Exit(1)
	}

	// 4. 根据 mode 分派部署逻辑
	if rawCfg.Mode == "corporate" {
		deployCorporateSwarm(homeDir, configFile, rawCfg.Name, targetConfigPath)
	} else {
		deployFlatSwarm(homeDir, configFile, rawCfg.Name, targetConfigPath)
	}
}

// deployCorporateSwarm 部署公司化蜂群
func deployCorporateSwarm(homeDir, configFile, swarmName, installedPath string) {
	corpCfg, err := swarm.LoadCorporateSwarmConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法解析公司化蜂群配置: %v\n", err)
		os.Exit(1)
	}

	result := deployResult{
		SwarmName:  swarmName,
		Mode:       "corporate",
		ConfigPath: installedPath,
		Success:    true,
	}

	agentsDir := filepath.Join(homeDir, ".goclaw", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法创建 agents 目录: %v\n", err)
		os.Exit(1)
	}

	// 管理层角色列表
	type roleEntry struct {
		agentID   string
		role      string
		model     string
		identity  string
	}

	// 根据 identity variant 选择 IDENTITY 模板
	var secretaryIdentity, hrIdentity, pmIdentity string
	switch deployIdentityVariant {
	case "cpp2go":
		secretaryIdentity = swarm.SecretaryIdentityCpp2Go(swarmName, corpCfg.HR.AgentID, corpCfg.PM.AgentID)
		hrIdentity = swarm.HRIdentityCpp2Go(swarmName, corpCfg.Secretary.AgentID, corpCfg.PM.AgentID)
		pmIdentity = swarm.PMIdentityCpp2Go(swarmName, corpCfg.Secretary.AgentID, corpCfg.HR.AgentID)
	default:
		secretaryIdentity = swarm.SecretaryIdentity(swarmName, corpCfg.HR.AgentID, corpCfg.PM.AgentID)
		hrIdentity = swarm.HRIdentity(swarmName, corpCfg.Secretary.AgentID, corpCfg.PM.AgentID)
		pmIdentity = swarm.PMIdentity(swarmName, corpCfg.Secretary.AgentID, corpCfg.HR.AgentID)
	}

	roles := []roleEntry{
		{corpCfg.Secretary.AgentID, "secretary", corpCfg.Secretary.Model, secretaryIdentity},
		{corpCfg.HR.AgentID, "hr", corpCfg.HR.Model, hrIdentity},
		{corpCfg.PM.AgentID, "pm", corpCfg.PM.Model, pmIdentity},
	}

	// 创建管理层 Agent
	for _, r := range roles {
		created, err := deployOneAgent(homeDir, swarmName, r.agentID, r.role, r.model, r.identity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 创建 %s agent 失败: %v\n", r.role, err)
			os.Exit(1)
		}
		if created {
			result.AgentsCreated = append(result.AgentsCreated, r.agentID)
		} else {
			result.AgentsSkipped = append(result.AgentsSkipped, r.agentID)
		}
	}

	// 收集 Worker 角色
	for _, tmpl := range corpCfg.WorkerPool.Templates {
		result.WorkerRoles = append(result.WorkerRoles, tmpl.Role+" ("+tmpl.Name+")")
	}

	// 输出结果
	if deployJSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		printCorporateDeployResult(result, corpCfg)
	}
}

// deployFlatSwarm 部署普通蜂群
func deployFlatSwarm(homeDir, configFile, swarmName, installedPath string) {
	flatCfg, err := swarm.LoadSwarmConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法解析蜂群配置: %v\n", err)
		os.Exit(1)
	}

	result := deployResult{
		SwarmName:  swarmName,
		Mode:       "flat",
		ConfigPath: installedPath,
		Success:    true,
	}

	// 为每个 agent_id 创建 Agent（如果不存在）
	for _, agentID := range flatCfg.AgentIDs {
		created, err := deployOneAgent(homeDir, swarmName, agentID, agentID, "", "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 创建 agent '%s' 失败: %v\n", agentID, err)
			os.Exit(1)
		}
		if created {
			result.AgentsCreated = append(result.AgentsCreated, agentID)
		} else {
			result.AgentsSkipped = append(result.AgentsSkipped, agentID)
		}
	}

	if deployJSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		printFlatDeployResult(result, flatCfg)
	}
}

// deployOneAgent 创建单个 Agent（配置 + 工作区 + IDENTITY）
// 返回 (是否新建, 错误)
func deployOneAgent(homeDir, swarmName, agentID, role, model, identity string) (bool, error) {
	agentsDir := filepath.Join(homeDir, ".goclaw", "agents")
	agentConfigPath := filepath.Join(agentsDir, agentID+".json")

	// 检查是否已存在
	if !deployForce {
		if _, err := os.Stat(agentConfigPath); err == nil {
			return false, nil
		}
	}

	// 确定 workspace 路径（公司化蜂群用 swarmName/role 子目录）
	workspace := filepath.Join(homeDir, ".goclaw", "workspaces", swarmName, role)

	if model == "" {
		model = "gpt-4"
	}

	// 创建 Agent 配置
	agentCfg := &config.AgentConfig{
		ID:         agentID,
		Name:       agentID,
		Workspace:  workspace,
		Model:      model,
		ConfigPath: agentConfigPath,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"swarm":      swarmName,
			"role":       role,
			"deploy_via": "swarm deploy",
		},
	}

	if err := config.SaveAgent(agentCfg); err != nil {
		return false, fmt.Errorf("保存 agent 配置失败: %w", err)
	}

	// 初始化工作区
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return false, fmt.Errorf("创建工作区失败: %w", err)
	}

	wsMgr := workspace2.NewManager(workspace)
	if err := wsMgr.Ensure(); err != nil {
		return false, fmt.Errorf("初始化工作区失败: %w", err)
	}

	// 写入 IDENTITY.md（如果提供了 identity 内容）
	if identity != "" {
		identityPath := filepath.Join(workspace, "IDENTITY.md")
		if err := os.WriteFile(identityPath, []byte(identity), 0644); err != nil {
			return false, fmt.Errorf("写入 IDENTITY.md 失败: %w", err)
		}
	}

	return true, nil
}

// printCorporateDeployResult 打印公司化蜂群部署结果
func printCorporateDeployResult(result deployResult, cfg *config.CorporateSwarmConfig) {
	fmt.Printf("蜂群 '%s' 部署完成 (corporate 模式)\n", result.SwarmName)
	fmt.Printf("配置已安装到: %s\n\n", result.ConfigPath)

	// 管理层
	fmt.Println("管理层:")
	fmt.Printf("  Secretary : %s (model: %s)\n", cfg.Secretary.AgentID, cfg.Secretary.Model)
	fmt.Printf("  HR        : %s (model: %s)\n", cfg.HR.AgentID, cfg.HR.Model)
	fmt.Printf("  PM        : %s (model: %s)\n", cfg.PM.AgentID, cfg.PM.Model)

	// Agent 创建情况
	if len(result.AgentsCreated) > 0 {
		fmt.Printf("\n新建 Agent (%d):\n", len(result.AgentsCreated))
		for _, id := range result.AgentsCreated {
			fmt.Printf("  + %s\n", id)
		}
	}
	if len(result.AgentsSkipped) > 0 {
		fmt.Printf("\n已存在跳过 (%d):\n", len(result.AgentsSkipped))
		for _, id := range result.AgentsSkipped {
			fmt.Printf("  ~ %s\n", id)
		}
	}

	// Worker 模板
	if len(result.WorkerRoles) > 0 {
		fmt.Printf("\nWorker 模板 (%d):\n", len(result.WorkerRoles))
		for _, role := range result.WorkerRoles {
			fmt.Printf("  - %s\n", role)
		}
	}

	// 后续步骤
	fmt.Println("\n后续步骤:")
	fmt.Printf("  1. 自定义 Agent 身份:  goclaw agents identity %s\n", cfg.Secretary.AgentID)
	fmt.Printf("  2. 查看已部署 Agent:   goclaw agents list\n")
	fmt.Printf("  3. 启动蜂群:           goclaw swarm start %s\n", result.SwarmName)
}

// printFlatDeployResult 打印普通蜂群部署结果
func printFlatDeployResult(result deployResult, cfg *swarm.SwarmConfig) {
	fmt.Printf("蜂群 '%s' 部署完成 (flat 模式)\n", result.SwarmName)
	fmt.Printf("配置已安装到: %s\n\n", result.ConfigPath)

	fmt.Printf("Agent (%d):\n", len(cfg.AgentIDs))
	for _, id := range cfg.AgentIDs {
		fmt.Printf("  - %s\n", id)
	}

	if len(result.AgentsCreated) > 0 {
		fmt.Printf("\n新建 Agent (%d):\n", len(result.AgentsCreated))
		for _, id := range result.AgentsCreated {
			fmt.Printf("  + %s\n", id)
		}
	}
	if len(result.AgentsSkipped) > 0 {
		fmt.Printf("\n已存在跳过 (%d):\n", len(result.AgentsSkipped))
		for _, id := range result.AgentsSkipped {
			fmt.Printf("  ~ %s\n", id)
		}
	}

	if len(cfg.Flows) > 0 {
		fmt.Printf("\n消息流转规则 (%d):\n", len(cfg.Flows))
		for _, flow := range cfg.Flows {
			fmt.Printf("  %s: %s → %s\n", flow.Name, flow.From, flow.To)
		}
	}

	fmt.Println("\n后续步骤:")
	fmt.Printf("  1. 查看已部署 Agent:   goclaw agents list\n")
	fmt.Printf("  2. 启动蜂群:           goclaw swarm start %s\n", result.SwarmName)
}

