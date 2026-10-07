package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/anton-kaitharan/wardent/internal/adapter"
	"github.com/anton-kaitharan/wardent/internal/adapter/devin"
	"github.com/anton-kaitharan/wardent/internal/audit"
)

const InternalDeadline = 1500 * time.Millisecond

type Heartbeat struct {
	LastEventTS   string `json:"last_event_ts"`
	LastAgent     string `json:"last_agent"`
	LastSessionID string `json:"last_session_id"`
	LastEventName string `json:"last_event_name"`
	TotalEvents   int64  `json:"total_events"`
}

func UpdateHeartbeat(dir, agent, sessionID, eventName, ts string) {
	hbPath := filepath.Join(dir, "heartbeat.json")
	var hb Heartbeat
	if data, err := os.ReadFile(hbPath); err == nil {
		_ = json.Unmarshal(data, &hb)
	}
	hb.LastEventTS = ts
	hb.LastAgent = agent
	hb.LastSessionID = sessionID
	hb.LastEventName = eventName
	hb.TotalEvents++

	if out, err := json.MarshalIndent(hb, "", "  "); err == nil {
		_ = os.WriteFile(hbPath, append(out, '\n'), 0644)
	}
}

func Handle(agentName, eventSlug string, r io.Reader, w io.Writer) (exitCode int) {
	// Guaranteed fail-safe: catch any panics and always exit 0
	defer func() {
		if rec := recover(); rec != nil {
			logDiagnostic(fmt.Sprintf("PANIC in hook handler: %v", rec))
			exitCode = 0
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), InternalDeadline)
	defer cancel()

	done := make(chan struct{})
	var handleErr error

	go func() {
		defer close(done)
		handleErr = executeHook(ctx, agentName, eventSlug, r)
	}()

	select {
	case <-ctx.Done():
		logDiagnostic(fmt.Sprintf("hook execution exceeded internal deadline (%v)", InternalDeadline))
		return 0
	case <-done:
		if handleErr != nil {
			logDiagnostic(fmt.Sprintf("hook execution error: %v", handleErr))
		}
		return 0
	}
}

func executeHook(ctx context.Context, agentName, eventSlug string, r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}
	if len(raw) == 0 {
		return fmt.Errorf("empty stdin payload")
	}

	var ad adapter.Adapter
	switch agentName {
	case "devin":
		ad = devin.New()
	default:
		return fmt.Errorf("unsupported agent: %s", agentName)
	}

	ev, err := ad.Parse(eventSlug, raw)
	if err != nil {
		return fmt.Errorf("adapter parse error: %w", err)
	}

	logger, err := audit.NewLogger()
	if err != nil {
		return fmt.Errorf("logger init error: %w", err)
	}
	defer logger.Close()

	if err := logger.Append(ev); err != nil {
		return fmt.Errorf("failed to append to audit log: %w", err)
	}

	UpdateHeartbeat(audit.DefaultLogDir(), agentName, ev.SessionID, ev.Kind, ev.TS)

	return nil
}

func logDiagnostic(msg string) {
	diagPath := filepath.Join(audit.DefaultLogDir(), "wardent_diagnostics.log")
	line := fmt.Sprintf("[%s] %s\n", time.Now().UTC().Format(time.RFC3339), msg)
	_ = os.MkdirAll(audit.DefaultLogDir(), 0700)
	f, err := os.OpenFile(diagPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		_, _ = f.WriteString(line)
		_ = f.Close()
	}
}
