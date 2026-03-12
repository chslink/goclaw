package swarm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/gateway/admin"
)

// flatSwarmAdminAdapter 适配 SwarmManager 到 admin.SwarmManagerAPI
type flatSwarmAdminAdapter struct {
	mgr *SwarmManager
}

func (a *flatSwarmAdminAdapter) GetMode() string {
	return "flat"
}

func (a *flatSwarmAdminAdapter) GetStatus() map[string]interface{} {
	return a.mgr.GetStatus()
}

func (a *flatSwarmAdminAdapter) ListAgents() []string {
	return a.mgr.ListAgents()
}

func (a *flatSwarmAdminAdapter) GetAgentWorkspace(id string) string {
	return a.mgr.GetAgentWorkspace(id)
}

func (a *flatSwarmAdminAdapter) GetTasks() []map[string]interface{} {
	// flat 模式没有任务看板
	return []map[string]interface{}{}
}

func (a *flatSwarmAdminAdapter) GetApprovals() (pending, resolved []map[string]interface{}) {
	// flat 模式没有审批
	return []map[string]interface{}{}, []map[string]interface{}{}
}

func (a *flatSwarmAdminAdapter) ApproveRequest(id string) error {
	return fmt.Errorf("flat mode does not support approvals")
}

func (a *flatSwarmAdminAdapter) RejectRequest(id, reason string) error {
	return fmt.Errorf("flat mode does not support approvals")
}

func (a *flatSwarmAdminAdapter) GetMessages(n int) []map[string]interface{} {
	entries := a.mgr.GetMessageLog().GetRecent(n)
	return messageEntriesToMaps(entries)
}

func (a *flatSwarmAdminAdapter) AddAgent(ctx context.Context, agentID, workspace, model string) error {
	if workspace != "" || model != "" {
		return a.mgr.AddAgentWithConfig(ctx, agentID, workspace, model)
	}
	return a.mgr.AddAgent(ctx, agentID)
}

func (a *flatSwarmAdminAdapter) RemoveAgent(agentID string) error {
	return a.mgr.RemoveAgent(agentID)
}

// AdminAPI 返回 admin.SwarmManagerAPI 适配器（flat 模式）
func (m *SwarmManager) AdminAPI() admin.SwarmManagerAPI {
	return &flatSwarmAdminAdapter{mgr: m}
}

// corporateSwarmAdminAdapter 适配 CorporateSwarmManager 到 admin.SwarmManagerAPI
type corporateSwarmAdminAdapter struct {
	mgr *CorporateSwarmManager
}

func (a *corporateSwarmAdminAdapter) GetMode() string {
	return "corporate"
}

func (a *corporateSwarmAdminAdapter) GetStatus() map[string]interface{} {
	return a.mgr.GetStatus()
}

func (a *corporateSwarmAdminAdapter) ListAgents() []string {
	return a.mgr.ListAgents()
}

func (a *corporateSwarmAdminAdapter) GetAgentWorkspace(id string) string {
	return a.mgr.GetAgentWorkspace(id)
}

func (a *corporateSwarmAdminAdapter) GetTasks() []map[string]interface{} {
	board := a.mgr.GetTaskBoard()
	tasks := board.ListAll()
	result := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, taskToMap(task))
	}
	return result
}

func (a *corporateSwarmAdminAdapter) GetApprovals() (pending, resolved []map[string]interface{}) {
	approvalMgr := a.mgr.GetApproval()

	pendingReqs := approvalMgr.GetPending()
	pending = make([]map[string]interface{}, 0, len(pendingReqs))
	for _, req := range pendingReqs {
		pending = append(pending, approvalToMap(req))
	}

	resolvedReqs := approvalMgr.GetResolved()
	resolved = make([]map[string]interface{}, 0, len(resolvedReqs))
	for _, req := range resolvedReqs {
		resolved = append(resolved, approvalToMap(req))
	}

	return pending, resolved
}

func (a *corporateSwarmAdminAdapter) ApproveRequest(id string) error {
	return a.mgr.GetApproval().Approve(id)
}

func (a *corporateSwarmAdminAdapter) RejectRequest(id, reason string) error {
	return a.mgr.GetApproval().Reject(id, reason)
}

func (a *corporateSwarmAdminAdapter) GetMessages(n int) []map[string]interface{} {
	entries := a.mgr.GetMessageLog().GetRecent(n)
	return messageEntriesToMaps(entries)
}

func (a *corporateSwarmAdminAdapter) AddAgent(ctx context.Context, agentID, workspace, model string) error {
	roleCfg := config.CorporateRoleConfig{AgentID: agentID, Model: model}
	return a.mgr.AddWorker(ctx, agentID, roleCfg)
}

func (a *corporateSwarmAdminAdapter) RemoveAgent(agentID string) error {
	return a.mgr.RemoveWorker(agentID)
}

// AdminAPI 返回 admin.SwarmManagerAPI 适配器（corporate 模式）
func (m *CorporateSwarmManager) AdminAPI() admin.SwarmManagerAPI {
	return &corporateSwarmAdminAdapter{mgr: m}
}

// taskToMap 将 Task 转换为 map（用于 JSON 序列化）
func taskToMap(task *Task) map[string]interface{} {
	data, err := json.Marshal(task)
	if err != nil {
		return map[string]interface{}{"id": task.ID, "error": err.Error()}
	}
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	return m
}

// approvalToMap 将 ApprovalRequest 转换为 map
func approvalToMap(req *ApprovalRequest) map[string]interface{} {
	data, err := json.Marshal(req)
	if err != nil {
		return map[string]interface{}{"id": req.ID, "error": err.Error()}
	}
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	return m
}

// messageEntriesToMaps 将 MessageLogEntry 列表转换为 map 列表
func messageEntriesToMaps(entries []MessageLogEntry) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		var m map[string]interface{}
		_ = json.Unmarshal(data, &m)
		result = append(result, m)
	}
	return result
}
