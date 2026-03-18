package config

import (
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the global logger instance.
var Logger *zap.SugaredLogger

// Config represents the OPC Platform configuration.
type Config struct {
	// DefaultOutput is the default output format (json, yaml, table).
	DefaultOutput string `yaml:"defaultOutput" mapstructure:"defaultOutput"`

	// LogLevel controls the logging verbosity.
	LogLevel string `yaml:"logLevel" mapstructure:"logLevel"`

	// StateDir is the directory for storing state data.
	StateDir string `yaml:"stateDir" mapstructure:"stateDir"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DefaultOutput: "table",
		LogLevel:      "info",
		StateDir:      filepath.Join(GetConfigDir(), "state"),
	}
}

// GetConfigDir returns the path to ~/.opc/.
func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".opc")
}

// GetStateDir returns the path to ~/.opc/state/.
func GetStateDir() string {
	return filepath.Join(GetConfigDir(), "state")
}

// GetConfigFilePath returns the path to ~/.opc/config.yaml.
func GetConfigFilePath() string {
	return filepath.Join(GetConfigDir(), "config.yaml")
}

// EnsureConfigDir creates the ~/.opc/ directory if it does not exist.
func EnsureConfigDir() error {
	dir := GetConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stateDir := GetStateDir()
	return os.MkdirAll(stateDir, 0o755)
}

// ParseLogLevel converts a string log level to a zapcore.Level.
// Supported values: debug, info, warn, warning, error (case-insensitive).
// Returns InfoLevel for empty or unrecognized values.
func ParseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// InitLogger initializes the global zap logger.
// The level string is parsed via ParseLogLevel. If verbose is true, the level
// is overridden to Debug regardless of the level parameter.
func InitLogger(verbose bool, level string) {
	zapLevel := ParseLogLevel(level)
	if verbose {
		zapLevel = zapcore.DebugLevel
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      verbose,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := cfg.Build()
	if err != nil {
		// Fallback to a no-op logger if build fails.
		logger = zap.NewNop()
	}

	Logger = logger.Sugar()
}
