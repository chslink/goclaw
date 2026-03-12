package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusApproval  TaskStatus = "approval"
	TaskStatusApproved  TaskStatus = "approved"
	TaskStatusRejected  TaskStatus = "rejected"
	TaskStatusAssigned  TaskStatus = "assigned"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusReview    TaskStatus = "review"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityNormal TaskPriority = "normal"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityUrgent TaskPriority = "urgent"
)

// Task 任务数据结构
type Task struct {
	ID          string       `json:"id"`
	ParentID    string       `json:"parent_id,omitempty"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	CreatedBy   string       `json:"created_by"`
	AssignedTo  string       `json:"assigned_to,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Result      string       `json:"result,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// validTransitions 定义合法的状态转换
var validTransitions = map[TaskStatus][]TaskStatus{
	TaskStatusPending:   {TaskStatusApproval, TaskStatusCancelled},
	TaskStatusApproval:  {TaskStatusApproved, TaskStatusRejected, TaskStatusCancelled},
	TaskStatusApproved:  {TaskStatusAssigned, TaskStatusCancelled},
	TaskStatusRejected:  {TaskStatusApproval, TaskStatusCancelled},
	TaskStatusAssigned:  {TaskStatusRunning, TaskStatusCancelled},
	TaskStatusRunning:   {TaskStatusReview, TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled},
	TaskStatusReview:    {TaskStatusCompleted, TaskStatusRunning, TaskStatusCancelled},
	TaskStatusCompleted: {},
	TaskStatusFailed:    {TaskStatusPending, TaskStatusCancelled},
	TaskStatusCancelled: {},
}

// IsValidTransition 检查状态转换是否合法
func IsValidTransition(from, to TaskStatus) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// TaskBoard 任务看板
type TaskBoard struct {
	mu             sync.RWMutex
	tasks          map[string]*Task
	dataDir        string
	onStatusChange func(task *Task, oldStatus TaskStatus)
}

// NewTaskBoard 创建任务看板
func NewTaskBoard(dataDir string) *TaskBoard {
	return &TaskBoard{
		tasks:   make(map[string]*Task),
		dataDir: dataDir,
	}
}

// SetOnStatusChange 设置状态变更回调
func (b *TaskBoard) SetOnStatusChange(fn func(task *Task, oldStatus TaskStatus)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onStatusChange = fn
}

// CreateTask 创建任务
func (b *TaskBoard) CreateTask(title, description, createdBy string, priority TaskPriority) *Task {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	task := &Task{
		ID:          uuid.New().String()[:8],
		Title:       title,
		Description: description,
		Status:      TaskStatusPending,
		Priority:    priority,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    make(map[string]string),
	}

	b.tasks[task.ID] = task
	return task
}

// CreateSubTask 创建子任务
func (b *TaskBoard) CreateSubTask(parentID, title, description, createdBy string, priority TaskPriority) (*Task, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.tasks[parentID]; !ok {
		return nil, fmt.Errorf("parent task %s not found", parentID)
	}

	now := time.Now()
	task := &Task{
		ID:          uuid.New().String()[:8],
		ParentID:    parentID,
		Title:       title,
		Description: description,
		Status:      TaskStatusPending,
		Priority:    priority,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    make(map[string]string),
	}

	b.tasks[task.ID] = task
	return task, nil
}

// UpdateStatus 更新任务状态
func (b *TaskBoard) UpdateStatus(taskID string, newStatus TaskStatus) error {
	b.mu.Lock()

	task, ok := b.tasks[taskID]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}

	oldStatus := task.Status
	if !IsValidTransition(oldStatus, newStatus) {
		b.mu.Unlock()
		return fmt.Errorf("invalid transition from %s to %s", oldStatus, newStatus)
	}

	task.Status = newStatus
	task.UpdatedAt = time.Now()

	if newStatus == TaskStatusCompleted || newStatus == TaskStatusFailed || newStatus == TaskStatusCancelled {
		now := time.Now()
		task.CompletedAt = &now
	}

	callback := b.onStatusChange
	b.mu.Unlock()

	if callback != nil {
		callback(task, oldStatus)
	}

	return nil
}

// SetResult 设置任务结果
func (b *TaskBoard) SetResult(taskID, result string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	task, ok := b.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}

// AssignTask 分配任务
func (b *TaskBoard) AssignTask(taskID, assignee string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	task, ok := b.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.AssignedTo = assignee
	task.UpdatedAt = time.Now()
	return nil
}

// GetTask 获取任务
func (b *TaskBoard) GetTask(taskID string) (*Task, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	task, ok := b.tasks[taskID]
	if !ok {
		return nil, false
	}

	copied := *task
	return &copied, true
}

// ListByStatus 按状态列出任务
func (b *TaskBoard) ListByStatus(status TaskStatus) []*Task {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*Task
	for _, task := range b.tasks {
		if task.Status == status {
			copied := *task
			result = append(result, &copied)
		}
	}
	return result
}

// ListAll 列出所有任务
func (b *TaskBoard) ListAll() []*Task {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*Task, 0, len(b.tasks))
	for _, task := range b.tasks {
		copied := *task
		result = append(result, &copied)
	}
	return result
}

// GetSubTasks 获取子任务
func (b *TaskBoard) GetSubTasks(parentID string) []*Task {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*Task
	for _, task := range b.tasks {
		if task.ParentID == parentID {
			copied := *task
			result = append(result, &copied)
		}
	}
	return result
}

// Count 返回任务总数
func (b *TaskBoard) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.tasks)
}

// taskBoardData 持久化数据结构
type taskBoardData struct {
	Tasks []*Task `json:"tasks"`
}

// SaveToDisk 保存到磁盘
func (b *TaskBoard) SaveToDisk() error {
	b.mu.RLock()
	tasks := make([]*Task, 0, len(b.tasks))
	for _, task := range b.tasks {
		tasks = append(tasks, task)
	}
	b.mu.RUnlock()

	data := taskBoardData{Tasks: tasks}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	filePath := filepath.Join(b.dataDir, "corporate_tasks.json")
	if err := os.MkdirAll(b.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	return os.WriteFile(filePath, jsonData, 0644)
}

// LoadFromDisk 从磁盘加载
func (b *TaskBoard) LoadFromDisk() error {
	filePath := filepath.Join(b.dataDir, "corporate_tasks.json")
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read tasks file: %w", err)
	}

	var data taskBoardData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.tasks = make(map[string]*Task, len(data.Tasks))
	for _, task := range data.Tasks {
		b.tasks[task.ID] = task
	}

	return nil
}

// FormatTaskSummary 格式化任务摘要（供 IM 展示）
func FormatTaskSummary(task *Task) string {
	return fmt.Sprintf("[%s] %s (状态: %s, 优先级: %s, 负责人: %s)",
		task.ID, task.Title, task.Status, task.Priority, task.AssignedTo)
}

// FormatTaskBoard 格式化任务看板
func FormatTaskBoard(tasks []*Task) string {
	if len(tasks) == 0 {
		return "当前没有任务"
	}

	var result string
	result = fmt.Sprintf("任务看板 (共 %d 个任务):\n", len(tasks))
	for _, task := range tasks {
		result += fmt.Sprintf("  %s\n", FormatTaskSummary(task))
	}
	return result
}
