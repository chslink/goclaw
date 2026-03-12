package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/smallnest/goclaw/acp"
	"github.com/smallnest/goclaw/agent"
	"github.com/smallnest/goclaw/agent/tools"
	"github.com/smallnest/goclaw/bus"
	"github.com/smallnest/goclaw/channels"
	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/cron"
	"github.com/smallnest/goclaw/gateway"
	"github.com/smallnest/goclaw/internal"
	"github.com/smallnest/goclaw/internal/logger"
	"github.com/smallnest/goclaw/internal/workspace"
	"github.com/smallnest/goclaw/memory"
	"github.com/smallnest/goclaw/providers"
	"github.com/smallnest/goclaw/session"
	"go.uber.org/zap"
)

// SwarmBaseServices 蜂群模式的基础服务集合
type SwarmBaseServices struct {
	Config         *config.Config
	WorkspaceDir   string
	HomeDir        string
	GoclawDir      string
	MessageBus     *bus.MessageBus
	SessionMgr     *session.Manager
	MemoryStore    *agent.MemoryStore
	ContextBuilder *agent.ContextBuilder
	ToolRegistry   *agent.ToolRegistry
	SkillsLoader   *agent.SkillsLoader
	Provider       providers.Provider
	ChannelMgr     *channels.Manager
	CronService    *cron.Service
	AcpMgr         *acp.Manager
	Gateway        *gateway.Server
	MemorySearchMgr memory.MemorySearchManager

	cancelFunc context.CancelFunc
}

// initSwarmBaseServices 初始化蜂群基础服务（与 runStart 对齐）
func initSwarmBaseServices() *SwarmBaseServices {
	// 确保内置技能
	if err := internal.EnsureBuiltinSkills(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to ensure builtin skills: %v\n", err)
	}

	// 确保配置文件存在
	if configCreated, err := internal.EnsureConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to ensure config: %v\n", err)
	} else if configCreated {
		fmt.Println("Config file created at: " + internal.GetConfigPath())
	}

	// 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志（如果尚未初始化，使用 swarm verbose 标志）
	logLvl := "info"
	if swarmVerbose {
		logLvl = "debug"
	}
	if err := logger.Init(logLvl, false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// 验证配置
	if err := config.Validate(cfg); err != nil {
		logger.Fatal("Invalid configuration", zap.Error(err))
	}

	// 获取 workspace 目录
	workspaceDir, err := config.GetWorkspacePath(cfg)
	if err != nil {
		logger.Fatal("Failed to get workspace path", zap.Error(err))
	}

	// 创建 workspace 管理器
	workspaceMgr := workspace.NewManager(workspaceDir)
	if err := workspaceMgr.Ensure(); err != nil {
		logger.Warn("Failed to ensure workspace files", zap.Error(err))
	}

	// 获取 home 目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Fatal("Failed to get home directory", zap.Error(err))
	}
	goclawDir := homeDir + "/.goclaw"

	// 创建消息总线
	messageBus := bus.NewMessageBus(100)

	// 创建会话管理器
	sessionDir := homeDir + "/.goclaw/sessions"
	sessionMgr, err := session.NewManager(sessionDir)
	if err != nil {
		logger.Fatal("Failed to create session manager", zap.Error(err))
	}

	// 创建记忆存储 + 上下文构建器
	memoryStore := agent.NewMemoryStore(workspaceDir)
	contextBuilder := agent.NewContextBuilder(memoryStore, workspaceDir)

	// 创建工具注册表
	toolRegistry := agent.NewToolRegistry()

	// 技能加载器
	globalSkillsDir := goclawDir + "/skills"
	workspaceSkillsDir := workspaceDir + "/skills"
	currentSkillsDir := "./skills"

	skillsLoader := agent.NewSkillsLoader(goclawDir, []string{
		globalSkillsDir,
		workspaceSkillsDir,
		currentSkillsDir,
	})
	if err := skillsLoader.Discover(); err != nil {
		logger.Warn("Failed to discover skills", zap.Error(err))
	} else if skills := skillsLoader.List(); len(skills) > 0 {
		logger.Info("Skills loaded", zap.Int("count", len(skills)))
	}

	// 注册文件系统工具
	fsTool := tools.NewFileSystemTool(cfg.Tools.FileSystem.AllowedPaths, cfg.Tools.FileSystem.DeniedPaths, workspaceDir)
	for _, tool := range fsTool.GetTools() {
		if err := toolRegistry.RegisterExisting(tool); err != nil {
			logger.Warn("Failed to register tool", zap.String("tool", tool.Name()))
		}
	}

	// 注册 use_skill 工具
	if err := toolRegistry.RegisterExisting(tools.NewUseSkillTool()); err != nil {
		logger.Warn("Failed to register use_skill tool", zap.Error(err))
	}

	// 注册 Shell 工具
	shellTool := tools.NewShellTool(
		cfg.Tools.Shell.Enabled,
		cfg.Tools.Shell.AllowedCmds,
		cfg.Tools.Shell.DeniedCmds,
		cfg.Tools.Shell.Timeout,
		cfg.Tools.Shell.WorkingDir,
		cfg.Tools.Shell.Sandbox,
	)
	for _, tool := range shellTool.GetTools() {
		if err := toolRegistry.RegisterExisting(tool); err != nil {
			logger.Warn("Failed to register tool", zap.String("tool", tool.Name()))
		}
	}

	// 注册 Web 工具
	webTool := tools.NewWebTool(
		cfg.Tools.Web.SearchAPIKey,
		cfg.Tools.Web.SearchEngine,
		cfg.Tools.Web.Timeout,
	)
	for _, tool := range webTool.GetTools() {
		if err := toolRegistry.RegisterExisting(tool); err != nil {
			logger.Warn("Failed to register tool", zap.String("tool", tool.Name()))
		}
	}

	// 注册浏览器工具
	if cfg.Tools.Browser.Enabled {
		browserTool := tools.NewBrowserTool(
			cfg.Tools.Browser.Headless,
			cfg.Tools.Browser.Timeout,
		)
		for _, tool := range browserTool.GetTools() {
			if err := toolRegistry.RegisterExisting(tool); err != nil {
				logger.Warn("Failed to register tool", zap.String("tool", tool.Name()))
			}
		}
		logger.Info("Browser tools registered")
	}

	// 创建 LLM 提供商
	provider, err := providers.NewProvider(cfg)
	if err != nil {
		logger.Fatal("Failed to create LLM provider", zap.Error(err))
	}

	// 创建通道管理器
	channelMgr := channels.NewManager(messageBus)
	if err := channelMgr.SetupFromConfig(cfg); err != nil {
		logger.Warn("Failed to setup channels from config", zap.Error(err))
	}

	// 创建 Cron 服务
	cronService, err := cron.NewService(cron.DefaultCronConfig(), messageBus)
	if err != nil {
		logger.Warn("Failed to create cron service", zap.Error(err))
	}

	// 注册 Cron 工具
	if cfg.Tools.Cron.Enabled && cronService != nil {
		cronTool := tools.NewCronTool(cronService)
		for _, tool := range cronTool.GetTools() {
			if err := toolRegistry.RegisterExisting(tool); err != nil {
				logger.Warn("Failed to register tool", zap.String("tool", tool.Name()), zap.Error(err))
			}
		}
		logger.Info("Cron tools registered")
	}

	// 创建 ACP 管理器（如果启用）
	var acpMgr *acp.Manager
	if cfg.ACP.Enabled {
		acpMgr = acp.GetOrCreateGlobalManager(cfg)
		toolRegistry.RegisterAgentTool(agent.NewSpawnAcpTool(cfg, acpMgr))

		threadBindingService := channels.NewThreadBindingService(cfg, sessionMgr)
		channelMgr.SetThreadBindingService(threadBindingService)

		acpRouter := acp.NewAcpSessionRouter(acpMgr)
		acpRouter.SetThreadBindingService(threadBindingService)
		channelMgr.SetAcpRouter(acpRouter)
		acp.SetGlobalThreadBindingService(threadBindingService)
	}

	// 创建网关服务器
	gatewayServer := gateway.NewServer(cfg, messageBus, channelMgr, sessionMgr, cronService, acpMgr)

	// 创建记忆搜索管理器
	var memorySearchMgr memory.MemorySearchManager
	memSearchMgr, err := memory.GetMemorySearchManager(cfg.Memory, workspaceDir)
	if err != nil {
		logger.Warn("Failed to create memory search manager, memory tools will be unavailable", zap.Error(err))
	} else {
		memorySearchMgr = memSearchMgr
		logger.Info("Memory search manager created", zap.String("backend", cfg.Memory.Backend))
	}

	return &SwarmBaseServices{
		Config:         cfg,
		WorkspaceDir:   workspaceDir,
		HomeDir:        homeDir,
		GoclawDir:      goclawDir,
		MessageBus:     messageBus,
		SessionMgr:     sessionMgr,
		MemoryStore:    memoryStore,
		ContextBuilder: contextBuilder,
		ToolRegistry:   toolRegistry,
		SkillsLoader:   skillsLoader,
		Provider:       provider,
		ChannelMgr:     channelMgr,
		CronService:    cronService,
		AcpMgr:         acpMgr,
		Gateway:        gatewayServer,
		MemorySearchMgr: memorySearchMgr,
	}
}

// Start 启动所有基础服务
func (s *SwarmBaseServices) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	// 启动 Cron 服务
	if s.CronService != nil {
		if err := s.CronService.Start(ctx); err != nil {
			logger.Warn("Failed to start cron service", zap.Error(err))
		}
	}

	// 启动 ACP 线程清理
	if s.AcpMgr != nil && s.Config.ACP.Enabled {
		threadBindingService := channels.NewThreadBindingService(s.Config, s.SessionMgr)
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if expired := threadBindingService.CleanupExpired(); expired > 0 {
						logger.Info("Cleaned up expired ACP thread bindings", zap.Int("count", expired))
					}
				}
			}
		}()
	}

	// 启动网关
	if err := s.Gateway.Start(ctx); err != nil {
		logger.Warn("Failed to start gateway server", zap.Error(err))
	}

	// 启动 IM 通道
	if err := s.ChannelMgr.Start(ctx); err != nil {
		logger.Error("Failed to start channels", zap.Error(err))
	}

	// 启动出站消息分发
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Outbound message dispatcher panicked", zap.Any("panic", r))
			}
		}()
		if err := s.ChannelMgr.DispatchOutbound(ctx); err != nil {
			logger.Error("Outbound message dispatcher exited with error", zap.Error(err))
		}
	}()

	logger.Info("Swarm base services started",
		zap.Int("channels", len(s.ChannelMgr.List())),
		zap.Int("tools", s.ToolRegistry.Count()))

	return nil
}

// Shutdown 关闭所有基础服务
func (s *SwarmBaseServices) Shutdown() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	if s.Gateway != nil {
		_ = s.Gateway.Stop()
	}
	if s.CronService != nil {
		_ = s.CronService.Stop()
	}
	if s.ChannelMgr != nil {
		_ = s.ChannelMgr.Stop()
	}
	if s.Provider != nil {
		s.Provider.Close()
	}
	if s.MemorySearchMgr != nil {
		_ = s.MemorySearchMgr.Close()
	}
	if s.MessageBus != nil {
		s.MessageBus.Close()
	}
	_ = logger.Sync()
}
