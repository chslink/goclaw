package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/swarm"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var swarmCmd = &cobra.Command{
	Use:   "swarm",
	Short: "Manage swarm of agents",
	Long: `Manage a swarm of agents that work together asynchronously.

Examples:
  goclaw swarm start sanshengliubu        Start the sanshengliubu swarm
  goclaw swarm status sanshengliubu       Show swarm status
  goclaw swarm stop                       Stop the running swarm
  goclaw swarm list                       List all swarms
  goclaw swarm deploy config.json         Deploy a swarm from config file`,
}

var swarmStartCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start a swarm",
	Long: `Start a swarm of agents. All agents will be initialized and wait for messages.

The swarm configuration is loaded from ~/.goclaw/swarms/<name>.json

Examples:
  goclaw swarm start sanshengliubu
  goclaw swarm start cpp2go-refactor -v`,
	Args: cobra.ExactArgs(1),
	Run:  runSwarmStart,
}

var swarmStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a swarm",
	Run:   runSwarmStop,
}

var swarmStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show swarm status",
	Args:  cobra.ExactArgs(1),
	Run:   runSwarmStatus,
}

var swarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all swarms",
	Run:   runSwarmList,
}

var swarmSendCmd = &cobra.Command{
	Use:   "send <agent> <message>",
	Short: "Send a message to an agent in the swarm",
	Run:   runSwarmSend,
}

var swarmAddAgentCmd = &cobra.Command{
	Use:   "add-agent <agent-id>",
	Short: "Add an agent to the running swarm (via Admin API)",
	Long: `Dynamically add an agent to the running swarm via Admin HTTP API.

If --workspace and --model are not specified, the agent config is loaded from disk.

Examples:
  goclaw swarm add-agent new-worker
  goclaw swarm add-agent temp-agent --workspace /tmp/temp --model gpt-4
  goclaw swarm add-agent new-worker --port 28789`,
	Args: cobra.ExactArgs(1),
	Run:  runSwarmAddAgent,
}

var swarmRemoveAgentCmd = &cobra.Command{
	Use:   "remove-agent <agent-id>",
	Short: "Remove an agent from the running swarm (via Admin API)",
	Long: `Dynamically remove an agent from the running swarm via Admin HTTP API.

Examples:
  goclaw swarm remove-agent old-worker
  goclaw swarm remove-agent old-worker --port 28789`,
	Args: cobra.ExactArgs(1),
	Run:  runSwarmRemoveAgent,
}

var (
	swarmVerbose        bool
	swarmTimeout        int
	swarmAdminPort      int
	swarmAgentWorkspace string
	swarmAgentModel     string
)

var activeSwarm *swarm.SwarmManager

func init() {
	rootCmd.AddCommand(swarmCmd)
	swarmCmd.AddCommand(swarmStartCmd)
	swarmCmd.AddCommand(swarmStopCmd)
	swarmCmd.AddCommand(swarmStatusCmd)
	swarmCmd.AddCommand(swarmListCmd)
	swarmCmd.AddCommand(swarmSendCmd)
	swarmCmd.AddCommand(swarmAddAgentCmd)
	swarmCmd.AddCommand(swarmRemoveAgentCmd)

	swarmCmd.PersistentFlags().BoolVarP(&swarmVerbose, "verbose", "v", false, "Verbose output")
	swarmStartCmd.Flags().IntVarP(&swarmTimeout, "timeout", "t", 300, "Swarm timeout in seconds")

	swarmAddAgentCmd.Flags().IntVar(&swarmAdminPort, "port", 28789, "Admin API port")
	swarmAddAgentCmd.Flags().StringVar(&swarmAgentWorkspace, "workspace", "", "Agent workspace path")
	swarmAddAgentCmd.Flags().StringVar(&swarmAgentModel, "model", "", "Agent model")

	swarmRemoveAgentCmd.Flags().IntVar(&swarmAdminPort, "port", 28789, "Admin API port")
}

func runSwarmStart(cmd *cobra.Command, args []string) {
	swarmName := args[0]

	// 1. 初始化基础服务（IM 通道、工具、网关等）
	svc := initSwarmBaseServices()
	defer svc.Shutdown()

	homeDir := svc.HomeDir

	// 2. 加载蜂群配置
	swarmConfigPath := filepath.Join(homeDir, ".goclaw", "swarms", swarmName+".json")
	swarmCfg, err := swarm.LoadSwarmConfig(swarmConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load swarm config '%s': %v\n", swarmConfigPath, err)
		os.Exit(1)
	}

	// 3. 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. 启动基础服务
	if err := svc.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start base services: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Swarm base services started for swarm",
		zap.String("swarm", swarmName),
		zap.Int("channels", len(svc.ChannelMgr.List())),
		zap.Int("tools", svc.ToolRegistry.Count()))

	// 5. 根据 mode 分派
	if swarmCfg.Mode == "corporate" {
		runCorporateSwarmStartWithServices(ctx, swarmName, svc)
		return
	}

	// Flat 模式
	runFlatSwarmStartWithServices(ctx, swarmName, swarmCfg, svc)
}

// runFlatSwarmStartWithServices 使用基础服务启动 flat 蜂群
func runFlatSwarmStartWithServices(ctx context.Context, swarmName string, swarmCfg *swarm.SwarmConfig, svc *SwarmBaseServices) {
	activeSwarm = swarm.NewSwarmManager(swarmCfg, svc.Provider, svc.SessionMgr, svc.HomeDir)

	// 传递记忆搜索管理器
	if svc.MemorySearchMgr != nil {
		activeSwarm.SetMemorySearchManager(svc.MemorySearchMgr)
	}

	if err := activeSwarm.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start swarm: %v\n", err)
		os.Exit(1)
	}

	// 注入 SwarmManager 到 Admin UI
	svc.Gateway.SetSwarmManager(activeSwarm.AdminAPI())

	fmt.Printf("Swarm '%s' started with %d agents\n", swarmName, len(swarmCfg.AgentIDs))
	for _, agentID := range swarmCfg.AgentIDs {
		fmt.Printf("  - %s\n", agentID)
	}
	fmt.Printf("Base services: %d channels, %d tools\n", len(svc.ChannelMgr.List()), svc.ToolRegistry.Count())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nSwarm is running. Press Ctrl+C to stop.")

	select {
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, stopping swarm...")
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %v, stopping swarm...\n", sig)
	}

	if err := activeSwarm.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop swarm: %v\n", err)
	}

	fmt.Println("Swarm stopped")
}

func runSwarmStop(cmd *cobra.Command, args []string) {
	if activeSwarm == nil {
		fmt.Println("No active swarm to stop")
		return
	}

	if err := activeSwarm.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop swarm: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Swarm stopped")
	activeSwarm = nil
}

func runSwarmStatus(cmd *cobra.Command, args []string) {
	if activeSwarm == nil {
		fmt.Println("No active swarm")
		return
	}

	status := activeSwarm.GetStatus()
	data, _ := json.MarshalIndent(status, "", "  ")
	fmt.Println(string(data))
}

func runSwarmList(cmd *cobra.Command, args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	swarmDir := filepath.Join(homeDir, ".goclaw", "swarms")
	entries, err := os.ReadDir(swarmDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No swarms configured")
			return
		}
		fmt.Fprintf(os.Stderr, "Failed to read swarms directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Available swarms:")
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			name := entry.Name()[:len(entry.Name())-5]
			configPath := filepath.Join(swarmDir, entry.Name())

			cfg, err := swarm.LoadSwarmConfig(configPath)
			if err != nil {
				fmt.Printf("  - %s (error loading config)\n", name)
				continue
			}

			if cfg.Mode == "corporate" {
				// 尝试以 corporate 格式加载获取详情
				corpCfg, err := swarm.LoadCorporateSwarmConfig(configPath)
				if err == nil {
					workerCount := len(corpCfg.WorkerPool.Templates)
					fmt.Printf("  - %s [corporate] (3 managers + %d worker templates)\n", name, workerCount)
				} else {
					fmt.Printf("  - %s [corporate]\n", name)
				}
			} else {
				fmt.Printf("  - %s [flat] (%d agents, %d flows)\n", name, len(cfg.AgentIDs), len(cfg.Flows))
			}
		}
	}
}

func runSwarmSend(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: agent and message are required\n")
		os.Exit(1)
	}

	if activeSwarm == nil {
		fmt.Println("No active swarm")
		os.Exit(1)
	}

	agentID := args[0]
	message := args[1]

	ctx := context.Background()

	if err := activeSwarm.SendMessage(ctx, "cli", agentID, message, "manual"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send message: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Message sent to agent '%s'\n", agentID)
}

func runSwarmAddAgent(cmd *cobra.Command, args []string) {
	agentID := args[0]

	reqBody := map[string]string{
		"agent_id": agentID,
	}
	if swarmAgentWorkspace != "" {
		reqBody["workspace"] = swarmAgentWorkspace
	}
	if swarmAgentModel != "" {
		reqBody["model"] = swarmAgentModel
	}

	data, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("http://localhost:%d/admin/api/swarms/agents", swarmAdminPort)

	resp, err := http.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to admin API: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure the swarm is running with gateway enabled on port %d\n", swarmAdminPort)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Failed to add agent: %v\n", result["error"])
		os.Exit(1)
	}

	fmt.Printf("Agent '%s' added successfully\n", agentID)
	if agents, ok := result["agents"].([]interface{}); ok {
		fmt.Printf("Current agents (%d):\n", len(agents))
		for _, a := range agents {
			fmt.Printf("  - %s\n", a)
		}
	}
}

func runSwarmRemoveAgent(cmd *cobra.Command, args []string) {
	agentID := args[0]

	url := fmt.Sprintf("http://localhost:%d/admin/api/swarms/agents/%s", swarmAdminPort, agentID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create request: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to admin API: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure the swarm is running with gateway enabled on port %d\n", swarmAdminPort)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Failed to remove agent: %v\n", result["error"])
		os.Exit(1)
	}

	fmt.Printf("Agent '%s' removed successfully\n", agentID)
	if agents, ok := result["agents"].([]interface{}); ok {
		fmt.Printf("Current agents (%d):\n", len(agents))
		for _, a := range agents {
			fmt.Printf("  - %s\n", a)
		}
	}
}
