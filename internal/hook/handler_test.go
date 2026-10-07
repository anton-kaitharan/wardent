package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleValidEvent(t *testing.T) {
	tempData := t.TempDir()
	t.Setenv("WARDENT_DATA_DIR", tempData)

	payload := `{
		"hook_event_name": "PreToolUse",
		"tool_name": "exec",
		"tool_input": {"command": "echo test"},
		"session_id": "sess_123",
		"prompt_id": "turn_1"
	}`

	var out bytes.Buffer
	code := Handle("devin", "pre-tool-use", strings.NewReader(payload), &out)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if out.Len() > 0 {
		t.Errorf("expected empty stdout for observe-only, got: %s", out.String())
	}

	// Verify audit.jsonl was written
	auditPath := filepath.Join(tempData, "audit.jsonl")
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	if !strings.Contains(string(data), "sess_123") {
		t.Errorf("audit log missing session_id: %s", string(data))
	}

	// Verify heartbeat file was written
	hbPath := filepath.Join(tempData, "heartbeat.json")
	hbData, err := os.ReadFile(hbPath)
	if err != nil {
		t.Fatalf("failed to read heartbeat: %v", err)
	}
	if !strings.Contains(string(hbData), "sess_123") {
		t.Errorf("heartbeat missing session: %s", string(hbData))
	}
}

func TestHandleFailSafeOnErrors(t *testing.T) {
	tempData := t.TempDir()
	t.Setenv("WARDENT_DATA_DIR", tempData)

	cases := []struct {
		name      string
		agent     string
		event     string
		inputData string
	}{
		{
			name:      "Empty Stdin",
			agent:     "devin",
			event:     "pre-tool-use",
			inputData: "",
		},
		{
			name:      "Malformed JSON",
			agent:     "devin",
			event:     "pre-tool-use",
			inputData: "{not valid json at all",
		},
		{
			name:      "Unknown Agent",
			agent:     "nonexistent-agent",
			event:     "pre-tool-use",
			inputData: `{"hook_event_name": "PreToolUse"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			code := Handle(tc.agent, tc.event, strings.NewReader(tc.inputData), &out)
			if code != 0 {
				t.Errorf("FAIL-SAFE BROKEN: expected code 0, got %d", code)
			}
			if out.Len() > 0 {
				t.Errorf("expected empty stdout, got: %s", out.String())
			}
		})
	}
}
