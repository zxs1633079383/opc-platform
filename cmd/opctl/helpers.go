package main

import (
	"context"
	"path/filepath"

	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"github.com/zlc-ai/opc-platform/pkg/adapter/openclaw"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/storage/sqlite"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

// getController creates a Controller with storage and adapter registry.
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

	logger := config.Logger
	if logger == nil {
		config.InitLogger(false)
		logger = config.Logger
	}

	ctrl := controller.New(store, registry, logger)

	cleanup := func() {
		store.Close()
	}

	return ctrl, cleanup, nil
}

// cmdContext returns the command's context, or a background context.
func cmdContext() context.Context {
	return context.Background()
}
