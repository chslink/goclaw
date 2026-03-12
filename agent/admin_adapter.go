package agent

import "github.com/smallnest/goclaw/gateway/admin"

// AdminAPI 返回 AgentManager 的 admin.AgentManagerAPI 适配器
func (m *AgentManager) AdminAPI() admin.AgentManagerAPI {
	return &agentManagerAdminAdapter{mgr: m}
}

// agentManagerAdminAdapter 适配 AgentManager 到 admin.AgentManagerAPI 接口
type agentManagerAdminAdapter struct {
	mgr *AgentManager
}

func (a *agentManagerAdminAdapter) ListAgents() []string {
	return a.mgr.ListAgents()
}

func (a *agentManagerAdminAdapter) GetAgentConfig(id string) *admin.AgentInfo {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	_, ok := a.mgr.agents[id]
	if !ok {
		return nil
	}

	info := &admin.AgentInfo{
		ID:     id,
		Status: "running",
	}

	// 从配置中获取更多信息
	if a.mgr.cfg != nil {
		for _, agentCfg := range a.mgr.cfg.Agents.List {
			if agentCfg.ID == id {
				info.Name = agentCfg.Name
				info.Model = agentCfg.Model
				break
			}
		}
	}

	return info
}

func (a *agentManagerAdminAdapter) GetToolsInfo() (map[string]interface{}, error) {
	return a.mgr.GetToolsInfo()
}

func (a *agentManagerAdminAdapter) GetAgentWorkspace(id string) string {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	ag, ok := a.mgr.agents[id]
	if !ok {
		return ""
	}
	return ag.workspace
}
