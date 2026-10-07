package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var (
	reWardentCmd = regexp.MustCompile(`\bwardent(\.exe)?\s+hook\s+devin\b`)
)

var ManagedEvents = []struct {
	EventName string
	Slug      string
}{
	{"SessionStart", "session-start"},
	{"UserPromptSubmit", "user-prompt-submit"},
	{"PreToolUse", "pre-tool-use"},
	{"PostToolUse", "post-tool-use"},
	{"Stop", "stop"},
	{"SessionEnd", "session-end"},
}

type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type EventMatcherGroup struct {
	Matcher string      `json:"matcher"`
	Hooks   []HookEntry `json:"hooks"`
}

type HooksMap map[string][]EventMatcherGroup

func GetUserConfigPath() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "devin", "config.json")
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "devin", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "devin", "config.json")
	}
	return filepath.Join(home, ".config", "devin", "config.json")
}

func GetProjectHooksPath(projectDir string) string {
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	return filepath.Join(projectDir, ".devin", "hooks.v1.json")
}

func BackupFile(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil // nothing to backup
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}
	return backupPath, nil
}

func isWardentHook(cmd string) bool {
	return reWardentCmd.MatchString(cmd)
}

func buildWardentGroups() HooksMap {
	m := make(HooksMap, len(ManagedEvents))
	for _, ev := range ManagedEvents {
		m[ev.EventName] = []EventMatcherGroup{
			{
				Matcher: ".*",
				Hooks: []HookEntry{
					{
						Type:    "command",
						Command: fmt.Sprintf("wardent hook devin %s", ev.Slug),
						Timeout: 5,
					},
				},
			},
		}
	}
	return m
}

func mergeHooksMap(existing HooksMap) HooksMap {
	res := make(HooksMap)
	// copy existing non-Wardent hooks
	for evName, groups := range existing {
		var cleanGroups []EventMatcherGroup
		for _, g := range groups {
			var cleanHooks []HookEntry
			for _, h := range g.Hooks {
				if !isWardentHook(h.Command) {
					cleanHooks = append(cleanHooks, h)
				}
			}
			if len(cleanHooks) > 0 {
				g.Hooks = cleanHooks
				cleanGroups = append(cleanGroups, g)
			}
		}
		if len(cleanGroups) > 0 {
			res[evName] = cleanGroups
		}
	}

	// append canonical Wardent hooks
	wardent := buildWardentGroups()
	for evName, wGroups := range wardent {
		res[evName] = append(res[evName], wGroups...)
	}

	return res
}

func removeWardentHooks(existing HooksMap) HooksMap {
	res := make(HooksMap)
	for evName, groups := range existing {
		var cleanGroups []EventMatcherGroup
		for _, g := range groups {
			var cleanHooks []HookEntry
			for _, h := range g.Hooks {
				if !isWardentHook(h.Command) {
					cleanHooks = append(cleanHooks, h)
				}
			}
			if len(cleanHooks) > 0 {
				g.Hooks = cleanHooks
				cleanGroups = append(cleanGroups, g)
			}
		}
		if len(cleanGroups) > 0 {
			res[evName] = cleanGroups
		}
	}
	return res
}

// InstallProject installs hooks into .devin/hooks.v1.json at projectDir
func InstallProject(projectDir string) (string, error) {
	target := GetProjectHooksPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", fmt.Errorf("failed to create .devin dir: %w", err)
	}

	backup, err := BackupFile(target)
	if err != nil {
		return "", fmt.Errorf("failed to backup existing hooks file: %w", err)
	}

	var existing HooksMap
	if data, err := os.ReadFile(target); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = make(HooksMap)
	}

	merged := mergeHooksMap(existing)
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return "", err
	}
	out = append(out, '\n')

	if err := os.WriteFile(target, out, 0644); err != nil {
		return "", err
	}

	return backup, nil
}

// UninstallProject removes Wardent hooks from .devin/hooks.v1.json
func UninstallProject(projectDir string) error {
	target := GetProjectHooksPath(projectDir)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}

	var existing HooksMap
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("invalid hooks file format: %w", err)
	}

	cleaned := removeWardentHooks(existing)
	if len(cleaned) == 0 {
		return os.Remove(target)
	}

	out, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	return os.WriteFile(target, out, 0644)
}

// InstallUser installs hooks into user config file (%APPDATA%\devin\config.json)
func InstallUser() (string, error) {
	target := GetUserConfigPath()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", fmt.Errorf("failed to create user config dir: %w", err)
	}

	backup, err := BackupFile(target)
	if err != nil {
		return "", fmt.Errorf("failed to backup existing user config: %w", err)
	}

	configObj := make(map[string]interface{})
	if data, err := os.ReadFile(target); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &configObj)
	}

	var existingHooks HooksMap
	if rawHooks, ok := configObj["hooks"]; ok {
		if hooksBytes, err := json.Marshal(rawHooks); err == nil {
			_ = json.Unmarshal(hooksBytes, &existingHooks)
		}
	}
	if existingHooks == nil {
		existingHooks = make(HooksMap)
	}

	merged := mergeHooksMap(existingHooks)
	configObj["hooks"] = merged

	out, err := json.MarshalIndent(configObj, "", "  ")
	if err != nil {
		return "", err
	}
	out = append(out, '\n')

	if err := os.WriteFile(target, out, 0644); err != nil {
		return "", err
	}

	return backup, nil
}

// UninstallUser removes Wardent hooks from user config file
func UninstallUser() error {
	target := GetUserConfigPath()
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}

	configObj := make(map[string]interface{})
	if err := json.Unmarshal(data, &configObj); err != nil {
		return fmt.Errorf("invalid config file: %w", err)
	}

	rawHooks, ok := configObj["hooks"]
	if !ok {
		return nil
	}

	var existingHooks HooksMap
	if hooksBytes, err := json.Marshal(rawHooks); err == nil {
		_ = json.Unmarshal(hooksBytes, &existingHooks)
	}

	cleaned := removeWardentHooks(existingHooks)
	if len(cleaned) == 0 {
		delete(configObj, "hooks")
	} else {
		configObj["hooks"] = cleaned
	}

	out, err := json.MarshalIndent(configObj, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	return os.WriteFile(target, out, 0644)
}
