package hook

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// slowReader delays each read to simulate slow disk or network pipe
type slowReader struct {
	r     io.Reader
	delay time.Duration
}

func (s *slowReader) Read(p []byte) (n int, err error) {
	time.Sleep(s.delay)
	return s.r.Read(p)
}

func TestChaosSuite(t *testing.T) {
	tempData := t.TempDir()
	t.Setenv("WARDENT_DATA_DIR", tempData)

	t.Run("Random Binary Garbage Stdin", func(t *testing.T) {
		garbage := make([]byte, 64*1024)
		_, _ = rand.Read(garbage)
		var out bytes.Buffer

		code := Handle("devin", "pre-tool-use", bytes.NewReader(garbage), &out)
		if code != 0 {
			t.Errorf("expected exit code 0 on random garbage, got %d", code)
		}
		if out.Len() > 0 {
			t.Errorf("expected empty stdout on garbage, got %s", out.String())
		}
	})

	t.Run("Truncated JSON Stdin", func(t *testing.T) {
		truncated := `{"hook_event_name": "PreToolUse", "tool_name": "exec", "tool_input": {"command": "ech`
		var out bytes.Buffer

		code := Handle("devin", "pre-tool-use", strings.NewReader(truncated), &out)
		if code != 0 {
			t.Errorf("expected exit code 0 on truncated JSON, got %d", code)
		}
	})

	t.Run("Huge 5MB Payload", func(t *testing.T) {
		largeContent := strings.Repeat("A", 5*1024*1024)
		payload := `{"hook_event_name": "PreToolUse", "tool_name": "write", "tool_input": {"file_path": "huge.txt", "content": "` + largeContent + `"}, "session_id": "huge_sess"}`
		var out bytes.Buffer

		code := Handle("devin", "pre-tool-use", strings.NewReader(payload), &out)
		if code != 0 {
			t.Errorf("expected exit code 0 on huge payload, got %d", code)
		}
	})

	t.Run("Unwritable Log Directory", func(t *testing.T) {
		readOnlyDir := filepath.Join(tempData, "readonly_dir")
		if err := os.MkdirAll(readOnlyDir, 0500); err != nil {
			t.Fatal(err)
		}
		// On Windows, set ReadOnly attribute on directory or file
		blockedFile := filepath.Join(readOnlyDir, "audit.jsonl")
		_ = os.WriteFile(blockedFile, []byte("blocked"), 0400)

		t.Setenv("WARDENT_DATA_DIR", readOnlyDir)

		payload := `{"hook_event_name": "PreToolUse", "tool_name": "exec", "tool_input": {"command": "ls"}, "session_id": "s1"}`
		var out bytes.Buffer

		// Must not panic or return non-zero exit code when audit log cannot be written
		code := Handle("devin", "pre-tool-use", strings.NewReader(payload), &out)
		if code != 0 {
			t.Errorf("expected exit code 0 on unwritable log, got %d", code)
		}
	})

	t.Run("Slow Stdin Beyond Internal Deadline", func(t *testing.T) {
		payload := `{"hook_event_name": "PreToolUse", "tool_name": "exec"}`
		slowR := &slowReader{
			r:     strings.NewReader(payload),
			delay: 2 * time.Second, // exceeds InternalDeadline (1.5s)
		}
		var out bytes.Buffer

		t0 := time.Now()
		code := Handle("devin", "pre-tool-use", slowR, &out)
		elapsed := time.Since(t0)

		if code != 0 {
			t.Errorf("expected exit code 0 on slow reader timeout, got %d", code)
		}
		if elapsed > 2500*time.Millisecond {
			t.Errorf("handler took %v, should have timed out around %v", elapsed, InternalDeadline)
		}
	})
}
