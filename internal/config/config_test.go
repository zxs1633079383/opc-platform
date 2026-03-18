package config

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"warning", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"DEBUG", zapcore.DebugLevel},
		{"Info", zapcore.InfoLevel},
		{"WARN", zapcore.WarnLevel},
		{"ERROR", zapcore.ErrorLevel},
		{"", zapcore.InfoLevel},
		{"invalid", zapcore.InfoLevel},
		{"  debug  ", zapcore.DebugLevel},
	}

	for _, tt := range tests {
		t.Run("input_"+tt.input, func(t *testing.T) {
			got := ParseLogLevel(tt.input)
			if got != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestInitLoggerWithLevel(t *testing.T) {
	// Verify InitLogger does not panic with various combinations.
	InitLogger(false, "debug")
	if Logger == nil {
		t.Fatal("Logger should not be nil after InitLogger")
	}

	InitLogger(true, "error")
	if Logger == nil {
		t.Fatal("Logger should not be nil after InitLogger with verbose")
	}

	InitLogger(false, "")
	if Logger == nil {
		t.Fatal("Logger should not be nil after InitLogger with empty level")
	}
}
