package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/gateway"
	"github.com/zlc-ai/opc-platform/pkg/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the OPC daemon",
	Long: `Start the OPC Platform daemon process.

The daemon holds the controller in memory, manages agent lifecycles,
and exposes an HTTP API so other opctl commands can communicate with it.

All opctl commands automatically detect a running daemon and route
operations through it instead of creating ephemeral controllers.`,
	RunE: runServe,
}

var (
	servePort int
	serveHost string
)

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 9527, "HTTP listen port")
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "HTTP listen host")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	config.InitLogger(verbose)
	logger := config.Logger

	// Create controller (persistent for daemon lifetime).
	ctrl, cleanup, err := getController()
	if err != nil {
		return fmt.Errorf("init controller: %w", err)
	}
	defer cleanup()

	// Write PID file so CLI can detect us.
	pidPath := filepath.Join(config.GetConfigDir(), "daemon.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644); err != nil {
		logger.Warnw("could not write pid file", "error", err)
	}
	defer os.Remove(pidPath)

	// Write address file so CLI knows our port.
	addrPath := filepath.Join(config.GetConfigDir(), "daemon.addr")
	addr := fmt.Sprintf("http://%s:%d", serveHost, servePort)
	if err := os.WriteFile(addrPath, []byte(addr), 0o644); err != nil {
		logger.Warnw("could not write addr file", "error", err)
	}
	defer os.Remove(addrPath)

	// Create gateway (no channels configured by default).
	gw := gateway.New(logger)

	// Start health check loop.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctrl.StartHealthCheckLoop(ctx)

	// Start checkpoint loop (every 5 minutes).
	// ctrl.StartCheckpointLoop(ctx, 5*time.Minute)

	// Start HTTP server.
	srv := server.New(ctrl, nil, gw, server.Config{
		Port: servePort,
		Host: serveHost,
	}, logger)

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	fmt.Printf("OPC daemon running on %s\n", addr)
	fmt.Println("Press Ctrl+C to stop.")

	// Wait for signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	fmt.Printf("\nReceived %s, shutting down...\n", sig)
	cancel()

	if err := srv.Stop(context.Background()); err != nil {
		logger.Warnw("error stopping server", "error", err)
	}

	fmt.Println("Daemon stopped.")
	return nil
}
