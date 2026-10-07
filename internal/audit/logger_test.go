package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anton-kaitharan/wardent/internal/event"
)

func TestRedactText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes []string
	}{
		{
			name:     "AWS Access Key",
			input:    "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE and run",
			contains: "[REDACTED_AWS_KEY]",
			excludes: []string{"AKIAIOSFODNN7EXAMPLE"},
		},
		{
			name:     "GitHub Token",
			input:    "curl -H 'Authorization: token ghp_123456789012345678901234567890123456' https://api.github.com",
			contains: "[REDACTED_GITHUB_TOKEN]",
			excludes: []string{"ghp_123456789012345678901234567890123456"},
		},
		{
			name:     "OpenAI / Anthropic Secret Key",
			input:    "export OPENAI_API_KEY=sk-proj-123456789012345678901234567890",
			contains: "[REDACTED_API_KEY]",
			excludes: []string{"sk-proj-123456789012345678901234567890"},
		},
		{
			name:     "Slack Token",
			input:    "slack_token: xoxb-1234567890-123456789012-abcdef123456",
			contains: "[REDACTED_SLACK_TOKEN]",
			excludes: []string{"xoxb-1234567890-123456789012-abcdef123456"},
		},
		{
			name:     "Stripe Secret Key",
			input:    "stripe.api_key = '" + "sk_" + "live_" + "1234567890abcdef12345678" + "'",
			contains: "[REDACTED_STRIPE_KEY]",
			excludes: []string{"sk_" + "live_" + "1234567890abcdef12345678"},
		},
		{
			name:     "RSA Private Key Multiline",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0Y+...\n-----END RSA PRIVATE KEY-----",
			contains: "[REDACTED_PRIVATE_KEY]",
			excludes: []string{"MIIEowIBAAKCAQEA0Y"},
		},
		{
			name:     "Authorization Bearer Header",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz.abc",
			contains: "[REDACTED_AUTH_TOKEN]",
			excludes: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		},
		{
			name:     "Generic Password Assignment",
			input:    "DB_PASSWORD='super_secret_db_pass_1234'; connect()",
			contains: "[REDACTED",
			excludes: []string{"super_secret_db_pass_1234"},
		},
		{
			name: "Dotenv File Content",
			input: `
PORT=8080
DATABASE_URL=postgres://localhost/db
SECRET_KEY="very_confidential_secret_string_12345"
API_TOKEN=token_value_987654321
LOG_LEVEL=info
`,
			contains: "[REDACTED_ENV_VALUE]",
			excludes: []string{"very_confidential_secret_string_12345", "token_value_987654321"},
		},
		{
			name:     "Windows Path with Secret in Arguments",
			input:    `C:\Windows\System32\cmd.exe /c "set DB_PASSWORD=my_prod_password_999 && node app.js"`,
			contains: "[REDACTED_SECRET]",
			excludes: []string{"my_prod_password_999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactText(tt.input)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("expected redacted text to contain %q, got: %s", tt.contains, got)
			}
			for _, ex := range tt.excludes {
				if strings.Contains(got, ex) {
					t.Errorf("redacted text must not contain secret %q, got: %s", ex, got)
				}
			}
		})
	}
}

func TestLoggerAppendAndRotate(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(
		WithDir(tempDir),
		WithMaxSize(200), // very small max size to test rotation quickly
		WithMaxBackups(3),
	)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Append 10 events
	for i := 0; i < 10; i++ {
		ev := event.AgentEvent{
			SchemaVersion: event.SchemaVersion,
			EventID:       event.GenerateID(),
			TS:            event.NowISO8601(),
			Agent: event.AgentInfo{
				Kind:           "devin",
				AdapterVersion: "0.1.0",
			},
			SessionID: "sess_test",
			Kind:      "tool_pre",
			Tool: &event.ToolInfo{
				ID:    "tool_1",
				Name:  "exec",
				Class: "shell",
				Shell: &event.ShellInfo{
					Dialect: "posix",
					Command: "echo AKIAIOSFODNN7EXAMPLE",
				},
				RawInputDigest: event.Digest([]byte("test")),
			},
		}

		if err := logger.Append(ev); err != nil {
			t.Fatalf("failed to append record %d: %v", i, err)
		}
	}

	// Verify main audit.jsonl exists and is not empty
	mainLog := filepath.Join(tempDir, "audit.jsonl")
	data, err := os.ReadFile(mainLog)
	if err != nil {
		t.Fatalf("failed to read audit.jsonl: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("audit.jsonl should not be empty")
	}

	// Verify redaction was applied
	if strings.Contains(string(data), "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("audit log contains unredacted AWS key!")
	}
	if !strings.Contains(string(data), "[REDACTED_AWS_KEY]") {
		t.Errorf("audit log missing redacted marker")
	}

	// Verify rotated backup files exist
	rotated1 := filepath.Join(tempDir, "audit.1.jsonl")
	if _, err := os.Stat(rotated1); os.IsNotExist(err) {
		t.Errorf("expected rotated backup file %s to exist", rotated1)
	}

	// Verify records deserialize properly
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		var rec AuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("failed to unmarshal JSONL line: %v", err)
		}
		if rec.SchemaVersion != event.SchemaVersion {
			t.Errorf("schema version mismatch: got %d, want %d", rec.SchemaVersion, event.SchemaVersion)
		}
	}
}
