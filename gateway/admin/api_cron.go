package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/smallnest/goclaw/cron"
)

// handleListCron 列出所有 Cron 任务
func (h *AdminHandler) handleListCron(w http.ResponseWriter, r *http.Request) {
	if h.cronSvc == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"jobs":   []interface{}{},
			"count":  0,
			"status": nil,
		})
		return
	}

	jobs := h.cronSvc.ListJobs()
	status := h.cronSvc.GetStatus()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":   jobs,
		"count":  len(jobs),
		"status": status,
	})
}

// handleCreateCron 创建 Cron 任务
func (h *AdminHandler) handleCreateCron(w http.ResponseWriter, r *http.Request) {
	if h.cronSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cron service not available")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// 解析请求
	var req struct {
		Name     string `json:"name"`
		Schedule struct {
			Type           string `json:"type"`
			At             string `json:"at,omitempty"`
			Every          string `json:"every,omitempty"`
			CronExpression string `json:"cron,omitempty"`
		} `json:"schedule"`
		Payload struct {
			Type            string `json:"type"`
			Message         string `json:"message,omitempty"`
			SystemEventType string `json:"system_event_type,omitempty"`
		} `json:"payload"`
		WakeMode string `json:"wake_mode,omitempty"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	job := &cron.Job{
		Name:          req.Name,
		State:         cron.JobState{Enabled: true},
		SessionTarget: cron.SessionTargetMain,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 解析 schedule
	job.Schedule.Type = cron.ScheduleType(req.Schedule.Type)
	if req.Schedule.At != "" {
		t, err := time.Parse(time.RFC3339, req.Schedule.At)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid at time: "+err.Error())
			return
		}
		job.Schedule.At = t
	}
	if req.Schedule.Every != "" {
		dur, err := cron.ParseHumanDuration(req.Schedule.Every)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid every duration: "+err.Error())
			return
		}
		job.Schedule.EveryDuration = dur
	}
	if req.Schedule.CronExpression != "" {
		job.Schedule.CronExpression = req.Schedule.CronExpression
	}

	// 解析 payload
	job.Payload.Type = cron.PayloadType(req.Payload.Type)
	job.Payload.Message = req.Payload.Message
	job.Payload.SystemEventType = req.Payload.SystemEventType

	// 解析 wake mode
	if req.WakeMode != "" {
		job.WakeMode = cron.WakeMode(req.WakeMode)
	}

	if err := h.cronSvc.AddJob(job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add job: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

// handleDeleteCron 删除 Cron 任务
func (h *AdminHandler) handleDeleteCron(w http.ResponseWriter, r *http.Request) {
	if h.cronSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cron service not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}

	if err := h.cronSvc.RemoveJob(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove job: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "removed",
		"id":     id,
	})
}

// handleRunCron 手动运行 Cron 任务
func (h *AdminHandler) handleRunCron(w http.ResponseWriter, r *http.Request) {
	if h.cronSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cron service not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}

	if err := h.cronSvc.RunJob(context.Background(), id, true); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to run job: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "run_requested",
		"id":     id,
	})
}
