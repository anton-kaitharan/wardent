package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anton-kaitharan/wardent/internal/event"
)

const (
	DefaultMaxFileSize = 10 * 1024 * 1024 // 10MB
	DefaultMaxBackups  = 5
	DefaultMaxAgeDays  = 30
)

type AuditRecord struct {
	SchemaVersion int              `json:"schema_version"`
	RecordID      string           `json:"record_id"`
	LoggedAt      string           `json:"logged_at"` // ISO8601 UTC
	Event         event.AgentEvent `json:"event"`
}

type Logger struct {
	mu          sync.Mutex
	dir         string
	filename    string
	filePath    string
	maxSize     int64
	maxBackups  int
	maxAgeDays  int
	file        *os.File
	currentSize int64
}

type Option func(*Logger)

func WithDir(dir string) Option {
	return func(l *Logger) {
		l.dir = dir
	}
}

func WithMaxSize(size int64) Option {
	return func(l *Logger) {
		l.maxSize = size
	}
}

func WithMaxBackups(backups int) Option {
	return func(l *Logger) {
		l.maxBackups = backups
	}
}

func WithMaxAgeDays(days int) Option {
	return func(l *Logger) {
		l.maxAgeDays = days
	}
}

func DefaultLogDir() string {
	if dir := os.Getenv("WARDENT_DATA_DIR"); dir != "" {
		return dir
	}
	if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
		return filepath.Join(appData, "wardent")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "wardent")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "wardent")
	}
	return filepath.Join(home, ".local", "share", "wardent")
}

func NewLogger(opts ...Option) (*Logger, error) {
	l := &Logger{
		dir:        DefaultLogDir(),
		filename:   "audit.jsonl",
		maxSize:    DefaultMaxFileSize,
		maxBackups: DefaultMaxBackups,
		maxAgeDays: DefaultMaxAgeDays,
	}
	for _, opt := range opts {
		opt(l)
	}

	l.filePath = filepath.Join(l.dir, l.filename)
	if err := os.MkdirAll(l.dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	return l, nil
}

func (l *Logger) FilePath() string {
	return l.filePath
}

func (l *Logger) RedactEvent(ev event.AgentEvent) event.AgentEvent {
	// Deep copy/redact sensitive fields
	if ev.Prompt != nil {
		ev.Prompt.Text = RedactText(ev.Prompt.Text)
	}
	if ev.Tool != nil {
		if ev.Tool.Shell != nil {
			ev.Tool.Shell.Command = RedactText(ev.Tool.Shell.Command)
		}
		if ev.Tool.PatchText != "" {
			ev.Tool.PatchText = RedactText(ev.Tool.PatchText)
		}
		for i := range ev.Tool.Files {
			ev.Tool.Files[i].Path = RedactText(ev.Tool.Files[i].Path)
		}
	}
	if ev.Result != nil {
		ev.Result.OutputExcerpt = RedactText(ev.Result.OutputExcerpt)
		ev.Result.ChangedFiles = RedactStringSlice(ev.Result.ChangedFiles)
	}
	if ev.Stop != nil {
		ev.Stop.AssistantMessage = RedactText(ev.Stop.AssistantMessage)
	}
	return ev
}

func (l *Logger) Append(ev event.AgentEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	redacted := l.RedactEvent(ev)
	rec := AuditRecord{
		SchemaVersion: event.SchemaVersion,
		RecordID:      event.GenerateID(),
		LoggedAt:      event.NowISO8601(),
		Event:         redacted,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal audit record: %w", err)
	}
	data = append(data, '\n')

	if err := l.ensureFileOpen(); err != nil {
		return err
	}

	// Check if rotation is needed
	if l.currentSize+int64(len(data)) > l.maxSize && l.maxSize > 0 {
		if err := l.rotate(); err != nil {
			// rotation error is non-fatal: continue writing to current file
			_ = err
		}
	}

	n, err := l.file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write audit record: %w", err)
	}
	l.currentSize += int64(n)

	return nil
}

func (l *Logger) ensureFileOpen() error {
	if l.file != nil {
		return nil
	}

	if err := os.MkdirAll(l.dir, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}

	l.file = f
	l.currentSize = fi.Size()
	return nil
}

func (l *Logger) rotate() error {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	// Shift existing rotated files: audit.4.jsonl -> audit.5.jsonl
	for i := l.maxBackups - 1; i >= 1; i-- {
		oldPath := filepath.Join(l.dir, fmt.Sprintf("audit.%d.jsonl", i))
		newPath := filepath.Join(l.dir, fmt.Sprintf("audit.%d.jsonl", i+1))
		if _, err := os.Stat(oldPath); err == nil {
			_ = os.Rename(oldPath, newPath)
		}
	}

	// Rename current file to audit.1.jsonl
	firstBackup := filepath.Join(l.dir, "audit.1.jsonl")
	if _, err := os.Stat(l.filePath); err == nil {
		_ = os.Rename(l.filePath, firstBackup)
	}

	// Remove files older than maxBackups
	excess := filepath.Join(l.dir, fmt.Sprintf("audit.%d.jsonl", l.maxBackups+1))
	_ = os.Remove(excess)

	// Clean up by age
	l.cleanOldFiles()

	// Reopen a fresh audit.jsonl
	return l.ensureFileOpen()
}

func (l *Logger) cleanOldFiles() {
	if l.maxAgeDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -l.maxAgeDays)

	files, err := filepath.Glob(filepath.Join(l.dir, "audit.*.jsonl"))
	if err != nil {
		return
	}

	for _, file := range files {
		if fi, err := os.Stat(file); err == nil && fi.ModTime().Before(cutoff) {
			_ = os.Remove(file)
		}
	}
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}
