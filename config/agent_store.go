package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentsDir 返回 agent 配置目录路径 (~/.goclaw/agents/)
func AgentsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".goclaw", "agents"), nil
}

// LoadAgentFile 从指定路径加载单个 agent 配置
func LoadAgentFile(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.ConfigPath = path

	// 若 ID 为空，从文件名推断
	if cfg.ID == "" {
		base := filepath.Base(path)
		cfg.ID = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// 若 Name 为空，使用 ID
	if cfg.Name == "" {
		cfg.Name = cfg.ID
	}

	return &cfg, nil
}

// LoadAllAgents 加载 ~/.goclaw/agents/ 下的全部 agent 配置
func LoadAllAgents() ([]*AgentConfig, error) {
	dir, err := AgentsDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var agents []*AgentConfig
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		cfg, err := LoadAgentFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // 跳过无效文件
		}
		agents = append(agents, cfg)
	}

	return agents, nil
}

// LoadAgentByName 按名称加载 agent 配置
func LoadAgentByName(name string) (*AgentConfig, error) {
	dir, err := AgentsDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, name+".json")
	cfg, err := LoadAgentFile(path)
	if err != nil {
		return nil, fmt.Errorf("agent '%s' not found: %w", name, err)
	}

	return cfg, nil
}

// SaveAgent 保存 agent 配置到磁盘
func SaveAgent(cfg *AgentConfig) error {
	path := cfg.ConfigPath
	if path == "" {
		dir, err := AgentsDir()
		if err != nil {
			return err
		}
		name := cfg.Name
		if name == "" {
			name = cfg.ID
		}
		if name == "" {
			return fmt.Errorf("agent must have a name or id")
		}
		path = filepath.Join(dir, name+".json")
		cfg.ConfigPath = path
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", absPath, err)
	}

	return nil
}

// AgentExists 检查指定名称的 agent 配置文件是否存在
func AgentExists(name string) bool {
	dir, err := AgentsDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, name+".json"))
	return err == nil
}
