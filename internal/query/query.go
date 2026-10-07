package query

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anton-kaitharan/wardent/internal/audit"
)

type Filter struct {
	SessionID string
	ToolName  string
	Since     time.Time
	Limit     int
}

func ReadAuditRecords(dir string) ([]audit.AuditRecord, error) {
	if dir == "" {
		dir = audit.DefaultLogDir()
	}
	logPath := filepath.Join(dir, "audit.jsonl")

	f, err := os.Open(logPath)
	if os.IsNotExist(err) {
		return nil, nil // no records yet
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []audit.AuditRecord
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec audit.AuditRecord
		if err := json.Unmarshal(line, &rec); err == nil {
			records = append(records, rec)
		}
	}

	return records, scanner.Err()
}

func FilterRecords(records []audit.AuditRecord, f Filter) []audit.AuditRecord {
	var out []audit.AuditRecord
	for i := len(records) - 1; i >= 0; i-- { // newest first
		rec := records[i]

		if f.SessionID != "" && !strings.EqualFold(rec.Event.SessionID, f.SessionID) {
			continue
		}
		if f.ToolName != "" {
			if rec.Event.Tool == nil || !strings.EqualFold(rec.Event.Tool.Name, f.ToolName) {
				continue
			}
		}
		if !f.Since.IsZero() {
			t, err := time.Parse(time.RFC3339Nano, rec.LoggedAt)
			if err != nil {
				t, err = time.Parse(time.RFC3339, rec.LoggedAt)
			}
			if err == nil && t.Before(f.Since) {
				continue
			}
		}

		out = append(out, rec)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out
}

func PrintLogTable(records []audit.AuditRecord, w io.Writer) {
	if len(records) == 0 {
		fmt.Fprintln(w, "No audit records found.")
		return
	}

	fmt.Fprintf(w, "%-20s %-15s %-16s %-12s %s\n", "TIME (UTC)", "AGENT", "EVENT", "TOOL", "DETAIL")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 80))

	for _, rec := range records {
		ts := rec.LoggedAt
		if len(ts) > 19 {
			ts = ts[:19] // trim to YYYY-MM-DDTHH:MM:SS
		}
		tool := "-"
		detail := "-"

		if rec.Event.Tool != nil {
			tool = rec.Event.Tool.Name
			if rec.Event.Tool.Shell != nil {
				detail = rec.Event.Tool.Shell.Command
			} else if len(rec.Event.Tool.Files) > 0 {
				detail = fmt.Sprintf("%s (%s)", rec.Event.Tool.Files[0].Path, rec.Event.Tool.Files[0].Op)
			}
		} else if rec.Event.Prompt != nil {
			detail = rec.Event.Prompt.Text
		} else if rec.Event.Stop != nil {
			detail = rec.Event.Stop.AssistantMessage
		}

		if len(detail) > 40 {
			detail = detail[:37] + "..."
		}

		fmt.Fprintf(w, "%-20s %-15s %-16s %-12s %s\n", ts, rec.Event.Agent.Kind, rec.Event.Kind, tool, detail)
	}
}

func ExplainRecord(records []audit.AuditRecord, id string, w io.Writer) error {
	var target *audit.AuditRecord
	for _, r := range records {
		if strings.EqualFold(r.RecordID, id) || strings.EqualFold(r.Event.EventID, id) {
			target = &r
			break
		}
	}

	if target == nil {
		return fmt.Errorf("record %s not found in audit log", id)
	}

	fmt.Fprintf(w, "Audit Record: %s\n", target.RecordID)
	fmt.Fprintf(w, "Logged At:    %s\n", target.LoggedAt)
	fmt.Fprintf(w, "Agent:        %s (adapter %s)\n", target.Event.Agent.Kind, target.Event.Agent.AdapterVersion)
	fmt.Fprintf(w, "Session ID:   %s\n", target.Event.SessionID)
	if target.Event.TurnID != "" {
		fmt.Fprintf(w, "Turn ID:      %s\n", target.Event.TurnID)
	}
	fmt.Fprintf(w, "Event Kind:   %s\n", target.Event.Kind)
	fmt.Fprintf(w, "Working Dir:  %s\n", target.Event.CWD)
	fmt.Fprintf(w, "\nPayload Details:\n")

	if target.Event.Tool != nil {
		fmt.Fprintf(w, "  Tool Name:  %s\n", target.Event.Tool.Name)
		fmt.Fprintf(w, "  Tool Class: %s\n", target.Event.Tool.Class)
		if target.Event.Tool.Shell != nil {
			fmt.Fprintf(w, "  Command:    %s\n", target.Event.Tool.Shell.Command)
		}
		for _, f := range target.Event.Tool.Files {
			fmt.Fprintf(w, "  File:       %s [%s]\n", f.Path, f.Op)
		}
	}
	if target.Event.Prompt != nil {
		fmt.Fprintf(w, "  Prompt:     %s\n", target.Event.Prompt.Text)
	}
	if target.Event.Result != nil {
		fmt.Fprintf(w, "  Success:    %v\n", target.Event.Result.OK)
		if target.Event.Result.OutputExcerpt != "" {
			fmt.Fprintf(w, "  Output:     %s\n", target.Event.Result.OutputExcerpt)
		}
	}

	fmt.Fprintf(w, "\nWardent Assessment:\n")
	fmt.Fprintf(w, "  Mode:       OBSERVE-ONLY (Phase 1)\n")
	fmt.Fprintf(w, "  Action:     allow_silent (no intervention)\n")

	// Advisory scan check
	var flags []string
	if target.Event.Tool != nil && target.Event.Tool.Shell != nil {
		cmd := target.Event.Tool.Shell.Command
		if strings.Contains(cmd, "rm -rf") || strings.Contains(cmd, "git push -f") || strings.Contains(cmd, "--force") {
			flags = append(flags, "destructive command pattern detected")
		}
	}
	if len(flags) > 0 {
		fmt.Fprintf(w, "  Notice:     %s\n", strings.Join(flags, "; "))
		fmt.Fprintf(w, "  Advisory:   In Phase 2 (Risk Guard), this would trigger a policy ask or deny rule.\n")
	} else {
		fmt.Fprintf(w, "  Notice:     Standard benign operation\n")
	}

	return nil
}
