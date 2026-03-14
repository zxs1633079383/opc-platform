package config

import (
	"os"
	"path/filepath"

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

// InitLogger initializes the global zap logger.
// When verbose is true, the log level is set to Debug; otherwise Info.
func InitLogger(verbose bool) {
	level := zapcore.InfoLevel
	if verbose {
		level = zapcore.DebugLevel
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
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
