package admin

import (
	"net/http"
	"os"
	"runtime"
	"time"
)

var startTime = time.Now()

// handleDashboard 仪表盘 API
func (h *AdminHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Agent 信息
	agentCount := 0
	if h.agentMgr != nil {
		agentCount = len(h.agentMgr.ListAgents())
	}

	// 会话信息
	sessionCount := 0
	if h.sessionMgr != nil {
		if sessions, err := h.sessionMgr.List(); err == nil {
			sessionCount = len(sessions)
		}
	}

	// 通道信息
	channelCount := 0
	channelList := []map[string]interface{}{}
	if h.channelMgr != nil {
		names := h.channelMgr.List()
		channelCount = len(names)
		for _, name := range names {
			status, _ := h.channelMgr.Status(name)
			channelList = append(channelList, status)
		}
	}

	// Cron 信息
	cronCount := 0
	if h.cronSvc != nil {
		cronCount = len(h.cronSvc.ListJobs())
	}

	// Swarm 信息
	swarmInfo := map[string]interface{}{
		"active": false,
	}
	if h.swarmMgr != nil {
		status := h.swarmMgr.GetStatus()
		status["active"] = true
		status["mode"] = h.swarmMgr.GetMode()
		status["agents"] = h.swarmMgr.ListAgents()
		swarmInfo = status
	}

	hostname, _ := os.Hostname()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"system": map[string]interface{}{
			"hostname":     hostname,
			"go_version":   runtime.Version(),
			"os":           runtime.GOOS,
			"arch":         runtime.GOARCH,
			"cpus":         runtime.NumCPU(),
			"goroutines":   runtime.NumGoroutine(),
			"uptime":       time.Since(startTime).String(),
			"uptime_secs":  int(time.Since(startTime).Seconds()),
			"memory_alloc": memStats.Alloc,
			"memory_sys":   memStats.Sys,
		},
		"agents":   agentCount,
		"sessions": sessionCount,
		"channels": map[string]interface{}{
			"count": channelCount,
			"list":  channelList,
		},
		"cron":  cronCount,
		"swarm": swarmInfo,
	})
}
