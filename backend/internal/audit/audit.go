package audit

import (
	"sync"
	"time"
)

type Entry struct {
	Time       time.Time `json:"time"`
	Actor      string    `json:"actor"`
	Role       string    `json:"role,omitempty"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id,omitempty"`
	Result     string    `json:"result"`
	Details    string    `json:"details,omitempty"`
}

type Logger struct {
	mu      sync.RWMutex
	entries []Entry
}

func NewLogger() *Logger {
	return &Logger{entries: make([]Entry, 0, 32)}
}

func (l *Logger) Record(entry Entry) {
	if l == nil {
		return
	}
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *Logger) List() []Entry {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}
