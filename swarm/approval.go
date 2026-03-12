package swarm

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/internal/logger"
	"go.uber.org/zap"
)

// ApprovalStatus 审批状态
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

// ApprovalRequest 审批请求
type ApprovalRequest struct {
	ID          string         `json:"id"`
	TaskID      string         `json:"task_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	RequestedBy string         `json:"requested_by"`
	Status      ApprovalStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}

// ApprovalManager 审批管理器
type ApprovalManager struct {
	mu             sync.RWMutex
	pending        map[string]*ApprovalRequest
	resolved       []*ApprovalRequest
	timeoutMinutes int
	onResolved     func(req *ApprovalRequest)
}

// NewApprovalManager 创建审批管理器
func NewApprovalManager(timeoutMinutes int) *ApprovalManager {
	if timeoutMinutes <= 0 {
		timeoutMinutes = 10
	}
	return &ApprovalManager{
		pending:        make(map[string]*ApprovalRequest),
		resolved:       make([]*ApprovalRequest, 0),
		timeoutMinutes: timeoutMinutes,
	}
}

// SetOnResolved 设置审批完成回调
func (m *ApprovalManager) SetOnResolved(fn func(req *ApprovalRequest)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onResolved = fn
}

// Submit 提交审批请求，返回 requestID
func (m *ApprovalManager) Submit(req *ApprovalRequest) (string, error) {
	if req.Title == "" {
		return "", fmt.Errorf("approval title cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ID == "" {
		req.ID = uuid.New().String()[:8]
	}
	req.Status = ApprovalStatusPending
	req.CreatedAt = time.Now()

	m.pending[req.ID] = req

	logger.Info("Approval request submitted",
		zap.String("id", req.ID),
		zap.String("title", req.Title),
		zap.String("requested_by", req.RequestedBy))

	return req.ID, nil
}

// TryResolve 尝试根据用户消息解析审批
func (m *ApprovalManager) TryResolve(userMessage string) (resolved bool, approved bool, reqID string) {
	m.mu.Lock()

	if len(m.pending) == 0 {
		m.mu.Unlock()
		return false, false, ""
	}

	// 清理过期请求
	m.cleanupExpiredLocked()

	// 判断用户意图
	isApprove := isApproveIntent(userMessage)
	isReject := isRejectIntent(userMessage)

	if !isApprove && !isReject {
		m.mu.Unlock()
		return false, false, ""
	}

	// 如果只有一个待审批，直接匹配
	var matched *ApprovalRequest
	if len(m.pending) == 1 {
		for _, req := range m.pending {
			matched = req
			break
		}
	} else {
		// 多个待审批时通过消息上下文匹配
		matched = m.matchByContext(userMessage)
	}

	if matched == nil {
		m.mu.Unlock()
		return false, false, ""
	}

	now := time.Now()
	matched.ResolvedAt = &now
	if isApprove {
		matched.Status = ApprovalStatusApproved
	} else {
		matched.Status = ApprovalStatusRejected
		matched.Reason = userMessage
	}

	reqID = matched.ID
	approved = isApprove
	callback := m.onResolved
	delete(m.pending, matched.ID)
	m.resolved = append(m.resolved, matched)
	m.mu.Unlock()

	if callback != nil {
		callback(matched)
	}

	logger.Info("Approval resolved",
		zap.String("id", reqID),
		zap.Bool("approved", approved))

	return true, approved, reqID
}

// Approve 直接批准指定请求（CLI 通道）
func (m *ApprovalManager) Approve(reqID string) error {
	return m.resolve(reqID, true, "")
}

// Reject 直接驳回指定请求（CLI 通道）
func (m *ApprovalManager) Reject(reqID, reason string) error {
	return m.resolve(reqID, false, reason)
}

func (m *ApprovalManager) resolve(reqID string, approved bool, reason string) error {
	m.mu.Lock()

	req, ok := m.pending[reqID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("approval request %s not found", reqID)
	}

	now := time.Now()
	req.ResolvedAt = &now
	req.Reason = reason
	if approved {
		req.Status = ApprovalStatusApproved
	} else {
		req.Status = ApprovalStatusRejected
	}

	callback := m.onResolved
	delete(m.pending, reqID)
	m.resolved = append(m.resolved, req)
	m.mu.Unlock()

	if callback != nil {
		callback(req)
	}

	return nil
}

// GetPending 获取所有待审批请求
func (m *ApprovalManager) GetPending() []*ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ApprovalRequest, 0, len(m.pending))
	for _, req := range m.pending {
		copied := *req
		result = append(result, &copied)
	}
	return result
}

// GetResolved 获取已完成的审批
func (m *ApprovalManager) GetResolved() []*ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ApprovalRequest, len(m.resolved))
	for i, req := range m.resolved {
		copied := *req
		result[i] = &copied
	}
	return result
}

// PendingCount 待审批数量
func (m *ApprovalManager) PendingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pending)
}

// cleanupExpiredLocked 清理过期请求（需要在持锁状态下调用）
func (m *ApprovalManager) cleanupExpiredLocked() {
	now := time.Now()
	for id, req := range m.pending {
		if now.Sub(req.CreatedAt) > time.Duration(m.timeoutMinutes)*time.Minute {
			req.Status = ApprovalStatusExpired
			expired := time.Now()
			req.ResolvedAt = &expired
			m.resolved = append(m.resolved, req)
			delete(m.pending, id)

			logger.Info("Approval request expired",
				zap.String("id", id),
				zap.String("title", req.Title))
		}
	}
}

// matchByContext 通过消息上下文匹配审批请求
func (m *ApprovalManager) matchByContext(message string) *ApprovalRequest {
	msg := strings.ToLower(message)
	for _, req := range m.pending {
		title := strings.ToLower(req.Title)
		if strings.Contains(msg, title) || strings.Contains(msg, req.ID) {
			return req
		}
	}
	return nil
}

// FormatApprovalRequest 格式化审批请求（供 IM 展示）
func FormatApprovalRequest(req *ApprovalRequest) string {
	return fmt.Sprintf("📋 审批请求 [%s]\n任务: %s\n说明: %s\n提交人: %s\n\n请回复「批准」或「驳回」",
		req.ID, req.Title, req.Description, req.RequestedBy)
}

// isApproveIntent 判断是否为批准意图
func isApproveIntent(message string) bool {
	msg := strings.ToLower(strings.TrimSpace(message))
	approveKeywords := []string{
		"批准", "同意", "通过", "可以", "好的", "没问题", "行",
		"approve", "approved", "yes", "ok", "agree", "lgtm",
	}
	for _, kw := range approveKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// isRejectIntent 判断是否为驳回意图
func isRejectIntent(message string) bool {
	msg := strings.ToLower(strings.TrimSpace(message))
	rejectKeywords := []string{
		"驳回", "拒绝", "不行", "不同意", "否决", "不可以",
		"reject", "rejected", "no", "deny", "denied", "disagree",
	}
	for _, kw := range rejectKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
