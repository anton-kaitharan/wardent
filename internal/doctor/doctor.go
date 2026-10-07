package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anton-kaitharan/wardent/internal/audit"
	"github.com/anton-kaitharan/wardent/internal/hook"
	"github.com/anton-kaitharan/wardent/internal/install"
)

type CheckResult struct {
	Name    string
	Status  string // "OK" | "WARN" | "FAIL"
	Details string
}

func RunDiagnostics(w io.Writer) bool {
	fmt.Fprintln(w, "Wardent Doctor — Diagnostics & System Health")
	fmt.Fprintln(w, strings.Repeat("=", 60))

	var results []CheckResult
	allOK := true

	// 1. Binary on PATH
	results = append(results, checkBinaryPath())

	// 2. Project Hooks Installation
	results = append(results, checkProjectHooks())

	// 3. User Config Hooks Installation
	results = append(results, checkUserConfigHooks())

	// 4. Live Round-Trip Test
	results = append(results, checkLiveRoundTrip())

	// 5. Heartbeat & Event Ingestion
	results = append(results, checkHeartbeat())

	// 6. Security & Override Checks
	results = append(results, checkProjectOverrides())

	// Print summary
	for _, res := range results {
		icon := "[✓]"
		if res.Status == "WARN" {
			icon = "[!]"
		} else if res.Status == "FAIL" {
			icon = "[✗]"
			allOK = false
		}
		fmt.Fprintf(w, "%-4s %-25s : %s\n", icon, res.Name, res.Details)
	}

	fmt.Fprintln(w, strings.Repeat("-", 60))
	if allOK {
		fmt.Fprintln(w, "Result: System is healthy and ready to observe AI agent actions.")
	} else {
		fmt.Fprintln(w, "Result: Issues detected. Please review warnings above.")
	}

	return allOK
}

func checkBinaryPath() CheckResult {
	path, err := exec.LookPath("wardent")
	if err != nil {
		path, err = exec.LookPath("wardent.exe")
	}
	if err != nil {
		exePath, _ := os.Executable()
		return CheckResult{
			Name:    "CLI on PATH",
			Status:  "WARN",
			Details: fmt.Sprintf("Executable not found on system PATH (current exe: %s). Add to PATH for GUI-launched agents.", exePath),
		}
	}
	return CheckResult{
		Name:    "CLI on PATH",
		Status:  "OK",
		Details: fmt.Sprintf("Found at %s", path),
	}
}

func checkProjectHooks() CheckResult {
	cwd, _ := os.Getwd()
	target := install.GetProjectHooksPath(cwd)
	data, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		return CheckResult{
			Name:    "Project Hooks",
			Status:  "WARN",
			Details: fmt.Sprintf("Not installed at %s (run 'wardent install')", target),
		}
	}
	if err != nil {
		return CheckResult{
			Name:    "Project Hooks",
			Status:  "FAIL",
			Details: fmt.Sprintf("Cannot read %s: %v", target, err),
		}
	}

	var m install.HooksMap
	if err := json.Unmarshal(data, &m); err != nil {
		return CheckResult{
			Name:    "Project Hooks",
			Status:  "FAIL",
			Details: fmt.Sprintf("Corrupt JSON in %s: %v", target, err),
		}
	}

	hasWardent := false
	for _, groups := range m {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if strings.Contains(h.Command, "wardent hook devin") {
					hasWardent = true
					break
				}
			}
		}
	}

	if !hasWardent {
		return CheckResult{
			Name:    "Project Hooks",
			Status:  "WARN",
			Details: "File exists but contains no active Wardent hook entries",
		}
	}

	return CheckResult{
		Name:    "Project Hooks",
		Status:  "OK",
		Details: fmt.Sprintf("Installed and valid (%s)", target),
	}
}

func checkUserConfigHooks() CheckResult {
	target := install.GetUserConfigPath()
	data, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		return CheckResult{
			Name:    "User Global Config",
			Status:  "OK",
			Details: "Not configured (project-level hooks active)",
		}
	}
	if err != nil {
		return CheckResult{
			Name:    "User Global Config",
			Status:  "WARN",
			Details: fmt.Sprintf("Unable to read %s: %v", target, err),
		}
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return CheckResult{
			Name:    "User Global Config",
			Status:  "WARN",
			Details: fmt.Sprintf("Non-JSON user config at %s", target),
		}
	}

	if rawHooks, ok := cfg["hooks"]; ok {
		hooksBytes, _ := json.Marshal(rawHooks)
		if strings.Contains(string(hooksBytes), "wardent hook devin") {
			return CheckResult{
				Name:    "User Global Config",
				Status:  "OK",
				Details: fmt.Sprintf("Wardent hooks installed globally (%s)", target),
			}
		}
	}

	return CheckResult{
		Name:    "User Global Config",
		Status:  "OK",
		Details: "Valid (no global hooks configured)",
	}
}

func checkLiveRoundTrip() CheckResult {
	payload := `{
		"hook_event_name": "PreToolUse",
		"tool_name": "exec",
		"tool_input": {"command": "echo __wardent_doctor_test__"},
		"session_id": "doctor_probe_session",
		"prompt_id": "turn_0"
	}`

	var out strings.Builder
	code := hook.Handle("devin", "pre-tool-use", strings.NewReader(payload), &out)
	if code != 0 {
		return CheckResult{
			Name:    "Live Hook Round-Trip",
			Status:  "FAIL",
			Details: fmt.Sprintf("Expected exit code 0, got %d", code),
		}
	}

	return CheckResult{
		Name:    "Live Hook Round-Trip",
		Status:  "OK",
		Details: "In-memory hook evaluation and audit append succeeded",
	}
}

func checkHeartbeat() CheckResult {
	hbPath := filepath.Join(audit.DefaultLogDir(), "heartbeat.json")
	data, err := os.ReadFile(hbPath)
	if os.IsNotExist(err) {
		return CheckResult{
			Name:    "Agent Heartbeat",
			Status:  "WARN",
			Details: "No agent events recorded yet. Start a session to observe agent activity.",
		}
	}
	if err != nil {
		return CheckResult{
			Name:    "Agent Heartbeat",
			Status:  "WARN",
			Details: fmt.Sprintf("Cannot read heartbeat file: %v", err),
		}
	}

	var hb hook.Heartbeat
	if err := json.Unmarshal(data, &hb); err != nil {
		return CheckResult{
			Name:    "Agent Heartbeat",
			Status:  "WARN",
			Details: "Corrupt heartbeat record",
		}
	}

	return CheckResult{
		Name:    "Agent Heartbeat",
		Status:  "OK",
		Details: fmt.Sprintf("Active (last event: %s at %s, total events: %d)", hb.LastEventName, hb.LastEventTS, hb.TotalEvents),
	}
}

func checkProjectOverrides() CheckResult {
	cwd, _ := os.Getwd()
	// Warn if .claude/ directory exists (Devin reads .claude/ hooks by default, which could interfere)
	claudeDir := filepath.Join(cwd, ".claude")
	if fi, err := os.Stat(claudeDir); err == nil && fi.IsDir() {
		return CheckResult{
			Name:    "Override Precedence",
			Status:  "WARN",
			Details: "Found .claude/ directory. Devin CLI reads .claude/ hooks by default; ensure no conflicting rules exist there.",
		}
	}

	return CheckResult{
		Name:    "Override Precedence",
		Status:  "OK",
		Details: "No conflicting legacy config directories detected.",
	}
}
