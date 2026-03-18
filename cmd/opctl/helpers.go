package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"github.com/zlc-ai/opc-platform/pkg/adapter/claudecode"
	"github.com/zlc-ai/opc-platform/pkg/adapter/codex"
	"github.com/zlc-ai/opc-platform/pkg/adapter/custom"
	"github.com/zlc-ai/opc-platform/pkg/adapter/openclaw"
	"github.com/zlc-ai/opc-platform/pkg/client"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/storage/sqlite"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

// getController creates a local Controller with storage and adapter registry.
// Returns the controller, a cleanup function, and any error.
func getController() (*controller.Controller, func(), error) {
	if err := config.EnsureConfigDir(); err != nil {
		return nil, nil, err
	}

	dbPath := filepath.Join(config.GetStateDir(), "opc.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		return nil, nil, err
	}

	registry := adapter.NewRegistry()
	registry.Register(v1.AgentTypeOpenClaw, func() adapter.Adapter {
		return openclaw.New()
	})
	registry.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter {
		return claudecode.New()
	})
	registry.Register(v1.AgentTypeCodex, func() adapter.Adapter {
		return codex.New()
	})
	registry.Register(v1.AgentTypeCustom, func() adapter.Adapter {
		return custom.New()
	})

	logger := config.Logger
	if logger == nil {
		config.InitLogger(false, "")
		logger = config.Logger
	}

	ctrl := controller.New(store, registry, logger)

	cleanup := func() {
		store.Close()
	}

	return ctrl, cleanup, nil
}

// getDaemonClient returns an HTTP client connected to the running daemon,
// or nil if no daemon is detected.
func getDaemonClient() *client.Client {
	addr := getDaemonAddr()
	if addr == "" {
		return nil
	}
	c := client.New(addr)
	if err := c.Ping(); err != nil {
		return nil
	}
	return c
}

// getDaemonAddr reads the daemon address from the addr file.
// Returns empty string if no daemon is running.
func getDaemonAddr() string {
	addrPath := filepath.Join(config.GetConfigDir(), "daemon.addr")
	data, err := os.ReadFile(addrPath)
	if err != nil {
		return ""
	}
	addr := strings.TrimSpace(string(data))
	if addr == "" {
		return ""
	}
	return addr
}

// isDaemonRunning checks if the daemon is reachable.
func isDaemonRunning() bool {
	return getDaemonClient() != nil
}

// cmdContext returns the command's context, or a background context.
func cmdContext() context.Context {
	return context.Background()
}
