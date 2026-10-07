package event

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const SchemaVersion = 1

type AgentInfo struct {
	Kind           string `json:"kind"`
	Version        string `json:"version,omitempty"`
	AdapterVersion string `json:"adapter_version"`
}

type SubagentInfo struct {
	ID   string `json:"id"`
	Type string `json:"type,omitempty"`
}

type PromptInfo struct {
	Text string `json:"text"`
}

type FileOp struct {
	Path string `json:"path"` // normalized forward slashes
	Op   string `json:"op"`   // "read" | "create" | "modify" | "delete"
}

type ShellInfo struct {
	Dialect string `json:"dialect"` // "posix" | "powershell" | "cmd"
	Command string `json:"command"`
}

type MCPInfo struct {
	Server string `json:"server"`
	Tool   string `json:"tool"`
	Source string `json:"source,omitempty"`
}

type ToolInfo struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Class           string     `json:"class"` // "shell" | "file_write" | "file_read" | "file_patch" | "network" | "mcp" | "agent" | "other"
	Shell           *ShellInfo `json:"shell,omitempty"`
	Files           []FileOp   `json:"files,omitempty"`
	PatchText       string     `json:"patch_text,omitempty"`
	MCP             *MCPInfo   `json:"mcp,omitempty"`
	RawInputDigest  string     `json:"raw_input_digest"`
}

type ResultInfo struct {
	OK            bool     `json:"ok"`
	ExitCode      *int     `json:"exit_code,omitempty"`
	OutputExcerpt string   `json:"output_excerpt,omitempty"`
	ChangedFiles  []string `json:"changed_files,omitempty"`
}

type StopInfo struct {
	AssistantMessage string `json:"assistant_message,omitempty"`
	StopHookActive   bool   `json:"stop_hook_active"`
}

type AgentEvent struct {
	SchemaVersion  int           `json:"schema_version"`
	EventID        string        `json:"event_id"`
	TS             string        `json:"ts"` // ISO8601 UTC
	Agent          AgentInfo     `json:"agent"`
	SessionID      string        `json:"session_id"`
	TurnID         string        `json:"turn_id,omitempty"`
	Subagent       *SubagentInfo `json:"subagent,omitempty"`
	CWD            string        `json:"cwd"`
	RepoRoot       string        `json:"repo_root,omitempty"`
	PermissionMode string        `json:"permission_mode,omitempty"`
	Model          string        `json:"model,omitempty"`
	Effort         string        `json:"effort,omitempty"`
	Kind           string        `json:"kind"` // "prompt_submitted" | "tool_pre" | "tool_post" | "tool_failed" | "turn_stop" | "session_start" | "session_end" | "permission_request"
	Prompt         *PromptInfo   `json:"prompt,omitempty"`
	Tool           *ToolInfo     `json:"tool,omitempty"`
	Result         *ResultInfo   `json:"result,omitempty"`
	Stop           *StopInfo     `json:"stop,omitempty"`
}

func GenerateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("evt_%s", hex.EncodeToString(b))
}

func Digest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func NormalizePath(p string) string {
	if p == "" {
		return ""
	}
	clean := filepath.Clean(p)
	return strings.ReplaceAll(clean, "\\", "/")
}

func NowISO8601() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
