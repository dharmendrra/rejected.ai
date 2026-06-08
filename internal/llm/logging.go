package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// auditLogPath is where LLM call traces are appended (JSON Lines).
const auditLogPath = "logs/llm_calls.log"

// auditLogger appends one JSON record per LLM call to auditLogPath. It is safe
// for concurrent use (the report pipeline and HTTP handlers may call in parallel).
//
// Levels:
//   - "info":  metadata only (model, sizes, duration, error) — no prompt bodies,
//     so it never writes candidate text and is PII-safe.
//   - "debug": everything in "info" plus the full system/user prompts and the
//     raw model output — useful for diagnosing JSON parse failures, but it WILL
//     contain candidate data, so use it deliberately.
type auditLogger struct {
	mu    sync.Mutex
	w     *os.File
	debug bool
}

func newAuditLogger(level string) (*auditLogger, error) {
	if err := os.MkdirAll(filepath.Dir(auditLogPath), 0o755); err != nil {
		return nil, fmt.Errorf("create llm log dir: %w", err)
	}
	f, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open llm audit log: %w", err)
	}
	return &auditLogger{w: f, debug: level == "debug"}, nil
}

// auditRecord is one JSON Lines entry. Body fields are omitted at "info" level.
type auditRecord struct {
	Time       string `json:"time"`
	Model      string `json:"model"`
	SystemLen  int    `json:"system_len"`
	UserLen    int    `json:"user_len"`
	OutputLen  int    `json:"output_len"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	System     string `json:"system,omitempty"` // debug only
	User       string `json:"user,omitempty"`   // debug only
	Output     string `json:"output,omitempty"` // debug only
}

func (l *auditLogger) write(r auditRecord) {
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.w.Write(append(b, '\n'))
}

// loggingCaller decorates a Caller, recording an audit entry around every Call.
type loggingCaller struct {
	inner  Caller
	logger *auditLogger
}

func (c *loggingCaller) ModelName() string { return c.inner.ModelName() }

func (c *loggingCaller) Call(ctx context.Context, system, user string) (string, error) {
	start := time.Now()
	out, err := c.inner.Call(ctx, system, user)

	rec := auditRecord{
		Time:       start.UTC().Format(time.RFC3339Nano),
		Model:      c.inner.ModelName(),
		SystemLen:  len(system),
		UserLen:    len(user),
		OutputLen:  len(out),
		DurationMs: time.Since(start).Milliseconds(),
	}
	if err != nil {
		rec.Error = err.Error()
	}
	if c.logger.debug {
		rec.System = system
		rec.User = user
		rec.Output = out
	}
	c.logger.write(rec)

	return out, err
}
