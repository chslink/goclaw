package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/smallnest/goclaw/config"
	workspace2 "github.com/smallnest/goclaw/internal/workspace"
	"github.com/spf13/cobra"
)

// agentWorkspace 获取 agent 的工作区路径（内部便捷函数）
func agentWorkspace(name string) (string, error) {
	cfg, err := config.LoadAgentByName(name)
	if err != nil {
		return "", err
	}
	return cfg.Workspace, nil
}

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage isolated agents",
	Long:  `Manage isolated agents with their own workspaces and configurations.`,
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	Run:   runAgentsList,
}

var agentsAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new agent",
	Args:  cobra.MaximumNArgs(1),
	Run:   runAgentsAdd,
}

var agentsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an agent",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentsDelete,
}

var agentsIdentityCmd = &cobra.Command{
	Use:   "identity <name>",
	Short: "Edit agent's IDENTITY.md file",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentsIdentity,
}

var agentsSoulCmd = &cobra.Command{
	Use:   "soul <name>",
	Short: "Edit agent's SOUL.md file",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentsSoul,
}

var agentsUserCmd = &cobra.Command{
	Use:   "user <name>",
	Short: "Edit agent's USER.md file",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentsUser,
}

var agentsToolsCmd = &cobra.Command{
	Use:   "tools <name>",
	Short: "Edit agent's TOOLS.md file",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentsTools,
}

var agentsMemoryCmd = &cobra.Command{
	Use:   "memory <name>",
	Short: "Edit agent's MEMORY.md file",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentsMemory,
}

var agentsBootstrapCmd = &cobra.Command{
	Use:   "bootstrap <name>",
	Short: "Initialize agent workspace with template files",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentsBootstrap,
}

func runAgentsIdentity(cmd *cobra.Command, args []string) {
	runAgentsEditFile("IDENTITY.md", args[0])
}

func runAgentsSoul(cmd *cobra.Command, args []string) {
	runAgentsEditFile("SOUL.md", args[0])
}

func runAgentsUser(cmd *cobra.Command, args []string) {
	runAgentsEditFile("USER.md", args[0])
}

func runAgentsTools(cmd *cobra.Command, args []string) {
	runAgentsEditFile("TOOLS.md", args[0])
}

func runAgentsMemory(cmd *cobra.Command, args []string) {
	runAgentsEditFile("MEMORY.md", args[0])
}

// Flags for agents list
var (
	agentsListJSON     bool
	agentsListBindings bool
)

// Flags for agents add
var (
	agentsAddWorkspace      string
	agentsAddModel          string
	agentsAddAgentDir       string
	agentsAddBind           []string
	agentsAddNonInteractive bool
	agentsAddJSON           bool
)

// Flags for agents delete
var (
	agentsDeleteForce bool
	agentsDeleteJSON  bool
)

func init() {
	// List flags
	agentsListCmd.Flags().BoolVar(&agentsListJSON, "json", false, "Output in JSON format")
	agentsListCmd.Flags().BoolVar(&agentsListBindings, "bindings", false, "Show channel bindings")

	// Add flags
	agentsAddCmd.Flags().StringVar(&agentsAddWorkspace, "workspace", "", "Workspace directory for the agent")
	agentsAddCmd.Flags().StringVar(&agentsAddModel, "model", "", "Model to use for the agent")
	agentsAddCmd.Flags().StringVar(&agentsAddAgentDir, "agent-dir", "", "Directory containing agent definitions")
	agentsAddCmd.Flags().StringSliceVar(&agentsAddBind, "bind", []string{}, "Bind agent to channels (e.g., telegram:123456)")
	agentsAddCmd.Flags().BoolVar(&agentsAddNonInteractive, "non-interactive", false, "Run in non-interactive mode")
	agentsAddCmd.Flags().BoolVar(&agentsAddJSON, "json", false, "Output in JSON format")

	// Delete flags
	agentsDeleteCmd.Flags().BoolVar(&agentsDeleteForce, "force", false, "Force deletion without confirmation")
	agentsDeleteCmd.Flags().BoolVar(&agentsDeleteJSON, "json", false, "Output in JSON format")

	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsAddCmd)
	agentsCmd.AddCommand(agentsDeleteCmd)
	agentsCmd.AddCommand(agentsIdentityCmd)
	agentsCmd.AddCommand(agentsSoulCmd)
	agentsCmd.AddCommand(agentsUserCmd)
	agentsCmd.AddCommand(agentsToolsCmd)
	agentsCmd.AddCommand(agentsMemoryCmd)
	agentsCmd.AddCommand(agentsBootstrapCmd)
}

// runAgentsList lists all configured agents
func runAgentsList(cmd *cobra.Command, args []string) {
	agents, err := config.LoadAllAgents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading agents: %v\n", err)
		os.Exit(1)
	}

	if agentsListJSON {
		data, err := json.MarshalIndent(agents, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	if len(agents) == 0 {
		fmt.Println("No agents found.")
		fmt.Println("\nCreate a new agent with: goclaw agents add [name]")
		return
	}

	// Sort agents by name
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})

	// Display in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tWORKSPACE\tMODEL\tBINDINGS\n")
	fmt.Fprintf(w, "----\t---------\t-----\t--------\n")
	for _, agent := range agents {
		bindings := ""
		if agentsListBindings && len(agent.Bindings) > 0 {
			bindings = fmt.Sprintf("%v", agent.Bindings)
		} else if len(agent.Bindings) > 0 {
			bindings = fmt.Sprintf("[%d]", len(agent.Bindings))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", agent.Name, agent.Workspace, agent.Model, bindings)
	}
	w.Flush()
}

// runAgentsAdd adds a new agent
func runAgentsAdd(cmd *cobra.Command, args []string) {
	var name string

	if len(args) > 0 {
		name = args[0]
	} else if !agentsAddNonInteractive {
		// Prompt for name if not provided and in interactive mode
		fmt.Print("Enter agent name: ")
		_, _ = fmt.Scanln(&name)
	}

	if name == "" {
		fmt.Fprintf(os.Stderr, "Error: Agent name is required\n")
		os.Exit(1)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	// Check if agent already exists
	if config.AgentExists(name) {
		fmt.Fprintf(os.Stderr, "Error: Agent '%s' already exists\n", name)
		os.Exit(1)
	}

	// Load default configuration
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load config: %v\n", err)
		cfg = &config.Config{}
	}

	// Set defaults
	workspace := agentsAddWorkspace
	if workspace == "" {
		workspace = filepath.Join(homeDir, ".goclaw", "workspaces", name)
	}

	model := agentsAddModel
	if model == "" {
		model = cfg.Agents.Defaults.Model
		if model == "" {
			model = "gpt-4"
		}
	}

	agentsDir, err := config.AgentsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting agents directory: %v\n", err)
		os.Exit(1)
	}
	agentConfigPath := filepath.Join(agentsDir, name+".json")

	// Create agent config
	agentCfg := &config.AgentConfig{
		ID:         name,
		Name:       name,
		Workspace:  workspace,
		Model:      model,
		AgentDir:   agentsAddAgentDir,
		Bindings:   agentsAddBind,
		ConfigPath: agentConfigPath,
		Metadata:   make(map[string]interface{}),
	}

	// Save agent configuration
	if err := config.SaveAgent(agentCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving agent: %v (path: %s)\n", err, agentCfg.ConfigPath)
		os.Exit(1)
	}

	// Create and initialize workspace directory with bootstrap files
	wsMgr := workspace2.NewManager(workspace)
	if err := wsMgr.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize workspace: %v\n", err)
	}

	if agentsAddJSON {
		data, err := json.MarshalIndent(agentCfg, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("Agent '%s' created successfully\n", name)
		fmt.Printf("  Workspace: %s\n", workspace)
		fmt.Printf("  Model: %s\n", model)
		if len(agentsAddBind) > 0 {
			fmt.Printf("  Bindings: %v\n", agentsAddBind)
		}
		fmt.Printf("\nWorkspace initialized with bootstrap files.\n")
		fmt.Printf("Customize your agent:\n")
		fmt.Printf("  goclaw agents identity %s\n", name)
		fmt.Printf("  goclaw agents soul %s\n", name)
	}
}

// runAgentsDelete deletes an agent
func runAgentsDelete(cmd *cobra.Command, args []string) {
	name := args[0]

	agentsDir, err := config.AgentsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting agents directory: %v\n", err)
		os.Exit(1)
	}

	agentConfigPath := filepath.Join(agentsDir, name+".json")

	// Check if agent exists
	if _, err := os.Stat(agentConfigPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Agent '%s' not found\n", name)
		os.Exit(1)
	}

	// Confirm deletion unless force flag is set
	if !agentsDeleteForce && !agentsDeleteJSON {
		fmt.Printf("Are you sure you want to delete agent '%s'? [y/N]: ", name)
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Deletion cancelled")
			return
		}
	}

	// Delete agent configuration
	if err := os.Remove(agentConfigPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting agent: %v\n", err)
		os.Exit(1)
	}

	if agentsDeleteJSON {
		result := map[string]interface{}{
			"status":  "deleted",
			"name":    name,
			"success": true,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("Agent '%s' deleted successfully\n", name)
		fmt.Println("Note: Workspace files are preserved. To remove them, delete the workspace directory manually.")
	}
}

// runAgentsEditFile opens an editor for a specific file in the agent's workspace
func runAgentsEditFile(filename, name string) {
	workspacePath, err := agentWorkspace(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	filePath := filepath.Join(workspacePath, filename)

	// Ensure the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create empty file
		if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
			os.Exit(1)
		}
	}

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Default editors based on platform
		if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
			editor = "notepad"
		} else {
			editor = "nano"
		}
	}

	// Open editor
	editorCmd := exec.Command(editor, filePath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated %s for agent '%s'\n", filename, name)
}

// runAgentsBootstrap initializes the agent workspace with template files
func runAgentsBootstrap(cmd *cobra.Command, args []string) {
	name := args[0]

	workspacePath, err := agentWorkspace(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create workspace directory if it doesn't exist
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating workspace: %v\n", err)
		os.Exit(1)
	}

	// Initialize workspace with template files
	wsMgr := workspace2.NewManager(workspacePath)
	if err := wsMgr.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing workspace: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initialized workspace for agent '%s'\n", name)
	fmt.Printf("  Workspace: %s\n", workspacePath)
	fmt.Println("\nCreated files:")
	for _, f := range workspace2.BootstrapFiles {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Println("\nEdit these files to customize your agent:")
	fmt.Printf("  goclaw agents identity %s\n", name)
	fmt.Printf("  goclaw agents soul %s\n", name)
	fmt.Printf("  goclaw agents user %s\n", name)
}
