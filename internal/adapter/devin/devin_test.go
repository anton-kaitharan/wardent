package devin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDevinAdapterContractWithFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "..", "fixtures", "devin", "3000.11.x", "windows")

	tests := []struct {
		fixtureFile  string
		expectedKind string
		check        func(t *testing.T, a *Adapter, raw []byte)
	}{
		{
			fixtureFile:  "session-start.json",
			expectedKind: "session_start",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("SessionStart", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.SessionID != "sess_test_123" {
					t.Errorf("expected session_id sess_test_123, got %s", ev.SessionID)
				}
			},
		},
		{
			fixtureFile:  "user-prompt-submit.json",
			expectedKind: "prompt_submitted",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("UserPromptSubmit", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.Prompt == nil || ev.Prompt.Text == "" {
					t.Errorf("expected prompt text, got nil or empty")
				}
			},
		},
		{
			fixtureFile:  "pre-tool-use-exec.json",
			expectedKind: "tool_pre",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("PreToolUse", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.Tool == nil || ev.Tool.Class != "shell" {
					t.Errorf("expected shell tool class, got: %+v", ev.Tool)
				}
				if ev.Tool.Shell == nil || ev.Tool.Shell.Command != "echo hello_probe_1" {
					t.Errorf("expected command echo hello_probe_1, got: %+v", ev.Tool.Shell)
				}
			},
		},
		{
			fixtureFile:  "post-tool-use-exec.json",
			expectedKind: "tool_post",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("PostToolUse", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.Result == nil || !ev.Result.OK {
					t.Errorf("expected OK result, got: %+v", ev.Result)
				}
			},
		},
		{
			fixtureFile:  "pre-tool-use-write.json",
			expectedKind: "tool_pre",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("PreToolUse", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.Tool == nil || ev.Tool.Class != "file_write" {
					t.Errorf("expected file_write tool class, got: %+v", ev.Tool)
				}
				if len(ev.Tool.Files) == 0 || ev.Tool.Files[0].Op != "create" {
					t.Errorf("expected create file op, got: %+v", ev.Tool.Files)
				}
				// Verify normalized forward slash path
				if ev.Tool.Files[0].Path != "C:/project/hello.txt" {
					t.Errorf("expected normalized forward slash path C:/project/hello.txt, got: %s", ev.Tool.Files[0].Path)
				}
			},
		},
		{
			fixtureFile:  "pre-tool-use-read.json",
			expectedKind: "tool_pre",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("PreToolUse", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.Tool == nil || ev.Tool.Class != "file_read" {
					t.Errorf("expected file_read tool class, got: %+v", ev.Tool)
				}
				if len(ev.Tool.Files) == 0 || ev.Tool.Files[0].Op != "read" {
					t.Errorf("expected read file op, got: %+v", ev.Tool.Files)
				}
			},
		},
		{
			fixtureFile:  "stop.json",
			expectedKind: "turn_stop",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("Stop", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.Stop == nil || ev.Stop.AssistantMessage == "" {
					t.Errorf("expected assistant message on stop, got: %+v", ev.Stop)
				}
			},
		},
		{
			fixtureFile:  "session-end.json",
			expectedKind: "session_end",
			check: func(t *testing.T, a *Adapter, raw []byte) {
				ev, err := a.Parse("SessionEnd", raw)
				if err != nil {
					t.Fatalf("parse failed: %v", err)
				}
				if ev.Kind != "session_end" {
					t.Errorf("expected session_end, got %s", ev.Kind)
				}
			},
		},
	}

	a := New()
	for _, tt := range tests {
		t.Run(tt.fixtureFile, func(t *testing.T) {
			path := filepath.Join(fixtureDir, tt.fixtureFile)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", path, err)
			}
			ev, err := a.Parse("", data)
			if err != nil {
				t.Fatalf("failed to parse fixture: %v", err)
			}
			if ev.Kind != tt.expectedKind {
				t.Errorf("expected kind %s, got %s", tt.expectedKind, ev.Kind)
			}
			tt.check(t, a, data)
		})
	}
}
