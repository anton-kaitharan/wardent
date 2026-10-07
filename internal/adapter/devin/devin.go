package devin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anton-kaitharan/wardent/internal/event"
)

const AdapterVersion = "0.1.0"

type rawPayload struct {
	HookEventName        string                 `json:"hook_event_name"`
	ToolName             string                 `json:"tool_name"`
	ToolInput            map[string]interface{} `json:"tool_input"`
	ToolUseID            string                 `json:"tool_use_id"`
	SessionID            string                 `json:"session_id"`
	PromptID             string                 `json:"prompt_id"`
	Prompt               string                 `json:"prompt"`
	StopHookActive       bool                   `json:"stop_hook_active"`
	LastAssistantMessage string                 `json:"last_assistant_message"`
	Source               string                 `json:"source"`
	Reason               string                 `json:"reason"`
	ToolResponse         *struct {
		Success bool    `json:"success"`
		Output  string  `json:"output"`
		Error   *string `json:"error"`
	} `json:"tool_response"`
}

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) AgentKind() string {
	return "devin"
}

func (a *Adapter) Parse(eventName string, raw []byte) (event.AgentEvent, error) {
	var p rawPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return event.AgentEvent{}, fmt.Errorf("failed to parse Devin hook payload: %w", err)
	}

	effEvent := p.HookEventName
	if effEvent == "" {
		effEvent = eventName
	}

	cwd, _ := os.Getwd()
	repoRoot := os.Getenv("DEVIN_PROJECT_DIR")
	if repoRoot == "" {
		repoRoot = cwd
	}

	ev := event.AgentEvent{
		SchemaVersion: event.SchemaVersion,
		EventID:       event.GenerateID(),
		TS:            event.NowISO8601(),
		Agent: event.AgentInfo{
			Kind:           "devin",
			AdapterVersion: AdapterVersion,
		},
		SessionID: p.SessionID,
		TurnID:    p.PromptID,
		CWD:       event.NormalizePath(cwd),
		RepoRoot:  event.NormalizePath(repoRoot),
	}

	switch strings.ToLower(effEvent) {
	case "sessionstart", "session-start":
		ev.Kind = "session_start"
	case "userpromptsubmit", "user-prompt-submit":
		ev.Kind = "prompt_submitted"
		ev.Prompt = &event.PromptInfo{Text: p.Prompt}
	case "pretooluse", "pre-tool-use":
		ev.Kind = "tool_pre"
		ev.Tool = a.parseToolInfo(p, raw)
	case "posttooluse", "post-tool-use":
		ev.Kind = "tool_post"
		ev.Tool = a.parseToolInfo(p, raw)
		if p.ToolResponse != nil {
			var errStr string
			if p.ToolResponse.Error != nil {
				errStr = *p.ToolResponse.Error
			}
			outExcerpt := p.ToolResponse.Output
			if len(outExcerpt) > 1024 {
				outExcerpt = outExcerpt[:1024] + "... [truncated]"
			}
			if errStr != "" && len(outExcerpt) > 0 {
				outExcerpt += " | error: " + errStr
			}
			ev.Result = &event.ResultInfo{
				OK:            p.ToolResponse.Success,
				OutputExcerpt: outExcerpt,
			}
			if !p.ToolResponse.Success {
				ev.Kind = "tool_failed"
			}
		}
	case "permissionrequest", "permission-request":
		ev.Kind = "permission_request"
		ev.Tool = a.parseToolInfo(p, raw)
	case "stop":
		ev.Kind = "turn_stop"
		ev.Stop = &event.StopInfo{
			AssistantMessage: p.LastAssistantMessage,
			StopHookActive:   p.StopHookActive,
		}
	case "sessionend", "session-end":
		ev.Kind = "session_end"
	default:
		ev.Kind = strings.ToLower(effEvent)
	}

	return ev, nil
}

func (a *Adapter) parseToolInfo(p rawPayload, raw []byte) *event.ToolInfo {
	if p.ToolName == "" {
		return nil
	}

	ti := &event.ToolInfo{
		ID:             p.ToolUseID,
		Name:           p.ToolName,
		RawInputDigest: event.Digest(raw),
	}

	toolLower := strings.ToLower(p.ToolName)

	switch {
	case toolLower == "exec":
		ti.Class = "shell"
		cmd, _ := p.ToolInput["command"].(string)
		ti.Shell = &event.ShellInfo{
			Dialect: "posix", // Devin runs hooks and shell under bash on Windows & Unix
			Command: cmd,
		}

	case toolLower == "write" || toolLower == "edit":
		ti.Class = "file_write"
		fp, _ := p.ToolInput["file_path"].(string)
		op := "modify"
		if toolLower == "write" {
			op = "create"
		}
		if fp != "" {
			ti.Files = []event.FileOp{
				{Path: event.NormalizePath(fp), Op: op},
			}
		}

	case toolLower == "read":
		ti.Class = "file_read"
		fp, _ := p.ToolInput["file_path"].(string)
		if fp != "" {
			ti.Files = []event.FileOp{
				{Path: event.NormalizePath(fp), Op: "read"},
			}
		}

	case toolLower == "apply_patch":
		ti.Class = "file_patch"
		patch, _ := p.ToolInput["patch"].(string)
		ti.PatchText = patch

	case strings.HasPrefix(toolLower, "mcp__"):
		ti.Class = "mcp"
		parts := strings.Split(p.ToolName, "__")
		if len(parts) >= 3 {
			ti.MCP = &event.MCPInfo{
				Server: parts[1],
				Tool:   parts[2],
			}
		}

	default:
		ti.Class = "other"
	}

	return ti
}
