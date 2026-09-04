package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSetupText(t *testing.T) {
	var buf bytes.Buffer
	l := Setup("info", "text", &buf)
	l.Info("test message", "key", "value")

	out := buf.String()
	if !strings.Contains(out, "level=INFO") {
		t.Errorf("expected level=INFO in output: %s", out)
	}
	if !strings.Contains(out, `msg="test message"`) {
		t.Errorf("expected msg in output: %s", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Errorf("expected key=value in output: %s", out)
	}
}

func TestSetupJSON(t *testing.T) {
	var buf bytes.Buffer
	l := Setup("debug", "json", &buf)
	l.Debug("json message", "count", 42)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse JSON log: %v", err)
	}
	if m["level"] != "DEBUG" {
		t.Errorf("level = %v, want DEBUG", m["level"])
	}
	if m["msg"] != "json message" {
		t.Errorf("msg = %v, want json message", m["msg"])
	}
	if count, ok := m["count"].(float64); !ok || int(count) != 42 {
		t.Errorf("count = %v, want 42", m["count"])
	}
}
