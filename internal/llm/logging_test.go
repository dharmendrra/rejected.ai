package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCaller struct{ out string }

func (f *fakeCaller) Call(ctx context.Context, system, user string) (string, error) { return f.out, nil }
func (f *fakeCaller) ModelName() string                                             { return "fake-model" }

func newTestLogger(t *testing.T, debug bool) (*auditLogger, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "llm.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open temp log: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return &auditLogger{w: f, debug: debug}, path
}

func readRecord(t *testing.T, path string) auditRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		t.Fatal("log is empty")
	}
	var rec auditRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("unmarshal record %q: %v", line, err)
	}
	return rec
}

func TestLoggingCaller_InfoOmitsBodies(t *testing.T) {
	logger, path := newTestLogger(t, false)
	c := &loggingCaller{inner: &fakeCaller{out: "the-output"}, logger: logger}

	if _, err := c.Call(context.Background(), "sys-prompt", "user-prompt"); err != nil {
		t.Fatalf("Call: %v", err)
	}

	rec := readRecord(t, path)
	if rec.Model != "fake-model" {
		t.Errorf("model = %q, want fake-model", rec.Model)
	}
	if rec.SystemLen != len("sys-prompt") || rec.UserLen != len("user-prompt") || rec.OutputLen != len("the-output") {
		t.Errorf("lengths wrong: %+v", rec)
	}
	// info level must NOT leak prompt/response bodies.
	if rec.System != "" || rec.User != "" || rec.Output != "" {
		t.Errorf("info level leaked bodies: %+v", rec)
	}
}

func TestLoggingCaller_DebugIncludesBodies(t *testing.T) {
	logger, path := newTestLogger(t, true)
	c := &loggingCaller{inner: &fakeCaller{out: "the-output"}, logger: logger}

	if _, err := c.Call(context.Background(), "sys-prompt", "user-prompt"); err != nil {
		t.Fatalf("Call: %v", err)
	}

	rec := readRecord(t, path)
	if rec.System != "sys-prompt" || rec.User != "user-prompt" || rec.Output != "the-output" {
		t.Errorf("debug level should include bodies: %+v", rec)
	}
}
