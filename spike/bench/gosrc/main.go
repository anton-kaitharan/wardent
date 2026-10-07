package main

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Rules struct {
	Regexes []string `json:"regexes"`
	Globs   []string `json:"globs"`
	Secrets []string `json:"secrets"`
}
type Pol struct {
	Rules []struct {
		Match struct {
			CommandRegex string `json:"command_regex"`
		} `json:"match"`
	} `json:"rules"`
}

func g2r(g string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch {
		case c == '*' && i+1 < len(g) && g[i+1] == '*':
			if i+2 < len(g) && g[i+2] == '/' {
				b.WriteString("(.*/)?")
				i += 2
			} else {
				b.WriteString(".*")
				i++
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case strings.ContainsRune(".+^${}()|[]\\", rune(c)):
			b.WriteString("\\" + string(c))
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "noop" {
		return
	}
	in, _ := io.ReadAll(os.Stdin)
	var ev struct {
		Name string                 `json:"hook_event_name"`
		Tool string                 `json:"tool_name"`
		TI   map[string]interface{} `json:"tool_input"`
	}
	if err := json.Unmarshal(in, &ev); err != nil {
		os.Exit(2)
	}
	rb, _ := os.ReadFile(os.Args[1])
	var rules Rules
	json.Unmarshal(rb, &rules)
	pb, _ := os.ReadFile(os.Args[2])
	var pol Pol
	json.Unmarshal(pb, &pol)
	var hits []string
	cmd, _ := ev.TI["command"].(string)
	fp, _ := ev.TI["file_path"].(string)
	ct, _ := ev.TI["content"].(string)
	if cmd != "" {
		for i, r := range rules.Regexes {
			if regexp.MustCompile("(?i)" + r).MatchString(cmd) {
				hits = append(hits, "re"+strconv.Itoa(i))
			}
		}
		for i, r := range pol.Rules {
			if regexp.MustCompile("(?i)" + r.Match.CommandRegex).MatchString(cmd) {
				hits = append(hits, "pol"+strconv.Itoa(i))
			}
		}
	}
	if fp != "" {
		p := strings.ReplaceAll(fp, "\\", "/")
		for i, g := range rules.Globs {
			if g2r(g).MatchString(p) {
				hits = append(hits, "gl"+strconv.Itoa(i))
			}
		}
	}
	if ct != "" {
		for i, r := range rules.Secrets {
			if regexp.MustCompile(r).MatchString(ct) {
				hits = append(hits, "sec"+strconv.Itoa(i))
			}
		}
	}
	line, _ := json.Marshal(map[string]interface{}{"ts": time.Now().UnixMilli(), "ev": ev.Name, "tool": ev.Tool, "n": len(in), "hits": hits})
	f, _ := os.OpenFile(os.Args[3], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.Write(append(line, '\n'))
	f.Close()
	if len(hits) > 0 {
		out, _ := json.Marshal(map[string]interface{}{"hookSpecificOutput": map[string]interface{}{"hookEventName": "PreToolUse", "permissionDecision": "ask", "permissionDecisionReason": strings.Join(hits, ",")}})
		os.Stdout.Write(out)
	}
}
