package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/model"
	"gopkg.in/yaml.v3"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create resources interactively",
}

var createAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Create an agent interactively with model selection and pricing info",
	Long: `Interactively create an agent by selecting type, model, and configuration.

Supported agent types:
  openclaw     — connects to OpenClaw gateway via WebSocket (configures gateway URL & token)
  claude-code  — spawns Claude CLI per task (select model with pricing details)
  codex        — spawns Codex CLI per task (select model with pricing details)`,
	RunE: runCreateAgent,
}

func init() {
	createCmd.AddCommand(createAgentCmd)
	rootCmd.AddCommand(createCmd)
}

// ──────────────────── interactive helpers ────────────────────

var stdinScanner *bufio.Scanner

func getScanner() *bufio.Scanner {
	if stdinScanner == nil {
		stdinScanner = bufio.NewScanner(os.Stdin)
	}
	return stdinScanner
}

func prompt(label, defaultVal string) string {
	s := getScanner()
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}
	s.Scan()
	val := strings.TrimSpace(s.Text())
	if val == "" {
		return defaultVal
	}
	return val
}

func promptRequired(label string) string {
	s := getScanner()
	for {
		fmt.Printf("  %s: ", label)
		s.Scan()
		val := strings.TrimSpace(s.Text())
		if val != "" {
			return val
		}
		fmt.Println("    (required, please enter a value)")
	}
}

func selectOne(label string, options []string) int {
	s := getScanner()
	fmt.Printf("\n  %s:\n", label)
	for i, opt := range options {
		fmt.Printf("    [%d] %s\n", i+1, opt)
	}
	for {
		fmt.Printf("  Select (1-%d): ", len(options))
		s.Scan()
		val := strings.TrimSpace(s.Text())
		n, err := strconv.Atoi(val)
		if err == nil && n >= 1 && n <= len(options) {
			return n - 1
		}
		fmt.Printf("    Invalid, please enter 1-%d\n", len(options))
	}
}

// ──────────────────── model display & selection ────────────────────

func showModelTable(models []model.ModelEntry) {
	fmt.Println()
	fmt.Printf("  %-3s %-24s %-8s %-8s %-12s %-12s %-8s %-9s %s\n",
		"#", "Model", "Context", "MaxOut", "In$/1M", "Out$/1M", "Cache$/1M", "Reason", "Input")
	fmt.Printf("  %s\n", strings.Repeat("─", 105))

	for i, m := range models {
		reasoning := " -"
		if m.Reasoning {
			reasoning = " yes"
		}
		inputTypes := make([]string, len(m.Input))
		for j, t := range m.Input {
			inputTypes[j] = string(t)
		}
		cache := " -"
		if m.Cost.CacheRead > 0 {
			cache = fmt.Sprintf("$%.2f", m.Cost.CacheRead)
		}
		fmt.Printf("  %-3d %-24s %-8s %-8s $%-11.2f $%-11.2f %-8s %-9s %s\n",
			i+1, m.Name, m.FormatContextWindow(), m.FormatMaxOutput(),
			m.Cost.Input, m.Cost.Output, cache, reasoning,
			strings.Join(inputTypes, ","))
	}
	fmt.Println()
}

// pickModel displays the model table and lets the user select one. Returns nil if skipped.
func pickModel(agentType string) *model.ModelEntry {
	models := model.ModelsForAgent(agentType)
	if len(models) == 0 {
		return nil
	}

	showModelTable(models)

	s := getScanner()
	for {
		fmt.Printf("  Select model (1-%d): ", len(models))
		s.Scan()
		val := strings.TrimSpace(s.Text())
		if val == "" {
			m := models[0]
			fmt.Printf("  ✓ Default: %s\n", m.Name)
			return &m
		}
		n, err := strconv.Atoi(val)
		if err == nil && n >= 1 && n <= len(models) {
			m := models[n-1]
			fmt.Printf("  ✓ Selected: %s  (In:$%.2f  Out:$%.2f per 1M tokens)\n",
				m.Name, m.Cost.Input, m.Cost.Output)
			return &m
		}
		fmt.Printf("    Invalid, enter 1-%d or press Enter for default\n", len(models))
	}
}

// pickFallback lets user optionally pick a fallback model, excluding the primary.
func pickFallback(agentType string, primaryID string) *model.ModelEntry {
	models := model.ModelsForAgent(agentType)
	var candidates []model.ModelEntry
	for _, m := range models {
		if m.ID != primaryID {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	fmt.Println("\n  Fallback model (used when primary fails):")
	for i, m := range candidates {
		fmt.Printf("    [%d] %-24s $%.2f / $%.2f per 1M\n", i+1, m.Name, m.Cost.Input, m.Cost.Output)
	}
	val := prompt("  Select fallback # (Enter to skip)", "")
	if val == "" {
		return nil
	}
	n, err := strconv.Atoi(val)
	if err == nil && n >= 1 && n <= len(candidates) {
		m := candidates[n-1]
		fmt.Printf("  ✓ Fallback: %s\n", m.Name)
		return &m
	}
	return nil
}

// ──────────────────── main flow ────────────────────

func runCreateAgent(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       OPC Create Agent Wizard        ║")
	fmt.Println("╚══════════════════════════════════════╝")

	// ── Step 1: Basic Info ──
	fmt.Println("\n─── Step 1: Basic Info ───")
	name := promptRequired("Agent name (e.g. my-coder)")
	description := prompt("Description (what does this agent do?)", "")
	role := prompt("Role label", "developer")

	// ── Step 2: Agent Type ──
	fmt.Println("\n─── Step 2: Agent Type ───")
	agentTypes := []string{
		"openclaw    — OpenClaw Gateway (WebSocket, persistent connection)",
		"claude-code — Claude Code CLI  (per-task process, local)",
		"codex       — Codex CLI        (per-task process, local)",
	}
	typeIdx := selectOne("Select agent type", agentTypes)
	agentTypeKeys := []v1.AgentType{v1.AgentTypeOpenClaw, v1.AgentTypeClaudeCode, v1.AgentTypeCodex}
	agentType := agentTypeKeys[typeIdx]
	fmt.Printf("  ✓ Type: %s\n", agentType)

	// ── Step 3: Type-specific Connection Config ──
	spec := v1.AgentSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindAgentSpec,
		Metadata:   v1.Metadata{Name: name, Labels: map[string]string{"role": role}},
		Spec: v1.AgentSpecBody{
			Type:        agentType,
			Description: description,
			Replicas:    1,
			Runtime: v1.RuntimeConfig{
				Timeout: v1.TimeoutConfig{Task: "600s", Idle: "1800s", Startup: "30s"},
			},
			Recovery:    v1.RecoveryConfig{Enabled: true, MaxRestarts: 3, RestartDelay: "15s", Backoff: "exponential"},
			HealthCheck: v1.HealthCheckConfig{Type: "heartbeat", Interval: "60s", Timeout: "10s", Retries: 3},
		},
	}

	switch agentType {
	case v1.AgentTypeOpenClaw:
		fmt.Println("\n─── Step 3: OpenClaw Gateway ───")
		gwURL := prompt("Gateway WebSocket URL", "ws://localhost:18789")
		gwToken := prompt("Gateway Token (optional, Enter to skip)", "")

		spec.Spec.Env = map[string]string{"OPENCLAW_GATEWAY_URL": gwURL}
		if gwToken != "" {
			spec.Spec.Env["OPENCLAW_GATEWAY_TOKEN"] = gwToken
		}
		spec.Spec.Protocol = v1.ProtocolConfig{Type: "websocket", Format: gwURL}
		fmt.Printf("  ✓ Gateway: %s\n", gwURL)
		if gwToken != "" {
			fmt.Printf("  ✓ Token:   %s...%s\n", gwToken[:4], gwToken[len(gwToken)-4:])
		} else {
			fmt.Println("  ✓ Token:   (none — will use env OPENCLAW_GATEWAY_TOKEN or ~/.openclaw/openclaw.json)")
		}

	case v1.AgentTypeClaudeCode:
		fmt.Println("\n─── Step 3: Claude Code ───")
		fmt.Println("  Claude Code runs locally via the `claude` CLI.")
		fmt.Println("  No connection config needed.")

	case v1.AgentTypeCodex:
		fmt.Println("\n─── Step 3: Codex ───")
		fmt.Println("  Codex runs locally via the `codex` CLI.")
		fmt.Println("  No connection config needed.")
	}

	// ── Step 4: Model Selection ──
	fmt.Printf("\n─── Step 4: Model Selection (%s) ───\n", agentType)
	selected := pickModel(string(agentType))
	if selected != nil {
		spec.Spec.Runtime.Model = v1.ModelConfig{
			Provider: selected.Provider,
			Name:     selected.CLIModelID,
		}
		// Fallback
		fb := pickFallback(string(agentType), selected.ID)
		if fb != nil {
			spec.Spec.Runtime.Model.Fallback = fb.CLIModelID
		}
	}

	// ── Step 5: Inference & Context ──
	fmt.Println("\n─── Step 5: Inference & Context ───")
	thinkingOpts := []string{"off", "low", "medium", "high"}
	thinkIdx := selectOne("Thinking level", thinkingOpts)
	spec.Spec.Runtime.Inference.Thinking = thinkingOpts[thinkIdx]

	workdir := prompt("Working directory", "/tmp/opc")
	spec.Spec.Context = v1.ContextConfig{Workdir: workdir}

	// ── Step 6: Preview ──
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	fmt.Println("\n─── Generated Agent YAML ───")
	fmt.Println(string(yamlBytes))

	// ── Step 7: Cost Estimate ──
	if selected != nil {
		fmt.Println("─── Cost Estimate ───")
		fmt.Printf("  Model:       %s (%s)\n", selected.Name, selected.Provider)
		fmt.Printf("  Input:       $%.2f per 1M tokens\n", selected.Cost.Input)
		fmt.Printf("  Output:      $%.2f per 1M tokens\n", selected.Cost.Output)
		if selected.Cost.CacheRead > 0 {
			fmt.Printf("  Cache read:  $%.2f per 1M tokens (%.0f%% of input)\n",
				selected.Cost.CacheRead, selected.Cost.CacheRead/selected.Cost.Input*100)
		}
		if selected.Cost.CacheWrite > 0 {
			fmt.Printf("  Cache write: $%.2f per 1M tokens\n", selected.Cost.CacheWrite)
		}
		fmt.Printf("  Context:     %s tokens\n", selected.FormatContextWindow())
		fmt.Printf("  Max output:  %s tokens\n", selected.FormatMaxOutput())
		est := float64(10_000)/1_000_000*selected.Cost.Input + float64(1_000)/1_000_000*selected.Cost.Output
		fmt.Printf("  ~Example:    10K in + 1K out ≈ $%.4f/task\n\n", est)
	}

	// ── Step 8: Confirm ──
	confirm := prompt("Apply this agent now? (y/n/save)", "y")

	switch strings.ToLower(confirm) {
	case "y", "yes":
		return applySpec(cmd, spec, name, yamlBytes)

	case "save", "s":
		outPath := prompt("Save YAML to", fmt.Sprintf("agent-%s.yaml", name))
		if err := os.WriteFile(outPath, yamlBytes, 0o644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("  ✓ Saved to %s\n", outPath)
		fmt.Printf("  Apply later: opctl apply -f %s\n", outPath)
		return nil

	default:
		fmt.Println("  Cancelled.")
		return nil
	}
}

func applySpec(cmd *cobra.Command, spec v1.AgentSpec, name string, yamlBytes []byte) error {
	// Try daemon first
	if c := getDaemonClient(); c != nil {
		msg, err := c.Apply(cmd.Context(), yamlBytes)
		if err != nil {
			return err
		}
		fmt.Println(msg)
		return nil
	}

	// Local mode
	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	if err := ctrl.Apply(context.Background(), spec); err != nil {
		return err
	}

	fmt.Printf("\n  ✓ agent/%s created successfully\n", name)
	fmt.Printf("  Start it: opctl run %s --message \"your task\"\n", name)
	return nil
}
