package hook

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func BenchmarkHookLatency(b *testing.B) {
	tempData := b.TempDir()
	b.Setenv("WARDENT_DATA_DIR", tempData)

	payload := `{
		"hook_event_name": "PreToolUse",
		"tool_name": "exec",
		"tool_input": {
			"command": "npm test -- --coverage && echo done",
			"description": "Run test suite",
			"timeout": 120000
		},
		"tool_use_id": "call_bench_12345",
		"session_id": "sess_bench_test",
		"prompt_id": "turn_bench_1"
	}`

	var out bytes.Buffer

	// Warmup
	for i := 0; i < 5; i++ {
		Handle("devin", "pre-tool-use", strings.NewReader(payload), &out)
	}

	latencies := make([]time.Duration, b.N)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		code := Handle("devin", "pre-tool-use", strings.NewReader(payload), &out)
		latencies[i] = time.Since(t0)
		if code != 0 {
			b.Fatalf("expected code 0, got %d", code)
		}
	}

	if b.N >= 100 {
		// Calculate p50, p95, p99
		durations := make([]float64, len(latencies))
		for i, d := range latencies {
			durations[i] = float64(d.Microseconds()) / 1000.0 // ms
		}
		// sort
		for i := 0; i < len(durations); i++ {
			for j := i + 1; j < len(durations); j++ {
				if durations[i] > durations[j] {
					durations[i], durations[j] = durations[j], durations[i]
				}
			}
		}

		p50 := durations[int(0.50*float64(len(durations)))]
		p95 := durations[int(0.95*float64(len(durations)))]
		p99 := durations[int(0.99*float64(len(durations)))]

		b.Logf("Measured hook latency across %d runs: p50=%.2fms, p95=%.2fms, p99=%.2fms (E5 budget: p95 <= 80ms)", b.N, p50, p95, p99)

		if p95 > 80.0 {
			b.Errorf("Latency exceeded E5 budget! p95=%.2fms > 80ms", p95)
		}
	}
}
