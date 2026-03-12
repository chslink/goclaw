package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/swarm"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var activeCorporateSwarm *swarm.CorporateSwarmManager

func init() {
	swarmCmd.AddCommand(swarmTasksCmd)
	swarmCmd.AddCommand(swarmApprovalsCmd)
	swarmCmd.AddCommand(swarmApproveCmd)
	swarmCmd.AddCommand(swarmRejectCmd)
}

var swarmTasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "List tasks in the corporate swarm task board",
	Run:   runSwarmTasks,
}

var swarmApprovalsCmd = &cobra.Command{
	Use:   "approvals",
	Short: "List pending approval requests",
	Run:   runSwarmApprovals,
}

var swarmApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve a pending approval request",
	Run:   runSwarmApprove,
}

var swarmRejectCmd = &cobra.Command{
	Use:   "reject <id> [reason]",
	Short: "Reject a pending approval request",
	Run:   runSwarmReject,
}

// runCorporateSwarmStartWithServices 使用基础服务启动公司化蜂群
func runCorporateSwarmStartWithServices(
	ctx context.Context,
	swarmName string,
	svc *SwarmBaseServices,
) {
	// 加载 corporate 配置
	configPath := fmt.Sprintf("%s/.goclaw/swarms/%s.json", svc.HomeDir, swarmName)
	corpCfg, err := swarm.LoadCorporateSwarmConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load corporate swarm config: %v\n", err)
		os.Exit(1)
	}

	// 创建 CorporateSwarmManager
	activeCorporateSwarm = swarm.NewCorporateSwarmManager(corpCfg, svc.Provider, svc.SessionMgr, svc.HomeDir)

	// 设置 IM 桥接：主通道总线 + 基础工具 + 技能加载器
	activeCorporateSwarm.SetChannelBus(svc.MessageBus)
	activeCorporateSwarm.SetBaseTools(svc.ToolRegistry.ListExisting())
	activeCorporateSwarm.SetSkillsLoader(svc.SkillsLoader)

	// 传递记忆搜索管理器
	if svc.MemorySearchMgr != nil {
		activeCorporateSwarm.SetMemorySearchManager(svc.MemorySearchMgr)
	}

	// 启动蜂群
	if err := activeCorporateSwarm.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start corporate swarm: %v\n", err)
		os.Exit(1)
	}

	// 注入 SwarmManager 到 Admin UI
	svc.Gateway.SetSwarmManager(activeCorporateSwarm.AdminAPI())

	agents := activeCorporateSwarm.ListAgents()
	fmt.Printf("Corporate swarm '%s' started with %d roles:\n", swarmName, len(agents))
	for _, agentID := range agents {
		fmt.Printf("  - %s\n", agentID)
	}
	fmt.Printf("Base services: %d channels, %d tools\n", len(svc.ChannelMgr.List()), svc.ToolRegistry.Count())

	// 启动入站桥接（主总线 → 蜂群）
	go func() {
		logger.Info("Inbound bridge started: IM → Corporate Swarm")
		for {
			msg, err := svc.MessageBus.ConsumeInbound(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("Inbound bridge consume error", zap.Error(err))
				continue
			}
			if msg == nil {
				continue
			}

			logger.Info("Inbound bridge forwarding message to corporate swarm",
				zap.String("channel", msg.Channel),
				zap.String("chat_id", msg.ChatID),
				zap.String("content", msg.Content))

			if err := activeCorporateSwarm.HandleInboundMessage(ctx, msg); err != nil {
				logger.Error("Corporate swarm failed to handle inbound message",
					zap.Error(err))
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nCorporate swarm is running with full IM support. Press Ctrl+C to stop.")

	select {
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, stopping corporate swarm...")
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %v, stopping corporate swarm...\n", sig)
	}

	if err := activeCorporateSwarm.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop corporate swarm: %v\n", err)
	}

	fmt.Println("Corporate swarm stopped")
}

func runSwarmTasks(cmd *cobra.Command, args []string) {
	if activeCorporateSwarm == nil {
		fmt.Println("No active corporate swarm")
		return
	}

	board := activeCorporateSwarm.GetTaskBoard()
	tasks := board.ListAll()

	if len(tasks) == 0 {
		fmt.Println("No tasks")
		return
	}

	fmt.Println(swarm.FormatTaskBoard(tasks))
}

func runSwarmApprovals(cmd *cobra.Command, args []string) {
	if activeCorporateSwarm == nil {
		fmt.Println("No active corporate swarm")
		return
	}

	approvalMgr := activeCorporateSwarm.GetApproval()
	pending := approvalMgr.GetPending()

	if len(pending) == 0 {
		fmt.Println("No pending approvals")
		return
	}

	fmt.Printf("Pending approvals (%d):\n", len(pending))
	for _, req := range pending {
		data, _ := json.MarshalIndent(req, "  ", "  ")
		fmt.Printf("  %s\n", string(data))
	}
}

func runSwarmApprove(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: approval ID is required\n")
		os.Exit(1)
	}

	if activeCorporateSwarm == nil {
		fmt.Println("No active corporate swarm")
		return
	}

	reqID := args[0]
	if err := activeCorporateSwarm.GetApproval().Approve(reqID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to approve: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Approval %s approved\n", reqID)
}

func runSwarmReject(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: approval ID is required\n")
		os.Exit(1)
	}

	if activeCorporateSwarm == nil {
		fmt.Println("No active corporate swarm")
		return
	}

	reqID := args[0]
	reason := ""
	if len(args) > 1 {
		reason = args[1]
	}

	if err := activeCorporateSwarm.GetApproval().Reject(reqID, reason); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reject: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Approval %s rejected\n", reqID)
}
