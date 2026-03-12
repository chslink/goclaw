package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SharedMemoryReadTool 共享记忆读取工具
type SharedMemoryReadTool struct {
	sharedDir string
	name      string
}

// NewSharedMemoryReadTool 创建共享记忆读取工具
func NewSharedMemoryReadTool(sharedDir string) *SharedMemoryReadTool {
	return &SharedMemoryReadTool{
		sharedDir: sharedDir,
		name:      "memory_read_shared",
	}
}

// Name 返回工具名称
func (t *SharedMemoryReadTool) Name() string {
	return t.name
}

// Description 返回工具描述
func (t *SharedMemoryReadTool) Description() string {
	return "Read shared memory files in the swarm shared directory. Use action='list' to list files, action='read' to read a specific file."
}

// Parameters 返回参数定义
func (t *SharedMemoryReadTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: 'list' or 'read'",
				"enum":        []string{"list", "read"},
			},
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "File name to read (required for action='read'). Must be a .md file.",
			},
		},
		"required": []string{"action"},
	}
}

// Execute 执行工具
func (t *SharedMemoryReadTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action is required")
	}

	switch action {
	case "list":
		return t.listFiles()
	case "read":
		filename, ok := params["filename"].(string)
		if !ok || filename == "" {
			return "", fmt.Errorf("filename is required for read action")
		}
		return t.readFile(filename)
	default:
		return "", fmt.Errorf("unknown action: %s (use 'list' or 'read')", action)
	}
}

func (t *SharedMemoryReadTool) listFiles() (string, error) {
	if err := os.MkdirAll(t.sharedDir, 0755); err != nil {
		return "", fmt.Errorf("failed to ensure shared directory: %w", err)
	}

	entries, err := os.ReadDir(t.sharedDir)
	if err != nil {
		return "", fmt.Errorf("failed to list shared directory: %w", err)
	}

	if len(entries) == 0 {
		return "Shared directory is empty. No shared memory files.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Shared memory files (%d):\n", len(entries)))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		info, _ := entry.Info()
		if info != nil {
			sb.WriteString(fmt.Sprintf("  - %s (%d bytes)\n", entry.Name(), info.Size()))
		} else {
			sb.WriteString(fmt.Sprintf("  - %s\n", entry.Name()))
		}
	}

	return sb.String(), nil
}

func (t *SharedMemoryReadTool) readFile(filename string) (string, error) {
	if err := validateSharedFilename(filename); err != nil {
		return "", err
	}

	path := filepath.Join(t.sharedDir, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("File '%s' not found in shared directory.", filename), nil
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// SharedMemoryWriteTool 共享记忆写入工具
type SharedMemoryWriteTool struct {
	sharedDir string
	agentID   string
	name      string
}

// NewSharedMemoryWriteTool 创建共享记忆写入工具
func NewSharedMemoryWriteTool(sharedDir string, agentID string) *SharedMemoryWriteTool {
	return &SharedMemoryWriteTool{
		sharedDir: sharedDir,
		agentID:   agentID,
		name:      "memory_write_shared",
	}
}

// Name 返回工具名称
func (t *SharedMemoryWriteTool) Name() string {
	return t.name
}

// Description 返回工具描述
func (t *SharedMemoryWriteTool) Description() string {
	return "Write to shared memory files in the swarm shared directory. Use action='write' to create/overwrite, action='append' to append content."
}

// Parameters 返回参数定义
func (t *SharedMemoryWriteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: 'write' or 'append'",
				"enum":        []string{"write", "append"},
			},
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "File name to write to. Must be a .md file.",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write",
			},
		},
		"required": []string{"action", "filename", "content"},
	}
}

// Execute 执行工具
func (t *SharedMemoryWriteTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action is required")
	}

	filename, ok := params["filename"].(string)
	if !ok || filename == "" {
		return "", fmt.Errorf("filename is required")
	}

	content, ok := params["content"].(string)
	if !ok || content == "" {
		return "", fmt.Errorf("content is required")
	}

	if err := validateSharedFilename(filename); err != nil {
		return "", err
	}

	// Ensure shared directory exists
	if err := os.MkdirAll(t.sharedDir, 0755); err != nil {
		return "", fmt.Errorf("failed to ensure shared directory: %w", err)
	}

	path := filepath.Join(t.sharedDir, filename)

	switch action {
	case "write":
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}
		return fmt.Sprintf("File '%s' written successfully by agent '%s'.", filename, t.agentID), nil

	case "append":
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to open file for append: %w", err)
		}
		defer f.Close()

		// Add separator with agent attribution
		header := fmt.Sprintf("\n\n---\n*[%s]*\n", t.agentID)
		if _, err := f.WriteString(header + content); err != nil {
			return "", fmt.Errorf("failed to append to file: %w", err)
		}
		return fmt.Sprintf("Content appended to '%s' by agent '%s'.", filename, t.agentID), nil

	default:
		return "", fmt.Errorf("unknown action: %s (use 'write' or 'append')", action)
	}
}

// validateSharedFilename 校验共享文件名安全性
func validateSharedFilename(filename string) error {
	// 禁止路径遍历
	if strings.Contains(filename, "..") {
		return fmt.Errorf("path traversal not allowed in filename")
	}

	// 禁止绝对路径
	if filepath.IsAbs(filename) {
		return fmt.Errorf("absolute paths not allowed")
	}

	// 禁止路径分隔符
	if strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("subdirectories not allowed in filename")
	}

	// 仅允许 .md 扩展名
	if !strings.HasSuffix(filename, ".md") {
		return fmt.Errorf("only .md files are allowed")
	}

	return nil
}
