package admin

import (
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

// LogEntry 结构化日志条目
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Caller  string    `json:"caller,omitempty"`
	Fields  string    `json:"fields,omitempty"`
}

// LogRingBuffer 环形缓冲区 + 订阅者通知
type LogRingBuffer struct {
	entries []LogEntry
	size    int
	pos     int
	count   int
	mu      sync.RWMutex
	subs    map[string]chan LogEntry
	subsMu  sync.RWMutex
}

// NewLogRingBuffer 创建日志环形缓冲区
func NewLogRingBuffer(size int) *LogRingBuffer {
	return &LogRingBuffer{
		entries: make([]LogEntry, size),
		size:    size,
		subs:    make(map[string]chan LogEntry),
	}
}

// Write 写入一条日志（由 zap hook 调用）
func (b *LogRingBuffer) Write(entry LogEntry) {
	b.mu.Lock()
	b.entries[b.pos] = entry
	b.pos = (b.pos + 1) % b.size
	if b.count < b.size {
		b.count++
	}
	b.mu.Unlock()

	// 通知所有订阅者（非阻塞）
	b.subsMu.RLock()
	for _, ch := range b.subs {
		select {
		case ch <- entry:
		default:
			// 订阅者处理不过来，丢弃
		}
	}
	b.subsMu.RUnlock()
}

// Recent 获取最近 n 条日志
func (b *LogRingBuffer) Recent(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n > b.count {
		n = b.count
	}

	result := make([]LogEntry, n)
	start := (b.pos - n + b.size) % b.size
	for i := 0; i < n; i++ {
		result[i] = b.entries[(start+i)%b.size]
	}
	return result
}

// Subscribe 订阅新日志
func (b *LogRingBuffer) Subscribe(id string) chan LogEntry {
	ch := make(chan LogEntry, 100)
	b.subsMu.Lock()
	b.subs[id] = ch
	b.subsMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (b *LogRingBuffer) Unsubscribe(id string) {
	b.subsMu.Lock()
	if ch, ok := b.subs[id]; ok {
		close(ch)
		delete(b.subs, id)
	}
	b.subsMu.Unlock()
}

// LogHookCore 包装 zapcore.Core 将日志同时写入 LogRingBuffer
type LogHookCore struct {
	zapcore.Core
	buffer *LogRingBuffer
}

// NewLogHookCore 创建带 hook 的 Core
func NewLogHookCore(core zapcore.Core, buffer *LogRingBuffer) zapcore.Core {
	return &LogHookCore{
		Core:   core,
		buffer: buffer,
	}
}

// Write 覆盖写入方法
func (c *LogHookCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// 写入原始 Core
	err := c.Core.Write(entry, fields)

	// 同时写入 Ring Buffer
	logEntry := LogEntry{
		Time:    entry.Time,
		Level:   entry.Level.String(),
		Message: entry.Message,
		Caller:  entry.Caller.TrimmedPath(),
	}

	c.buffer.Write(logEntry)

	return err
}

// Check 覆盖 Check 方法
func (c *LogHookCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Core.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// With 覆盖 With 方法
func (c *LogHookCore) With(fields []zapcore.Field) zapcore.Core {
	return &LogHookCore{
		Core:   c.Core.With(fields),
		buffer: c.buffer,
	}
}
