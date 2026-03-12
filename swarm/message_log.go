package swarm

import (
	"sync"
	"time"
)

// MessageDirection 消息方向
type MessageDirection string

const (
	DirectionRequest  MessageDirection = "request"  // 请求（发送给目标Agent）
	DirectionResponse MessageDirection = "response" // 响应（目标Agent回复）
)

// MessageLogEntry 一条 Agent 间的沟通记录
type MessageLogEntry struct {
	ID        string           `json:"id"`
	Timestamp time.Time        `json:"timestamp"`
	From      string           `json:"from"`
	To        string           `json:"to"`
	Direction MessageDirection `json:"direction"`
	Content   string           `json:"content"`
	FlowName  string           `json:"flow_name,omitempty"`
	SwarmName string           `json:"swarm_name,omitempty"`
}

// MessageLog 线程安全的环形缓冲区，记录 Agent 间的消息
type MessageLog struct {
	entries []MessageLogEntry
	maxSize int
	mu      sync.RWMutex
}

// NewMessageLog 创建消息日志，maxSize 为最大条数
func NewMessageLog(maxSize int) *MessageLog {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &MessageLog{
		entries: make([]MessageLogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add 添加一条日志
func (l *MessageLog) Add(entry MessageLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) >= l.maxSize {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)
}

// GetAll 获取所有日志（从旧到新）
func (l *MessageLog) GetAll() []MessageLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]MessageLogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// GetRecent 获取最近 n 条日志
func (l *MessageLog) GetRecent(n int) []MessageLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n >= len(l.entries) {
		result := make([]MessageLogEntry, len(l.entries))
		copy(result, l.entries)
		return result
	}
	start := len(l.entries) - n
	result := make([]MessageLogEntry, n)
	copy(result, l.entries[start:])
	return result
}

// Count 日志条数
func (l *MessageLog) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Clear 清空日志
func (l *MessageLog) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}
