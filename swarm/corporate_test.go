package swarm

import (
	"os"
	"path/filepath"
	"testing"
)

// === TaskBoard 测试 ===

func TestTaskBoard_CreateTask(t *testing.T) {
	board := NewTaskBoard(t.TempDir())

	task := board.CreateTask("测试任务", "这是一个测试", "secretary", TaskPriorityNormal)
	if task == nil {
		t.Fatal("expected task to be created")
	}
	if task.ID == "" {
		t.Error("expected task ID to be set")
	}
	if task.Title != "测试任务" {
		t.Errorf("expected title '测试任务', got '%s'", task.Title)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status 'pending', got '%s'", task.Status)
	}
	if task.CreatedBy != "secretary" {
		t.Errorf("expected created_by 'secretary', got '%s'", task.CreatedBy)
	}
	if board.Count() != 1 {
		t.Errorf("expected 1 task, got %d", board.Count())
	}
}

func TestTaskBoard_CreateSubTask(t *testing.T) {
	board := NewTaskBoard(t.TempDir())
	parent := board.CreateTask("父任务", "父任务描述", "pm", TaskPriorityHigh)

	sub, err := board.CreateSubTask(parent.ID, "子任务", "子任务描述", "pm", TaskPriorityNormal)
	if err != nil {
		t.Fatal(err)
	}
	if sub.ParentID != parent.ID {
		t.Errorf("expected parent_id '%s', got '%s'", parent.ID, sub.ParentID)
	}

	subs := board.GetSubTasks(parent.ID)
	if len(subs) != 1 {
		t.Errorf("expected 1 sub task, got %d", len(subs))
	}

	// 测试不存在的父任务
	_, err = board.CreateSubTask("nonexistent", "sub", "desc", "pm", TaskPriorityNormal)
	if err == nil {
		t.Error("expected error for nonexistent parent")
	}
}

func TestTaskBoard_UpdateStatus(t *testing.T) {
	board := NewTaskBoard(t.TempDir())
	task := board.CreateTask("状态测试", "状态测试描述", "hr", TaskPriorityNormal)

	// 合法转换: pending → approval
	if err := board.UpdateStatus(task.ID, TaskStatusApproval); err != nil {
		t.Errorf("expected valid transition pending→approval: %v", err)
	}

	// 合法转换: approval → approved
	if err := board.UpdateStatus(task.ID, TaskStatusApproved); err != nil {
		t.Errorf("expected valid transition approval→approved: %v", err)
	}

	// 合法转换: approved → assigned
	if err := board.UpdateStatus(task.ID, TaskStatusAssigned); err != nil {
		t.Errorf("expected valid transition approved→assigned: %v", err)
	}

	// 合法转换: assigned → running
	if err := board.UpdateStatus(task.ID, TaskStatusRunning); err != nil {
		t.Errorf("expected valid transition assigned→running: %v", err)
	}

	// 合法转换: running → completed
	if err := board.UpdateStatus(task.ID, TaskStatusCompleted); err != nil {
		t.Errorf("expected valid transition running→completed: %v", err)
	}

	// 验证完成时间已设置
	updated, ok := board.GetTask(task.ID)
	if !ok {
		t.Fatal("expected task to exist")
	}
	if updated.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestTaskBoard_InvalidTransition(t *testing.T) {
	board := NewTaskBoard(t.TempDir())
	task := board.CreateTask("非法转换", "测试", "hr", TaskPriorityNormal)

	// pending → completed（不合法）
	err := board.UpdateStatus(task.ID, TaskStatusCompleted)
	if err == nil {
		t.Error("expected error for invalid transition pending→completed")
	}

	// pending → running（不合法）
	err = board.UpdateStatus(task.ID, TaskStatusRunning)
	if err == nil {
		t.Error("expected error for invalid transition pending→running")
	}
}

func TestTaskBoard_CancelledFromAny(t *testing.T) {
	board := NewTaskBoard(t.TempDir())

	// 测试从 pending 取消
	task1 := board.CreateTask("取消测试1", "desc", "hr", TaskPriorityNormal)
	if err := board.UpdateStatus(task1.ID, TaskStatusCancelled); err != nil {
		t.Errorf("expected valid transition pending→cancelled: %v", err)
	}

	// 测试从 running 取消
	task2 := board.CreateTask("取消测试2", "desc", "hr", TaskPriorityNormal)
	_ = board.UpdateStatus(task2.ID, TaskStatusApproval)
	_ = board.UpdateStatus(task2.ID, TaskStatusApproved)
	_ = board.UpdateStatus(task2.ID, TaskStatusAssigned)
	_ = board.UpdateStatus(task2.ID, TaskStatusRunning)
	if err := board.UpdateStatus(task2.ID, TaskStatusCancelled); err != nil {
		t.Errorf("expected valid transition running→cancelled: %v", err)
	}
}

func TestTaskBoard_ListByStatus(t *testing.T) {
	board := NewTaskBoard(t.TempDir())
	board.CreateTask("任务1", "desc", "hr", TaskPriorityNormal)
	board.CreateTask("任务2", "desc", "hr", TaskPriorityHigh)
	task3 := board.CreateTask("任务3", "desc", "hr", TaskPriorityUrgent)

	_ = board.UpdateStatus(task3.ID, TaskStatusApproval)

	pending := board.ListByStatus(TaskStatusPending)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(pending))
	}

	approval := board.ListByStatus(TaskStatusApproval)
	if len(approval) != 1 {
		t.Errorf("expected 1 approval task, got %d", len(approval))
	}
}

func TestTaskBoard_AssignAndResult(t *testing.T) {
	board := NewTaskBoard(t.TempDir())
	task := board.CreateTask("分配测试", "desc", "hr", TaskPriorityNormal)

	if err := board.AssignTask(task.ID, "worker-1"); err != nil {
		t.Fatal(err)
	}

	if err := board.SetResult(task.ID, "任务完成结果"); err != nil {
		t.Fatal(err)
	}

	got, ok := board.GetTask(task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	if got.AssignedTo != "worker-1" {
		t.Errorf("expected assigned_to 'worker-1', got '%s'", got.AssignedTo)
	}
	if got.Result != "任务完成结果" {
		t.Errorf("expected result '任务完成结果', got '%s'", got.Result)
	}
}

func TestTaskBoard_NotFound(t *testing.T) {
	board := NewTaskBoard(t.TempDir())

	_, ok := board.GetTask("nonexistent")
	if ok {
		t.Error("expected task not found")
	}

	err := board.UpdateStatus("nonexistent", TaskStatusApproval)
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

// === TaskBoard 持久化测试 ===

func TestTaskBoard_Persistence(t *testing.T) {
	dir := t.TempDir()

	// 创建并保存
	board1 := NewTaskBoard(dir)
	board1.CreateTask("持久化任务1", "desc1", "hr", TaskPriorityNormal)
	board1.CreateTask("持久化任务2", "desc2", "pm", TaskPriorityHigh)

	if err := board1.SaveToDisk(); err != nil {
		t.Fatal(err)
	}

	// 验证文件存在
	filePath := filepath.Join(dir, "corporate_tasks.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected tasks file to exist")
	}

	// 新实例加载
	board2 := NewTaskBoard(dir)
	if err := board2.LoadFromDisk(); err != nil {
		t.Fatal(err)
	}

	if board2.Count() != 2 {
		t.Errorf("expected 2 tasks after load, got %d", board2.Count())
	}

	tasks := board2.ListAll()
	titles := make(map[string]bool)
	for _, task := range tasks {
		titles[task.Title] = true
	}
	if !titles["持久化任务1"] || !titles["持久化任务2"] {
		t.Error("expected both tasks to be loaded")
	}
}

func TestTaskBoard_LoadFromDisk_NoFile(t *testing.T) {
	board := NewTaskBoard(t.TempDir())
	if err := board.LoadFromDisk(); err != nil {
		t.Errorf("expected no error when file doesn't exist: %v", err)
	}
	if board.Count() != 0 {
		t.Errorf("expected 0 tasks, got %d", board.Count())
	}
}

// === ApprovalManager 测试 ===

func TestApprovalManager_Submit(t *testing.T) {
	mgr := NewApprovalManager(10)

	req := &ApprovalRequest{
		TaskID:      "task-1",
		Title:       "测试审批",
		Description: "需要审批的方案",
		RequestedBy: "hr",
	}

	id, err := mgr.Submit(req)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("expected non-empty request ID")
	}
	if mgr.PendingCount() != 1 {
		t.Errorf("expected 1 pending, got %d", mgr.PendingCount())
	}
}

func TestApprovalManager_SubmitEmpty(t *testing.T) {
	mgr := NewApprovalManager(10)

	_, err := mgr.Submit(&ApprovalRequest{})
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestApprovalManager_TryResolve_Approve_Chinese(t *testing.T) {
	mgr := NewApprovalManager(10)

	req := &ApprovalRequest{
		TaskID:      "task-1",
		Title:       "方案A",
		Description: "测试方案",
		RequestedBy: "hr",
	}
	mgr.Submit(req)

	// 中文批准
	resolved, approved, reqID := mgr.TryResolve("批准")
	if !resolved {
		t.Error("expected to be resolved")
	}
	if !approved {
		t.Error("expected to be approved")
	}
	if reqID == "" {
		t.Error("expected non-empty request ID")
	}
	if mgr.PendingCount() != 0 {
		t.Errorf("expected 0 pending after resolve, got %d", mgr.PendingCount())
	}
}

func TestApprovalManager_TryResolve_Approve_English(t *testing.T) {
	mgr := NewApprovalManager(10)

	mgr.Submit(&ApprovalRequest{
		TaskID:      "task-2",
		Title:       "Plan B",
		RequestedBy: "hr",
	})

	resolved, approved, _ := mgr.TryResolve("yes, approve it")
	if !resolved || !approved {
		t.Error("expected approval with English keyword")
	}
}

func TestApprovalManager_TryResolve_Reject(t *testing.T) {
	mgr := NewApprovalManager(10)

	mgr.Submit(&ApprovalRequest{
		TaskID:      "task-3",
		Title:       "方案C",
		RequestedBy: "hr",
	})

	resolved, approved, _ := mgr.TryResolve("驳回这个方案")
	if !resolved {
		t.Error("expected to be resolved")
	}
	if approved {
		t.Error("expected to be rejected")
	}
}

func TestApprovalManager_TryResolve_NoMatch(t *testing.T) {
	mgr := NewApprovalManager(10)

	mgr.Submit(&ApprovalRequest{
		TaskID:      "task-4",
		Title:       "方案D",
		RequestedBy: "hr",
	})

	// 不包含审批关键词的消息
	resolved, _, _ := mgr.TryResolve("今天天气怎么样？")
	if resolved {
		t.Error("expected not to be resolved with unrelated message")
	}
}

func TestApprovalManager_TryResolve_NoPending(t *testing.T) {
	mgr := NewApprovalManager(10)

	resolved, _, _ := mgr.TryResolve("批准")
	if resolved {
		t.Error("expected not resolved when no pending requests")
	}
}

func TestApprovalManager_DirectApprove(t *testing.T) {
	mgr := NewApprovalManager(10)

	req := &ApprovalRequest{
		TaskID:      "task-5",
		Title:       "CLI审批",
		RequestedBy: "hr",
	}
	id, _ := mgr.Submit(req)

	if err := mgr.Approve(id); err != nil {
		t.Fatal(err)
	}

	if mgr.PendingCount() != 0 {
		t.Errorf("expected 0 pending after direct approve, got %d", mgr.PendingCount())
	}

	resolved := mgr.GetResolved()
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
	if resolved[0].Status != ApprovalStatusApproved {
		t.Errorf("expected approved status, got %s", resolved[0].Status)
	}
}

func TestApprovalManager_DirectReject(t *testing.T) {
	mgr := NewApprovalManager(10)

	id, _ := mgr.Submit(&ApprovalRequest{
		TaskID:      "task-6",
		Title:       "CLI驳回",
		RequestedBy: "hr",
	})

	if err := mgr.Reject(id, "方案不合理"); err != nil {
		t.Fatal(err)
	}

	resolved := mgr.GetResolved()
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
	if resolved[0].Status != ApprovalStatusRejected {
		t.Errorf("expected rejected status, got %s", resolved[0].Status)
	}
	if resolved[0].Reason != "方案不合理" {
		t.Errorf("expected reason '方案不合理', got '%s'", resolved[0].Reason)
	}
}

func TestApprovalManager_ApproveNotFound(t *testing.T) {
	mgr := NewApprovalManager(10)

	err := mgr.Approve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestApprovalManager_Callback(t *testing.T) {
	mgr := NewApprovalManager(10)

	var callbackReq *ApprovalRequest
	mgr.SetOnResolved(func(req *ApprovalRequest) {
		callbackReq = req
	})

	id, _ := mgr.Submit(&ApprovalRequest{
		TaskID:      "task-cb",
		Title:       "回调测试",
		RequestedBy: "hr",
	})

	mgr.Approve(id)

	if callbackReq == nil {
		t.Fatal("expected callback to be called")
	}
	if callbackReq.Status != ApprovalStatusApproved {
		t.Errorf("expected approved status in callback, got %s", callbackReq.Status)
	}
}

// === IsValidTransition 测试 ===

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from  TaskStatus
		to    TaskStatus
		valid bool
	}{
		{TaskStatusPending, TaskStatusApproval, true},
		{TaskStatusPending, TaskStatusCancelled, true},
		{TaskStatusPending, TaskStatusRunning, false},
		{TaskStatusPending, TaskStatusCompleted, false},
		{TaskStatusApproval, TaskStatusApproved, true},
		{TaskStatusApproval, TaskStatusRejected, true},
		{TaskStatusApproval, TaskStatusRunning, false},
		{TaskStatusApproved, TaskStatusAssigned, true},
		{TaskStatusRejected, TaskStatusApproval, true},
		{TaskStatusAssigned, TaskStatusRunning, true},
		{TaskStatusRunning, TaskStatusCompleted, true},
		{TaskStatusRunning, TaskStatusFailed, true},
		{TaskStatusRunning, TaskStatusReview, true},
		{TaskStatusReview, TaskStatusCompleted, true},
		{TaskStatusReview, TaskStatusRunning, true},
		{TaskStatusCompleted, TaskStatusPending, false},
		{TaskStatusFailed, TaskStatusPending, true},
	}

	for _, tt := range tests {
		got := IsValidTransition(tt.from, tt.to)
		if got != tt.valid {
			t.Errorf("IsValidTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.valid)
		}
	}
}

// === IDENTITY 模板测试 ===

func TestSecretaryIdentity(t *testing.T) {
	content := SecretaryIdentity("test-company", "hr-agent", "pm-agent")
	if content == "" {
		t.Error("expected non-empty identity")
	}
	if !containsAll(content, "秘书", "Secretary", "hr-agent", "pm-agent", "agent_call") {
		t.Error("identity missing required content")
	}
}

func TestHRIdentity(t *testing.T) {
	content := HRIdentity("test-company", "secretary-agent", "pm-agent")
	if content == "" {
		t.Error("expected non-empty identity")
	}
	if !containsAll(content, "HR", "人力资源", "secretary-agent", "pm-agent") {
		t.Error("identity missing required content")
	}
}

func TestPMIdentity(t *testing.T) {
	content := PMIdentity("test-company", "secretary-agent", "hr-agent")
	if content == "" {
		t.Error("expected non-empty identity")
	}
	if !containsAll(content, "PM", "项目经理", "secretary-agent", "hr-agent") {
		t.Error("identity missing required content")
	}
}

// === FormatTaskSummary 测试 ===

func TestFormatTaskSummary(t *testing.T) {
	task := &Task{
		ID:         "abc123",
		Title:      "测试任务",
		Status:     TaskStatusRunning,
		Priority:   TaskPriorityHigh,
		AssignedTo: "worker-1",
	}

	summary := FormatTaskSummary(task)
	if !containsAll(summary, "abc123", "测试任务", "running", "high", "worker-1") {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestFormatApprovalRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:          "req-1",
		Title:       "审批测试",
		Description: "需要审批",
		RequestedBy: "hr",
	}

	formatted := FormatApprovalRequest(req)
	if !containsAll(formatted, "req-1", "审批测试", "需要审批", "hr") {
		t.Errorf("unexpected format: %s", formatted)
	}
}

// === 辅助函数 ===

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// =============================================================================
// C++ → Go Worker IDENTITY 模板测试
// =============================================================================

func TestAnalystWorkerIdentity(t *testing.T) {
	content := AnalystWorkerIdentity("cpp2go-test", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	if !containsAll(content, "Analyst", "源码分析", "pm-agent", "agent_call",
		"只读操作", "risk_assessment", "C++ → Go 转换规则") {
		t.Error("analyst identity missing required content")
	}
}

func TestArchitectWorkerIdentity(t *testing.T) {
	content := ArchitectWorkerIdentity("cpp2go-test", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	if !containsAll(content, "Architect", "架构设计", "pm-agent", "agent_call",
		"namespace", "package", "interface", "C++ → Go 转换规则") {
		t.Error("architect identity missing required content")
	}
}

func TestCoderWorkerIdentity(t *testing.T) {
	content := CoderWorkerIdentity("cpp2go-test", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	if !containsAll(content, "Coder", "编码工程师", "pm-agent", "agent_call",
		"go build", "PascalCase", "error", "C++ → Go 转换规则") {
		t.Error("coder identity missing required content")
	}
}

func TestReviewerWorkerIdentity(t *testing.T) {
	content := ReviewerWorkerIdentity("cpp2go-test", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	if !containsAll(content, "Reviewer", "审查", "pm-agent", "agent_call",
		"只读操作", "blocker", "正确性") {
		t.Error("reviewer identity missing required content")
	}
}

func TestTesterWorkerIdentity(t *testing.T) {
	content := TesterWorkerIdentity("cpp2go-test", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	if !containsAll(content, "Tester", "测试工程师", "pm-agent", "agent_call",
		"Table-Driven", "Benchmark", "t.Run") {
		t.Error("tester identity missing required content")
	}
}

func TestDocWriterWorkerIdentity(t *testing.T) {
	content := DocWriterWorkerIdentity("cpp2go-test", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	if !containsAll(content, "DocWriter", "文档工程师", "pm-agent", "agent_call",
		"godoc", "README", "迁移指南") {
		t.Error("doc_writer identity missing required content")
	}
}

func TestResearcherWorkerIdentity(t *testing.T) {
	content := ResearcherWorkerIdentity("cpp2go-test", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	if !containsAll(content, "Researcher", "研究员", "pm-agent", "agent_call",
		"web_search", "Boost.Asio", "Go 替代方案") {
		t.Error("researcher identity missing required content")
	}
}

// =============================================================================
// C++ → Go 管理层定制 IDENTITY 测试
// =============================================================================

func TestSecretaryIdentityCpp2Go(t *testing.T) {
	content := SecretaryIdentityCpp2Go("cpp2go-test", "hr-agent", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	// 验证包含基础 Secretary 内容
	if !containsAll(content, "秘书", "Secretary", "hr-agent", "pm-agent") {
		t.Error("missing base secretary content")
	}
	// 验证包含 C++ → Go 定制内容
	if !containsAll(content, "Phase 0", "Phase 5", "阶段感知", "C++ → Go 重构") {
		t.Error("missing cpp2go customization content")
	}
}

func TestHRIdentityCpp2Go(t *testing.T) {
	content := HRIdentityCpp2Go("cpp2go-test", "secretary-agent", "pm-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	// 验证包含基础 HR 内容
	if !containsAll(content, "HR", "人力资源", "secretary-agent", "pm-agent") {
		t.Error("missing base HR content")
	}
	// 验证包含 C++ → Go 定制内容
	if !containsAll(content, "Worker 角色分配", "复杂度评估", "Analyst", "Coder", "opus", "sonnet") {
		t.Error("missing cpp2go customization content")
	}
}

func TestPMIdentityCpp2Go(t *testing.T) {
	content := PMIdentityCpp2Go("cpp2go-test", "secretary-agent", "hr-agent")
	if content == "" {
		t.Fatal("expected non-empty identity")
	}
	// 验证包含基础 PM 内容
	if !containsAll(content, "PM", "项目经理", "secretary-agent", "hr-agent") {
		t.Error("missing base PM content")
	}
	// 验证包含 C++ → Go 定制内容
	if !containsAll(content, "阶段依赖", "Batch 1", "Batch 4", "质量关卡", "go build") {
		t.Error("missing cpp2go customization content")
	}
}

// === LoadCorporateSwarmConfig 加载 cpp2go-refactor.json 测试 ===

func TestLoadCorporateSwarmConfig_Cpp2Go(t *testing.T) {
	cfg, err := LoadCorporateSwarmConfig(filepath.Join("..", "examples", "swarm", "cpp2go-refactor.json"))
	if err != nil {
		t.Fatalf("failed to load cpp2go config: %v", err)
	}

	if cfg.Name != "cpp2go-refactor" {
		t.Errorf("expected name 'cpp2go-refactor', got '%s'", cfg.Name)
	}
	if cfg.Mode != "corporate" {
		t.Errorf("expected mode 'corporate', got '%s'", cfg.Mode)
	}

	// 验证管理层配置
	if cfg.Secretary.AgentID != "secretary-cpp2go" {
		t.Errorf("expected secretary 'secretary-cpp2go', got '%s'", cfg.Secretary.AgentID)
	}
	if cfg.HR.AgentID != "hr-cpp2go" {
		t.Errorf("expected hr 'hr-cpp2go', got '%s'", cfg.HR.AgentID)
	}
	if cfg.PM.AgentID != "pm-cpp2go" {
		t.Errorf("expected pm 'pm-cpp2go', got '%s'", cfg.PM.AgentID)
	}

	// 验证 Worker 模板数量
	if len(cfg.WorkerPool.Templates) != 7 {
		t.Errorf("expected 7 worker templates, got %d", len(cfg.WorkerPool.Templates))
	}

	// 验证所有角色都存在
	roles := make(map[string]bool)
	for _, tmpl := range cfg.WorkerPool.Templates {
		roles[tmpl.Role] = true
	}
	expectedRoles := []string{"analyst", "architect", "coder", "reviewer", "tester", "doc_writer", "researcher"}
	for _, role := range expectedRoles {
		if !roles[role] {
			t.Errorf("missing worker template for role '%s'", role)
		}
	}

	// 验证审批和汇报配置
	if cfg.Approval.TimeoutMinutes != 30 {
		t.Errorf("expected approval timeout 30, got %d", cfg.Approval.TimeoutMinutes)
	}
	if cfg.Reporting.IntervalMinutes != 15 {
		t.Errorf("expected reporting interval 15, got %d", cfg.Reporting.IntervalMinutes)
	}
	if cfg.Reporting.DetailLevel != "detailed" {
		t.Errorf("expected reporting detail 'detailed', got '%s'", cfg.Reporting.DetailLevel)
	}
}
