package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallProjectIdempotent(t *testing.T) {
	tempDir := t.TempDir()

	// Initial install
	backup1, err := InstallProject(tempDir)
	if err != nil {
		t.Fatalf("InstallProject failed: %v", err)
	}
	if backup1 != "" {
		t.Errorf("expected no backup on fresh install, got: %s", backup1)
	}

	target := GetProjectHooksPath(tempDir)
	data1, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read installed hooks: %v", err)
	}

	var m1 HooksMap
	if err := json.Unmarshal(data1, &m1); err != nil {
		t.Fatalf("unmarshal installed hooks failed: %v", err)
	}
	if len(m1) != len(ManagedEvents) {
		t.Errorf("expected %d events, got %d", len(ManagedEvents), len(m1))
	}

	// Second install (idempotency check)
	backup2, err := InstallProject(tempDir)
	if err != nil {
		t.Fatalf("second InstallProject failed: %v", err)
	}
	if backup2 == "" {
		t.Errorf("expected backup file created on re-install")
	}

	data2, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read installed hooks after 2nd run: %v", err)
	}

	var m2 HooksMap
	if err := json.Unmarshal(data2, &m2); err != nil {
		t.Fatalf("unmarshal 2nd hooks failed: %v", err)
	}

	// Verify exact same number of entries (no duplicates!)
	for evName, groups := range m2 {
		if len(groups) != 1 {
			t.Errorf("expected exactly 1 group for %s, got %d", evName, len(groups))
		}
	}

	// Uninstall
	if err := UninstallProject(tempDir); err != nil {
		t.Fatalf("UninstallProject failed: %v", err)
	}

	// Target should now be deleted since there were no user hooks
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected hooks file to be removed on full uninstall")
	}

	// Verify .claude directory was NEVER touched
	claudeDir := filepath.Join(tempDir, ".claude")
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Errorf("Wardent must NEVER create or touch .claude directory!")
	}
}

func TestPreservesCustomHooksOnInstallAndUninstall(t *testing.T) {
	tempDir := t.TempDir()
	target := GetProjectHooksPath(tempDir)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}

	// Create initial hooks file with custom user hook
	custom := HooksMap{
		"PreToolUse": []EventMatcherGroup{
			{
				Matcher: "exec",
				Hooks: []HookEntry{
					{
						Type:    "command",
						Command: "my-custom-check.sh",
					},
				},
			},
		},
	}
	initialBytes, _ := json.MarshalIndent(custom, "", "  ")
	_ = os.WriteFile(target, initialBytes, 0644)

	// Install Wardent
	_, err := InstallProject(tempDir)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	var installed HooksMap
	_ = json.Unmarshal(data, &installed)

	// Custom hook must still be present under PreToolUse
	preGroups := installed["PreToolUse"]
	foundCustom := false
	foundWardent := false
	for _, g := range preGroups {
		for _, h := range g.Hooks {
			if h.Command == "my-custom-check.sh" {
				foundCustom = true
			}
			if strings.Contains(h.Command, "wardent hook devin pre-tool-use") {
				foundWardent = true
			}
		}
	}
	if !foundCustom {
		t.Errorf("custom hook was destroyed by install!")
	}
	if !foundWardent {
		t.Errorf("wardent hook was not installed!")
	}

	// Uninstall Wardent
	if err := UninstallProject(tempDir); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	afterUninstallData, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("target was deleted even though custom hooks existed!")
	}

	var after HooksMap
	_ = json.Unmarshal(afterUninstallData, &after)

	// Custom hook must still be there, but Wardent removed
	for _, g := range after["PreToolUse"] {
		for _, h := range g.Hooks {
			if strings.Contains(h.Command, "wardent") {
				t.Errorf("wardent hook still present after uninstall!")
			}
			if h.Command == "my-custom-check.sh" {
				foundCustom = true
			}
		}
	}
	if !foundCustom {
		t.Errorf("custom hook disappeared after uninstall!")
	}
}
