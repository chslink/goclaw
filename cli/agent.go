package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/agent"
	"github.com/smallnest/goclaw/agent/tools"
	"github.com/smallnest/goclaw/bus"
	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/internal/logger"
	workspace2 "github.com/smallnest/goclaw/internal/workspace"
	"github.com/smallnest/goclaw/providers"
	"github.com/smallnest/goclaw/session"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run one agent turn",
	Long:  `Execute a single agent interaction with a message and optional parameters.`,
	Run:   runAgent,
}

// Flags for agent command
var (
	agentMessage       string
	agentTo            string
	agentID            string
	agentSessionID     string
	agentThinking      string
	agentVerbose       bool
	agentChannel       string
	agentLocal         bool
	agentDeliver       bool
	agentJSON          bool
	agentTimeout       int
	agentMaxIterations int
	agentStream        bool
)

func init() {
	agentCmd.Flags().StringVarP(&agentMessage, "message", "m", "", "Message to send to the agent")
	agentCmd.Flags().StringVar(&agentTo, "to", "", "Recipient number in E.164 used to derive the session key")
	agentCmd.Flags().StringVar(&agentID, "agent", "", "Agent id (overrides routing bindings)")
	agentCmd.Flags().StringVar(&agentSessionID, "session-id", "", "Use an explicit session id")
	agentCmd.Flags().StringVar(&agentThinking, "thinking", "off", "Thinking level: off | minimal | low | medium | high")
	agentCmd.Flags().BoolVar(&agentVerbose, "verbose", false, "Persist agent verbose level for the session")
	agentCmd.Flags().StringVar(&agentChannel, "channel", "", "Delivery channel: last|telegram|whatsapp|discord|irc|googlechat|slack|signal|imessage|feishu|nostr|msteams|mattermost|nextcloud-talk|matrix|bluebubbles|line|zalo|wecom|zalouser|synology-chat|tlon")
	agentCmd.Flags().BoolVar(&agentLocal, "local", false, "Run the embedded agent locally (requires model provider API keys in your shell)")
	agentCmd.Flags().BoolVar(&agentDeliver, "deliver", false, "Send the agent's reply back to the selected channel")
	agentCmd.Flags().BoolVar(&agentJSON, "json", false, "Output result as JSON")
	agentCmd.Flags().IntVar(&agentTimeout, "timeout", 600, "Override agent command timeout (seconds)")
	agentCmd.Flags().IntVar(&agentMaxIterations, "max-iterations", 15, "Maximum agent loop iterations")
	agentCmd.Flags().BoolVar(&agentStream, "stream", false, "Enable streaming output")

	_ = agentCmd.MarkFlagRequired("message")
}

// runAgent executes a single agent turn
func runAgent(cmd *cobra.Command, args []string) {
	// Validate message
	if agentMessage == "" {
		fmt.Fprintf(os.Stderr, "Error: --message is required\n")
		os.Exit(1)
	}

	// Validate that either --agent or --session-id is specified
	if agentID == "" && agentSessionID == "" {
		fmt.Fprintf(os.Stderr, "Error: either --agent or --session-id is required\n")
		os.Exit(1)
	}

	// Initialize logger if verbose or thinking mode is enabled
	if agentVerbose || agentThinking != "off" {
		if err := logger.Init("debug", false); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = logger.Sync() }()
	}

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	// Determine workspace based on --agent parameter
	var workspace string
	var agentCfgFile *config.AgentConfig
	if agentID != "" {
		agentCfgFile, err = config.LoadAgentByName(agentID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load agent '%s': %v\n", agentID, err)
			os.Exit(1)
		}
		workspace = agentCfgFile.Workspace
		if agentVerbose {
			fmt.Fprintf(os.Stderr, "Using agent '%s' with workspace: %s\n", agentID, workspace)
		}
	} else {
		workspace = homeDir + "/.goclaw/workspace"
	}

	// Ensure workspace directory exists and is initialized
	wsMgr := workspace2.NewManager(workspace)
	if err := wsMgr.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize workspace: %v\n", err)
		os.Exit(1)
	}

	// Create message bus
	messageBus := bus.NewMessageBus(100)
	defer messageBus.Close()

	// Create session manager
	sessionDir := homeDir + "/.goclaw/sessions"
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create sessions directory: %v\n", err)
		os.Exit(1)
	}
	sessionMgr, err := session.NewManager(sessionDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session manager: %v\n", err)
		os.Exit(1)
	}

	// Create memory store
	memoryStore := agent.NewMemoryStore(workspace)
	if err := memoryStore.EnsureBootstrapFiles(); err != nil {
		if agentVerbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create bootstrap files: %v\n", err)
		}
	}

	// Create context builder
	contextBuilder := agent.NewContextBuilder(memoryStore, workspace)

	// Create tool registry
	toolRegistry := agent.NewToolRegistry()

	// Register file system tool
	fsTool := tools.NewFileSystemTool(cfg.Tools.FileSystem.AllowedPaths, cfg.Tools.FileSystem.DeniedPaths, workspace)
	for _, tool := range fsTool.GetTools() {
		if err := toolRegistry.RegisterExisting(tool); err != nil && agentVerbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to register tool %s: %v\n", tool.Name(), err)
		}
	}

	// Register shell tool
	shellTool := tools.NewShellTool(
		cfg.Tools.Shell.Enabled,
		cfg.Tools.Shell.AllowedCmds,
		cfg.Tools.Shell.DeniedCmds,
		cfg.Tools.Shell.Timeout,
		cfg.Tools.Shell.WorkingDir,
		cfg.Tools.Shell.Sandbox,
	)
	for _, tool := range shellTool.GetTools() {
		if err := toolRegistry.RegisterExisting(tool); err != nil && agentVerbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to register tool %s: %v\n", tool.Name(), err)
		}
	}

	// Register web tool
	webTool := tools.NewWebTool(
		cfg.Tools.Web.SearchAPIKey,
		cfg.Tools.Web.SearchEngine,
		cfg.Tools.Web.Timeout,
	)
	for _, tool := range webTool.GetTools() {
		if err := toolRegistry.RegisterExisting(tool); err != nil && agentVerbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to register tool %s: %v\n", tool.Name(), err)
		}
	}

	// Register browser tool if enabled
	if cfg.Tools.Browser.Enabled {
		browserTool := tools.NewBrowserTool(
			cfg.Tools.Browser.Headless,
			cfg.Tools.Browser.Timeout,
		)
		for _, tool := range browserTool.GetTools() {
			if err := toolRegistry.RegisterExisting(tool); err != nil && agentVerbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to register browser tool %s: %v\n", tool.Name(), err)
			}
		}
	}

	// Register use_skill tool
	if err := toolRegistry.RegisterExisting(tools.NewUseSkillTool()); err != nil && agentVerbose {
		fmt.Fprintf(os.Stderr, "Warning: Failed to register use_skill: %v\n", err)
	}

	// Register agent_call tool
	agentCallTool := agent.NewAgentCallTool(messageBus)
	agentCallTool.SetAgentConfigGetter(func(agentID string) *config.AgentConfig {
		// First check in main config
		for _, agentCfg := range cfg.Agents.List {
			if agentCfg.ID == agentID {
				return &agentCfg
			}
		}
		// Then try to load from agent config file
		if loaded, err := config.LoadAgentByName(agentID); err == nil {
			return loaded
		}
		return nil
	})
	agentCallTool.SetAgentExistsChecker(func(agentID string) bool {
		return config.AgentExists(agentID)
	})
	// Set callAgent callback to directly call target agent
	agentCallTool.SetCallAgent(func(ctx context.Context, targetAgentID string, message string) (string, error) {
		// Load target agent config
		targetAgentCfg, err := config.LoadAgentByName(targetAgentID)
		if err != nil {
			return "", fmt.Errorf("failed to load agent config: %w", err)
		}

		// Get target agent workspace
		targetWorkspace := targetAgentCfg.Workspace
		if targetWorkspace == "" {
			targetWorkspace = filepath.Join(homeDir, ".goclaw", "workspaces", targetAgentID)
		}

		// Create target agent memory store and context builder
		targetMemoryStore := agent.NewMemoryStore(targetWorkspace)
		targetContextBuilder := agent.NewContextBuilder(targetMemoryStore, targetWorkspace)

		// Create target agent tool registry
		targetToolRegistry := agent.NewToolRegistry()

		// Register file system tool for target agent
		targetFsTool := tools.NewFileSystemTool(cfg.Tools.FileSystem.AllowedPaths, cfg.Tools.FileSystem.DeniedPaths, targetWorkspace)
		for _, tool := range targetFsTool.GetTools() {
			if err := targetToolRegistry.RegisterExisting(tool); err != nil && agentVerbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to register tool %s: %v\n", tool.Name(), err)
			}
		}

		// Register shell tool for target agent
		targetShellTool := tools.NewShellTool(
			cfg.Tools.Shell.Enabled,
			cfg.Tools.Shell.AllowedCmds,
			cfg.Tools.Shell.DeniedCmds,
			cfg.Tools.Shell.Timeout,
			cfg.Tools.Shell.WorkingDir,
			cfg.Tools.Shell.Sandbox,
		)
		for _, tool := range targetShellTool.GetTools() {
			if err := targetToolRegistry.RegisterExisting(tool); err != nil && agentVerbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to register tool %s: %v\n", tool.Name(), err)
			}
		}

		// Create target agent
		targetProvider, providerErr := providers.NewProvider(cfg)
		if providerErr != nil {
			return "", fmt.Errorf("failed to create target provider: %w", providerErr)
		}
		targetAgent, err := agent.NewAgent(&agent.NewAgentConfig{
			Bus:          messageBus,
			Provider:     targetProvider,
			SessionMgr:   sessionMgr,
			Tools:        targetToolRegistry,
			Context:      targetContextBuilder,
			Workspace:    targetWorkspace,
			MaxIteration: agentMaxIterations,
			SessionKey:   "agent:" + targetAgentID + ":call",
		})
		if err != nil {
			return "", fmt.Errorf("failed to create target agent: %w", err)
		}

		// Subscribe to target agent events
		eventChan := targetAgent.Subscribe()

		// Start target agent
		if err := targetAgent.Start(ctx); err != nil {
			return "", fmt.Errorf("failed to start target agent: %w", err)
		}
		defer targetAgent.Stop()

		// Send message to target agent
		targetInboundMsg := &bus.InboundMessage{
			ID:        uuid.New().String(),
			Channel:   "agent_call",
			SenderID:  agentID,
			ChatID:    "call",
			AgentID:   targetAgentID,
			Content:   message,
			Timestamp: time.Now(),
		}

		if err := messageBus.PublishInbound(ctx, targetInboundMsg); err != nil {
			return "", fmt.Errorf("failed to send message to target agent: %w", err)
		}

		// Wait for response
		var response strings.Builder
		for {
			select {
			case <-ctx.Done():
				return response.String(), nil
			case event, ok := <-eventChan:
				if !ok {
					return response.String(), nil
				}
				if event.Type == agent.EventMessageEnd {
					return response.String(), nil
				}
				if event.Type == agent.EventStreamContent {
					response.WriteString(event.StreamContent)
				}
			case <-time.After(60 * time.Second):
				return response.String(), fmt.Errorf("timeout waiting for target agent response")
			}
		}
	})
	toolRegistry.RegisterAgentTool(agentCallTool)

	// Create skills loader
	// 加载顺序（后加载的同名技能会覆盖前面的）：
	// 1. ./skills/ (当前目录，最高优先级)
	// 2. ${WORKSPACE}/skills/ (工作区目录)
	// 3. ~/.goclaw/skills/ (用户全局目录)
	var homeDirErr error
	homeDir, homeDirErr = os.UserHomeDir()
	if homeDirErr != nil && agentVerbose {
		fmt.Fprintf(os.Stderr, "Warning: Failed to get home directory: %v\n", homeDirErr)
		homeDir = os.Getenv("HOME")
	}
	goclawDir := homeDir + "/.goclaw"
	globalSkillsDir := goclawDir + "/skills"
	workspaceSkillsDir := workspace + "/skills"
	currentSkillsDir := "./skills"

	skillsLoader := agent.NewSkillsLoader(goclawDir, []string{
		globalSkillsDir,    // 最先加载（最低优先级）
		workspaceSkillsDir, // 其次加载
		currentSkillsDir,   // 最后加载（最高优先级）
	})
	if skillsErr := skillsLoader.Discover(); skillsErr != nil && agentVerbose {
		fmt.Fprintf(os.Stderr, "Warning: Failed to discover skills: %v\n", skillsErr)
	}

	// Create LLM provider
	provider, err := providers.NewProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create LLM provider: %v\n", err)
		os.Exit(1)
	}
	defer provider.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(agentTimeout)*time.Second)
	defer cancel()

	// Determine session key
	sessionKey := agentSessionID
	if sessionKey == "" {
		// Format: agent:<agentId>:<chatId> for ParseAgentSessionKey to work correctly
		if agentID != "" {
			sessionKey = "agent:" + agentID + ":default"
		} else {
			sessionKey = agentChannel + ":default"
		}
	}

	// Create new agent first
	agentInstance, err := agent.NewAgent(&agent.NewAgentConfig{
		Bus:          messageBus,
		Provider:     provider,
		SessionMgr:   sessionMgr,
		Tools:        toolRegistry,
		Context:      contextBuilder,
		Workspace:    workspace,
		MaxIteration: agentMaxIterations,
		SessionKey:   sessionKey,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	// Publish message to bus for processing
	inboundMsg := &bus.InboundMessage{
		Channel:   agentChannel,
		SenderID:  "cli",
		ChatID:    "default",
		AgentID:   agentID,
		Content:   agentMessage,
		Timestamp: time.Now(),
	}

	if err := messageBus.PublishInbound(ctx, inboundMsg); err != nil {
		if agentJSON {
			errorResult := map[string]interface{}{
				"error":   err.Error(),
				"success": false,
			}
			data, _ := json.MarshalIndent(errorResult, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Fprintf(os.Stderr, "Error publishing message: %v\n", err)
		}
		os.Exit(1)
	}

	// Subscribe to agent events for streaming output
	var eventChan <-chan *agent.Event
	if agentStream {
		eventChan = agentInstance.Subscribe()
		defer agentInstance.Unsubscribe(eventChan)
	}

	// Start the agent to process the message
	go func() {
		if err := agentInstance.Start(ctx); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			logger.Error("Agent error", zap.Error(err))
		}
	}()

	var response string

	// Handle streaming output
	if agentStream {
		response = handleStreamingOutput(ctx, eventChan, messageBus, agentThinking)
	} else {
		// Non-streaming: consume outbound message
		outbound, err := messageBus.ConsumeOutbound(ctx)
		if err != nil {
			if agentJSON {
				errorResult := map[string]interface{}{
					"error":   err.Error(),
					"success": false,
				}
				data, _ := json.MarshalIndent(errorResult, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Fprintf(os.Stderr, "Error consuming response: %v\n", err)
			}
			os.Exit(1)
		}
		response = outbound.Content
	}

	// Stop the agent
	if err := agentInstance.Stop(); err != nil && agentVerbose {
		fmt.Fprintf(os.Stderr, "Warning: Failed to stop agent: %v\n", err)
	}

	// Note: Messages are already saved to session by Agent.handleInboundMessage

	// Output response (for streaming, output is already done)
	if !agentStream {
		if agentJSON {
			result := map[string]interface{}{
				"response": response,
				"success":  true,
				"session":  sessionKey,
			}
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			if agentThinking != "off" {
				fmt.Println("\n💡 Response:")
			}
			fmt.Println(response)
		}
	}

	// Deliver through channel if requested
	if agentDeliver && !agentLocal {
		if err := deliverResponse(ctx, messageBus, response); err != nil && agentVerbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to deliver response: %v\n", err)
		}
	}
}

// handleStreamingOutput handles streaming output from agent events
func handleStreamingOutput(ctx context.Context, eventChan <-chan *agent.Event, messageBus *bus.MessageBus, thinkingLevel string) string {
	var fullContent, thinkingContent, finalContent strings.Builder
	inThinking := false
	inFinal := false

	for {
		select {
		case <-ctx.Done():
			fmt.Println() // Ensure newline on interrupt
			return fullContent.String()
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, output any remaining content
				if finalContent.Len() > 0 {
					fmt.Print(finalContent.String())
				}
				fmt.Println()
				return fullContent.String()
			}

			switch event.Type {
			case agent.EventStreamContent:
				content := event.StreamContent
				fullContent.WriteString(content)
				if !inThinking && !inFinal {
					fmt.Print(content)
				}

			case agent.EventStreamThinking:
				thinkingContent.WriteString(event.StreamContent)
				if thinkingLevel != "off" && !inThinking {
					inThinking = true
					fmt.Print("\n🤔 Thinking: ")
				}
				if thinkingLevel != "off" {
					fmt.Print(event.StreamContent)
				}

			case agent.EventStreamFinal:
				finalContent.WriteString(event.StreamContent)
				if !inFinal {
					inFinal = true
					if inThinking {
						fmt.Print("\n")
					}
					fmt.Print("\n📤 Final: ")
				}
				fmt.Print(event.StreamContent)

			case agent.EventStreamDone:
				// Stream complete, wait for final response
				inThinking = false
				inFinal = false

			case agent.EventToolExecutionStart:
				fmt.Printf("\n🔧 Tool: %s\n", event.ToolName)

			case agent.EventToolExecutionEnd:
				if event.ToolError {
					fmt.Printf("   ❌ Error\n")
				} else {
					fmt.Printf("   ✅ Done\n")
				}

			case agent.EventAgentEnd:
				// Agent finished, output newline and return
				fmt.Println()
				return fullContent.String()
			}
		}
	}
}

// deliverResponse delivers the response through the configured channel
func deliverResponse(ctx context.Context, messageBus *bus.MessageBus, content string) error {
	return messageBus.PublishOutbound(ctx, &bus.OutboundMessage{
		Channel:   agentChannel,
		ChatID:    "default",
		Content:   content,
		Timestamp: time.Now(),
	})
}

