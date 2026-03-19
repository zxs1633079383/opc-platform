package openclaw

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

const (
	defaultGatewayURL = "ws://localhost:18789"
	protocolVersion   = 3
	clientID          = "gateway-client"
	clientVersion     = "0.4.0"
	clientPlatform    = "opc-platform"
	clientMode        = "backend"
	clientRole        = "operator"
)

var scopes = []string{"operator.read", "operator.write"}

// rpcMessage is the generic envelope for all WS messages.
type rpcMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Event   string          `json:"event,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"` // OpenClaw events use "payload" instead of "params"
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return e.Message }

// rpcResponse is delivered through pending channels.
type rpcResponse struct {
	Result json.RawMessage
	Error  *rpcError
}

// Adapter implements adapter.Adapter for OpenClaw agents via WebSocket.
type Adapter struct {
	mu          sync.RWMutex
	phase       v1.AgentPhase
	metrics     v1.AgentMetrics
	spec        v1.AgentSpec
	startAt     time.Time
	conn        *websocket.Conn
	reqID       atomic.Int64
	pending     map[string]chan *rpcResponse
	pendingMu   sync.Mutex
	connID      string
	stopCh      chan struct{}
	challengeCh chan json.RawMessage
	devicePub   ed25519.PublicKey
	devicePriv  ed25519.PrivateKey
	deviceID    string
	logger      *zap.SugaredLogger
}

// New creates a new OpenClaw adapter.
func New() adapter.Adapter {
	l, _ := zap.NewProduction()
	return &Adapter{
		phase:  v1.AgentPhaseCreated,
		logger: l.Sugar().Named("openclaw"),
	}
}

func (a *Adapter) Type() v1.AgentType {
	return v1.AgentTypeOpenClaw
}

// --- Identity management ---

// loadOrCreateIdentity loads or generates an ed25519 identity for the agent.
// Keys are stored at ~/.opc/identity/{agentName}-ed25519.key.
func loadOrCreateIdentity(agentName string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("get home dir: %w", err)
	}

	identityDir := filepath.Join(home, ".opc", "identity")
	if err := os.MkdirAll(identityDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create identity dir: %w", err)
	}

	keyPath := filepath.Join(identityDir, agentName+"-ed25519.key")

	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == ed25519.PrivateKeySize {
		priv := ed25519.PrivateKey(data)
		pub := priv.Public().(ed25519.PublicKey)
		return pub, priv, nil
	}

	// Generate new key pair.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	if err := os.WriteFile(keyPath, []byte(priv), 0o600); err != nil {
		return nil, nil, fmt.Errorf("write identity key: %w", err)
	}

	return pub, priv, nil
}

// --- Token resolution ---

// resolveGatewayToken returns the gateway token from spec env, OS env, or config file.
func resolveGatewayToken(spec v1.AgentSpec) string {
	// 1. spec.Spec.Env
	if t := spec.Spec.Env["OPENCLAW_GATEWAY_TOKEN"]; t != "" {
		return t
	}
	// 2. OS environment
	if t := os.Getenv("OPENCLAW_GATEWAY_TOKEN"); t != "" {
		return t
	}
	// 3. ~/.openclaw/openclaw.json
	return readOpenClawGatewayToken()
}

// readOpenClawGatewayToken reads the gateway token from ~/.openclaw/openclaw.json.
func readOpenClawGatewayToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".openclaw", "openclaw.json"))
	if err != nil {
		return ""
	}
	var config struct {
		Gateway struct {
			Auth struct {
				Token string `json:"token"`
			} `json:"auth"`
		} `json:"gateway"`
	}
	if json.Unmarshal(data, &config) == nil {
		return config.Gateway.Auth.Token
	}
	return ""
}

// resolveGatewayURL returns the gateway WebSocket URL.
func resolveGatewayURL(spec v1.AgentSpec) string {
	if u := spec.Spec.Env["OPENCLAW_GATEWAY_URL"]; u != "" {
		return u
	}
	if u := os.Getenv("OPENCLAW_GATEWAY_URL"); u != "" {
		return u
	}
	return defaultGatewayURL
}

// --- Lifecycle ---

func (a *Adapter) Start(ctx context.Context, spec v1.AgentSpec) error {
	start := time.Now()
	agentName := spec.Metadata.Name
	a.logger.Infow("Start", "agentName", agentName)
	a.mu.Lock()
	a.spec = spec
	a.phase = v1.AgentPhaseStarting
	a.mu.Unlock()

	// Load or create device identity.
	pub, priv, err := loadOrCreateIdentity(agentName)
	if err != nil {
		a.mu.Lock()
		a.phase = v1.AgentPhaseFailed
		a.mu.Unlock()
		a.logger.Errorw("Start: failed to load identity", "agentName", agentName, "error", err)
		return fmt.Errorf("load identity: %w", err)
	}

	a.mu.Lock()
	a.devicePub = pub
	a.devicePriv = priv
	hash := sha256.Sum256(pub)
	a.deviceID = hex.EncodeToString(hash[:])
	a.pending = make(map[string]chan *rpcResponse)
	a.stopCh = make(chan struct{})
	a.challengeCh = make(chan json.RawMessage, 1)
	a.mu.Unlock()

	// Dial WebSocket.
	gatewayURL := resolveGatewayURL(spec)
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	header := http.Header{}
	conn, _, err := dialer.DialContext(ctx, gatewayURL, header)
	if err != nil {
		a.mu.Lock()
		a.phase = v1.AgentPhaseFailed
		a.mu.Unlock()
		a.logger.Errorw("Start: ws dial failed", "agentName", agentName, "gatewayURL", gatewayURL, "error", err)
		return fmt.Errorf("ws dial %s: %w", gatewayURL, err)
	}
	a.logger.Infow("Start: ws connected", "agentName", agentName, "gatewayURL", gatewayURL)

	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()

	// Start readLoop BEFORE handshake (deadlock prevention: readLoop needs to
	// receive the challenge event before we can send connect).
	go a.readLoop()

	// Wait for challenge event.
	var challenge json.RawMessage
	select {
	case challenge = <-a.challengeCh:
	case <-ctx.Done():
		a.cleanup()
		return fmt.Errorf("waiting for challenge: %w", ctx.Err())
	case <-time.After(15 * time.Second):
		a.cleanup()
		return fmt.Errorf("timeout waiting for gateway challenge")
	}

	// Parse challenge nonce.
	var challengePayload struct {
		Nonce string `json:"nonce"`
		Ts    int64  `json:"ts"`
	}
	if err := json.Unmarshal(challenge, &challengePayload); err != nil {
		a.cleanup()
		return fmt.Errorf("parse challenge: %w", err)
	}

	// Send connect request.
	token := resolveGatewayToken(spec)
	resp, err := a.sendConnect(ctx, spec, challengePayload.Nonce, token)
	if err != nil {
		a.cleanup()
		a.logger.Errorw("Start: handshake failed", "agentName", agentName, "error", err)
		return fmt.Errorf("connect handshake: %w", err)
	}

	// Parse connID from response if available.
	// OpenClaw hello-ok response nests connId under "server" object.
	if resp.Result != nil {
		var helloResult struct {
			ConnID string `json:"connId"`
			Server struct {
				ConnID string `json:"connId"`
			} `json:"server"`
		}
		if json.Unmarshal(resp.Result, &helloResult) == nil {
			connID := helloResult.ConnID
			if connID == "" {
				connID = helloResult.Server.ConnID
			}
			if connID != "" {
				a.mu.Lock()
				a.connID = connID
				a.mu.Unlock()
			}
		}
	}

	a.mu.Lock()
	a.phase = v1.AgentPhaseRunning
	a.startAt = time.Now()
	a.mu.Unlock()

	a.logger.Infow("Start completed", "agentName", agentName, "connID", a.connID, "duration", time.Since(start))
	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	a.logger.Infow("Stop", "agentName", a.spec.Metadata.Name)
	a.mu.Lock()
	defer a.mu.Unlock()

	a.phase = v1.AgentPhaseStopped
	a.cleanupLocked()
	a.logger.Infow("Stop completed", "agentName", a.spec.Metadata.Name)
	return nil
}

func (a *Adapter) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cleanupLocked()
}

func (a *Adapter) cleanupLocked() {
	if a.stopCh != nil {
		select {
		case <-a.stopCh:
		default:
			close(a.stopCh)
		}
	}
	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
	// Drain all pending requests.
	a.pendingMu.Lock()
	for id, ch := range a.pending {
		ch <- &rpcResponse{Error: &rpcError{Message: "connection closed"}}
		delete(a.pending, id)
	}
	a.pendingMu.Unlock()
}

func (a *Adapter) Health() v1.HealthStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.phase == v1.AgentPhaseRunning && a.conn != nil {
		return v1.HealthStatus{Healthy: true, Message: "connected"}
	}
	return v1.HealthStatus{Healthy: false, Message: fmt.Sprintf("not running (phase: %s)", a.phase)}
}

func (a *Adapter) Status() v1.AgentPhase {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.phase
}

func (a *Adapter) Metrics() v1.AgentMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	m := a.metrics
	if !a.startAt.IsZero() {
		m.UptimeSeconds = time.Since(a.startAt).Seconds()
	}
	return m
}

// --- WebSocket read loop ---

func (a *Adapter) readLoop() {
	defer func() {
		a.mu.Lock()
		if a.phase == v1.AgentPhaseRunning {
			a.phase = v1.AgentPhaseFailed
		}
		a.mu.Unlock()
	}()

	for {
		select {
		case <-a.stopCh:
			return
		default:
		}

		a.mu.RLock()
		conn := a.conn
		a.mu.RUnlock()
		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			// Check if intentionally closed.
			select {
			case <-a.stopCh:
				return
			default:
			}
			a.logger.Warnw("readLoop: connection lost", "agentName", a.spec.Metadata.Name, "error", err)
			return
		}

		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "event":
			a.handleEvent(msg)
		case "res":
			a.handleResponse(msg)
		}
	}
}

func (a *Adapter) handleEvent(msg rpcMessage) {
	if msg.Event == "connect.challenge" {
		// OpenClaw sends challenge data in "payload"; fall back to "params" for compatibility.
		data := msg.Payload
		if len(data) == 0 {
			data = msg.Params
		}
		select {
		case a.challengeCh <- data:
		default:
		}
	}
}

func (a *Adapter) handleResponse(msg rpcMessage) {
	if msg.ID == "" {
		return
	}
	a.pendingMu.Lock()
	ch, ok := a.pending[msg.ID]
	if ok {
		delete(a.pending, msg.ID)
	}
	a.pendingMu.Unlock()

	if ok {
		// OpenClaw responses use "payload" for success data; fall back to "result" for compat.
		result := msg.Payload
		if len(result) == 0 {
			result = msg.Result
		}
		resp := &rpcResponse{
			Result: result,
			Error:  msg.Error,
		}
		select {
		case ch <- resp:
		default:
		}
	}
}

// --- RPC helpers ---

func (a *Adapter) nextID() string {
	return fmt.Sprintf("opc-%d", a.reqID.Add(1))
}

// sendRequest sends an RPC request and waits for the response.
func (a *Adapter) sendRequest(ctx context.Context, method string, params interface{}) (*rpcResponse, error) {
	id := a.nextID()

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	msg := rpcMessage{
		Type:   "req",
		ID:     id,
		Method: method,
		Params: paramsJSON,
	}

	ch := make(chan *rpcResponse, 1)
	a.pendingMu.Lock()
	a.pending[id] = ch
	a.pendingMu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		a.pendingMu.Lock()
		delete(a.pending, id)
		a.pendingMu.Unlock()
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	a.mu.RLock()
	conn := a.conn
	a.mu.RUnlock()
	if conn == nil {
		a.pendingMu.Lock()
		delete(a.pending, id)
		a.pendingMu.Unlock()
		return nil, fmt.Errorf("not connected")
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		a.pendingMu.Lock()
		delete(a.pending, id)
		a.pendingMu.Unlock()
		return nil, fmt.Errorf("write ws: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp, resp.Error
		}
		return resp, nil
	case <-ctx.Done():
		a.pendingMu.Lock()
		delete(a.pending, id)
		a.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

// --- Handshake ---

func (a *Adapter) sendConnect(ctx context.Context, spec v1.AgentSpec, nonce, token string) (*rpcResponse, error) {
	signedAt := time.Now().UnixMilli()

	// Build signature payload.
	scopeStr := strings.Join(scopes, ",")
	sigPayload := fmt.Sprintf("v3|%s|%s|%s|%s|%s|%d|%s|%s|%s|",
		a.deviceID, clientID, clientMode, clientRole, scopeStr,
		signedAt, token, nonce, clientPlatform,
	)

	sig := ed25519.Sign(a.devicePriv, []byte(sigPayload))

	displayName := fmt.Sprintf("OPC Agent: %s", spec.Metadata.Name)

	params := map[string]interface{}{
		"minProtocol": protocolVersion,
		"maxProtocol": protocolVersion,
		"client": map[string]interface{}{
			"id":          clientID,
			"displayName": displayName,
			"version":     clientVersion,
			"platform":    clientPlatform,
			"mode":        clientMode,
		},
		"role":   clientRole,
		"scopes": scopes,
		"device": map[string]interface{}{
			"id":        a.deviceID,
			"publicKey": base64.RawURLEncoding.EncodeToString(a.devicePub),
			"signature": base64.RawURLEncoding.EncodeToString(sig),
			"signedAt":  signedAt,
			"nonce":     nonce,
		},
		"auth": map[string]interface{}{
			"token": token,
		},
	}

	return a.sendRequest(ctx, "connect", params)
}

// --- Execute ---

func (a *Adapter) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	execStart := time.Now()
	a.logger.Infow("Execute", "taskId", task.ID, "agentName", a.spec.Metadata.Name)
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		a.logger.Warnw("Execute: agent not running", "taskId", task.ID, "phase", a.phase)
		return adapter.ExecuteResult{}, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	// Send agent request.
	sessionKey := fmt.Sprintf("opc-%s-%s", a.spec.Metadata.Name, task.ID)
	idempotencyKey := fmt.Sprintf("opc-task-%s", task.ID)

	params := map[string]interface{}{
		"message":        task.Message,
		"sessionKey":     sessionKey,
		"idempotencyKey": idempotencyKey,
	}

	resp, err := a.sendRequest(ctx, "agent", params)
	if err != nil {
		a.mu.Lock()
		a.metrics.TasksFailed++
		a.mu.Unlock()
		a.logger.Errorw("Execute: agent request failed", "taskId", task.ID, "error", err, "duration", time.Since(execStart))
		return adapter.ExecuteResult{}, fmt.Errorf("openclaw agent request: %w", err)
	}

	// Check if the response is "accepted" (async) or immediate result.
	var parsed map[string]interface{}
	if resp.Result != nil {
		if err := json.Unmarshal(resp.Result, &parsed); err == nil {
			if runID, ok := parsed["runId"].(string); ok && runID != "" {
				// Async: wait for completion.
				resp, err = a.waitForRun(ctx, runID)
				if err != nil {
					a.mu.Lock()
					a.metrics.TasksFailed++
					a.mu.Unlock()
					return adapter.ExecuteResult{}, fmt.Errorf("openclaw agent.wait: %w", err)
				}
				parsed = nil
				if resp.Result != nil {
					json.Unmarshal(resp.Result, &parsed)
				}
			}
		}
	}

	result := extractResult(parsed)

	a.mu.Lock()
	a.metrics.TasksCompleted++
	a.metrics.TotalTokensIn += result.TokensIn
	a.metrics.TotalTokensOut += result.TokensOut
	a.mu.Unlock()

	a.logger.Infow("Execute completed", "taskId", task.ID,
		"tokensIn", result.TokensIn, "tokensOut", result.TokensOut,
		"cost", result.Cost, "duration", time.Since(execStart))
	return result, nil
}

// waitForRun polls agent.wait until the run completes.
func (a *Adapter) waitForRun(ctx context.Context, runID string) (*rpcResponse, error) {
	params := map[string]interface{}{
		"runId": runID,
	}
	return a.sendRequest(ctx, "agent.wait", params)
}

// extractResult pulls output text and token counts from a response map.
func extractResult(parsed map[string]interface{}) adapter.ExecuteResult {
	var result adapter.ExecuteResult
	if parsed == nil {
		return result
	}

	// Try multiple fields for output text.
	for _, key := range []string{"text", "message", "result", "summary", "content", "output"} {
		if v, ok := parsed[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				result.Output = s
				break
			}
		}
	}

	// Token counts.
	if v, ok := parsed["tokensIn"]; ok {
		if f, ok := v.(float64); ok {
			result.TokensIn = int(f)
		}
	}
	if v, ok := parsed["tokensOut"]; ok {
		if f, ok := v.(float64); ok {
			result.TokensOut = int(f)
		}
	}
	if v, ok := parsed["inputTokens"]; ok {
		if f, ok := v.(float64); ok {
			result.TokensIn = int(f)
		}
	}
	if v, ok := parsed["outputTokens"]; ok {
		if f, ok := v.(float64); ok {
			result.TokensOut = int(f)
		}
	}

	// Cost.
	if v, ok := parsed["cost"]; ok {
		if f, ok := v.(float64); ok {
			result.Cost = f
		}
	}
	if v, ok := parsed["totalCost"]; ok {
		if f, ok := v.(float64); ok {
			result.Cost = f
		}
	}

	return result
}

// --- Stream ---

func (a *Adapter) Stream(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return nil, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	// For WebSocket protocol, streaming uses the same agent request.
	// We execute synchronously and deliver the result as a single chunk.
	ch := make(chan adapter.Chunk, 1)

	go func() {
		defer close(ch)

		result, err := a.Execute(ctx, task)
		if err != nil {
			ch <- adapter.Chunk{Error: err}
			return
		}

		ch <- adapter.Chunk{Content: result.Output, Done: true}
	}()

	return ch, nil
}
