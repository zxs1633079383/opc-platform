package openclaw

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// ---------------------------------------------------------------------------
// 1. loadOrCreateIdentity
// ---------------------------------------------------------------------------

func TestLoadOrCreateIdentity_GeneratesNewKeyPair(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	pub, priv, err := loadOrCreateIdentity("test-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("public key size = %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("private key size = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}

	// Verify the key file was written.
	keyPath := filepath.Join(tmp, ".opc", "identity", "test-agent-ed25519.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}
	if len(data) != ed25519.PrivateKeySize {
		t.Fatalf("persisted key size = %d, want %d", len(data), ed25519.PrivateKeySize)
	}
}

func TestLoadOrCreateIdentity_LoadsExistingKey(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Generate first.
	pub1, priv1, err := loadOrCreateIdentity("reload-agent")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Load again — should return the same key pair.
	pub2, priv2, err := loadOrCreateIdentity("reload-agent")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if !pub1.Equal(pub2) {
		t.Fatal("public keys differ after reload")
	}
	if !priv1.Equal(priv2) {
		t.Fatal("private keys differ after reload")
	}
}

func TestLoadOrCreateIdentity_PersistedKeyMatches(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	pub, priv, err := loadOrCreateIdentity("persist-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read raw key bytes and reconstruct.
	keyPath := filepath.Join(tmp, ".opc", "identity", "persist-agent-ed25519.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	loadedPriv := ed25519.PrivateKey(data)
	loadedPub := loadedPriv.Public().(ed25519.PublicKey)

	if !pub.Equal(loadedPub) {
		t.Fatal("persisted public key does not match")
	}
	if !priv.Equal(loadedPriv) {
		t.Fatal("persisted private key does not match")
	}
}

// ---------------------------------------------------------------------------
// 2. resolveGatewayURL
// ---------------------------------------------------------------------------

func TestResolveGatewayURL_FromSpecEnv(t *testing.T) {
	spec := v1.AgentSpec{
		Spec: v1.AgentSpecBody{
			Env: map[string]string{"OPENCLAW_GATEWAY_URL": "ws://spec:9999"},
		},
	}
	got := resolveGatewayURL(spec)
	if got != "ws://spec:9999" {
		t.Fatalf("got %q, want ws://spec:9999", got)
	}
}

func TestResolveGatewayURL_FromOSEnv(t *testing.T) {
	origVal := os.Getenv("OPENCLAW_GATEWAY_URL")
	os.Setenv("OPENCLAW_GATEWAY_URL", "ws://osenv:8888")
	defer os.Setenv("OPENCLAW_GATEWAY_URL", origVal)

	spec := v1.AgentSpec{Spec: v1.AgentSpecBody{Env: map[string]string{}}}
	got := resolveGatewayURL(spec)
	if got != "ws://osenv:8888" {
		t.Fatalf("got %q, want ws://osenv:8888", got)
	}
}

func TestResolveGatewayURL_DefaultFallback(t *testing.T) {
	origVal := os.Getenv("OPENCLAW_GATEWAY_URL")
	os.Unsetenv("OPENCLAW_GATEWAY_URL")
	defer os.Setenv("OPENCLAW_GATEWAY_URL", origVal)

	spec := v1.AgentSpec{Spec: v1.AgentSpecBody{Env: map[string]string{}}}
	got := resolveGatewayURL(spec)
	if got != defaultGatewayURL {
		t.Fatalf("got %q, want %q", got, defaultGatewayURL)
	}
}

// ---------------------------------------------------------------------------
// 3. resolveGatewayToken
// ---------------------------------------------------------------------------

func TestResolveGatewayToken_FromSpecEnv(t *testing.T) {
	spec := v1.AgentSpec{
		Spec: v1.AgentSpecBody{
			Env: map[string]string{"OPENCLAW_GATEWAY_TOKEN": "spec-token"},
		},
	}
	got := resolveGatewayToken(spec)
	if got != "spec-token" {
		t.Fatalf("got %q, want spec-token", got)
	}
}

func TestResolveGatewayToken_FromOSEnv(t *testing.T) {
	origVal := os.Getenv("OPENCLAW_GATEWAY_TOKEN")
	os.Setenv("OPENCLAW_GATEWAY_TOKEN", "os-token")
	defer os.Setenv("OPENCLAW_GATEWAY_TOKEN", origVal)

	spec := v1.AgentSpec{Spec: v1.AgentSpecBody{Env: map[string]string{}}}
	got := resolveGatewayToken(spec)
	if got != "os-token" {
		t.Fatalf("got %q, want os-token", got)
	}
}

func TestResolveGatewayToken_FromConfigFile(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	origToken := os.Getenv("OPENCLAW_GATEWAY_TOKEN")
	os.Unsetenv("OPENCLAW_GATEWAY_TOKEN")
	defer os.Setenv("OPENCLAW_GATEWAY_TOKEN", origToken)

	// Write config file.
	configDir := filepath.Join(tmp, ".openclaw")
	os.MkdirAll(configDir, 0o755)
	configData := `{"gateway":{"auth":{"token":"file-token"}}}`
	os.WriteFile(filepath.Join(configDir, "openclaw.json"), []byte(configData), 0o644)

	spec := v1.AgentSpec{Spec: v1.AgentSpecBody{Env: map[string]string{}}}
	got := resolveGatewayToken(spec)
	if got != "file-token" {
		t.Fatalf("got %q, want file-token", got)
	}
}

func TestResolveGatewayToken_NoTokenReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	origToken := os.Getenv("OPENCLAW_GATEWAY_TOKEN")
	os.Unsetenv("OPENCLAW_GATEWAY_TOKEN")
	defer os.Setenv("OPENCLAW_GATEWAY_TOKEN", origToken)

	spec := v1.AgentSpec{Spec: v1.AgentSpecBody{Env: map[string]string{}}}
	got := resolveGatewayToken(spec)
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// 4. readOpenClawGatewayToken
// ---------------------------------------------------------------------------

func TestReadOpenClawGatewayToken_ValidConfig(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmp, ".openclaw")
	os.MkdirAll(configDir, 0o755)
	configData := `{"gateway":{"auth":{"token":"my-secret"}}}`
	os.WriteFile(filepath.Join(configDir, "openclaw.json"), []byte(configData), 0o644)

	got := readOpenClawGatewayToken()
	if got != "my-secret" {
		t.Fatalf("got %q, want my-secret", got)
	}
}

func TestReadOpenClawGatewayToken_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	got := readOpenClawGatewayToken()
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestReadOpenClawGatewayToken_MalformedJSON(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmp, ".openclaw")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "openclaw.json"), []byte("{invalid json"), 0o644)

	got := readOpenClawGatewayToken()
	if got != "" {
		t.Fatalf("got %q, want empty for malformed JSON", got)
	}
}

// ---------------------------------------------------------------------------
// 5. extractResult
// ---------------------------------------------------------------------------

func TestExtractResult_TextFields(t *testing.T) {
	fields := []string{"text", "message", "result", "summary", "content", "output"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			m := map[string]interface{}{field: "hello from " + field}
			r := extractResult(m)
			want := "hello from " + field
			if r.Output != want {
				t.Fatalf("got %q, want %q", r.Output, want)
			}
		})
	}
}

func TestExtractResult_TextFieldPriority(t *testing.T) {
	// "text" should win over "output" because it's checked first.
	m := map[string]interface{}{"text": "first", "output": "second"}
	r := extractResult(m)
	if r.Output != "first" {
		t.Fatalf("got %q, want 'first' (text has priority)", r.Output)
	}
}

func TestExtractResult_TokensInOut(t *testing.T) {
	m := map[string]interface{}{"tokensIn": float64(100), "tokensOut": float64(200)}
	r := extractResult(m)
	if r.TokensIn != 100 {
		t.Fatalf("TokensIn = %d, want 100", r.TokensIn)
	}
	if r.TokensOut != 200 {
		t.Fatalf("TokensOut = %d, want 200", r.TokensOut)
	}
}

func TestExtractResult_InputOutputTokens(t *testing.T) {
	m := map[string]interface{}{"inputTokens": float64(50), "outputTokens": float64(75)}
	r := extractResult(m)
	if r.TokensIn != 50 {
		t.Fatalf("TokensIn = %d, want 50", r.TokensIn)
	}
	if r.TokensOut != 75 {
		t.Fatalf("TokensOut = %d, want 75", r.TokensOut)
	}
}

func TestExtractResult_Cost(t *testing.T) {
	m := map[string]interface{}{"cost": float64(0.05)}
	r := extractResult(m)
	if r.Cost != 0.05 {
		t.Fatalf("Cost = %f, want 0.05", r.Cost)
	}
}

func TestExtractResult_TotalCost(t *testing.T) {
	m := map[string]interface{}{"totalCost": float64(1.23)}
	r := extractResult(m)
	if r.Cost != 1.23 {
		t.Fatalf("Cost = %f, want 1.23", r.Cost)
	}
}

func TestExtractResult_NilMap(t *testing.T) {
	r := extractResult(nil)
	if r.Output != "" || r.TokensIn != 0 || r.TokensOut != 0 || r.Cost != 0 {
		t.Fatalf("expected zero-value result for nil map, got %+v", r)
	}
}

func TestExtractResult_EmptyMap(t *testing.T) {
	r := extractResult(map[string]interface{}{})
	if r.Output != "" || r.TokensIn != 0 || r.TokensOut != 0 || r.Cost != 0 {
		t.Fatalf("expected zero-value result for empty map, got %+v", r)
	}
}

// ---------------------------------------------------------------------------
// 6. handleEvent
// ---------------------------------------------------------------------------

func TestHandleEvent_ChallengeDelivered(t *testing.T) {
	a := &Adapter{
		challengeCh: make(chan json.RawMessage, 1),
	}

	params := json.RawMessage(`{"nonce":"abc123"}`)
	msg := rpcMessage{Type: "event", Event: "connect.challenge", Params: params}
	a.handleEvent(msg)

	select {
	case got := <-a.challengeCh:
		if string(got) != string(params) {
			t.Fatalf("got %s, want %s", got, params)
		}
	default:
		t.Fatal("challenge not delivered")
	}
}

func TestHandleEvent_NonChallengeIgnored(t *testing.T) {
	a := &Adapter{
		challengeCh: make(chan json.RawMessage, 1),
	}

	msg := rpcMessage{Type: "event", Event: "some.other.event", Params: json.RawMessage(`{}`)}
	a.handleEvent(msg)

	select {
	case <-a.challengeCh:
		t.Fatal("non-challenge event should not be delivered")
	default:
		// expected
	}
}

// ---------------------------------------------------------------------------
// 7. handleResponse
// ---------------------------------------------------------------------------

func TestHandleResponse_MatchingID(t *testing.T) {
	a := &Adapter{
		pending: make(map[string]chan *rpcResponse),
	}

	ch := make(chan *rpcResponse, 1)
	a.pending["req-1"] = ch

	msg := rpcMessage{
		Type:   "res",
		ID:     "req-1",
		Result: json.RawMessage(`{"ok":true}`),
	}
	a.handleResponse(msg)

	select {
	case resp := <-ch:
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
		if string(resp.Result) != `{"ok":true}` {
			t.Fatalf("result = %s, want {\"ok\":true}", resp.Result)
		}
	default:
		t.Fatal("response not delivered")
	}

	// Verify the pending entry was removed.
	a.pendingMu.Lock()
	if _, ok := a.pending["req-1"]; ok {
		t.Fatal("pending entry should have been removed")
	}
	a.pendingMu.Unlock()
}

func TestHandleResponse_EmptyID(t *testing.T) {
	a := &Adapter{
		pending: make(map[string]chan *rpcResponse),
	}

	ch := make(chan *rpcResponse, 1)
	a.pending["req-1"] = ch

	msg := rpcMessage{Type: "res", ID: ""}
	a.handleResponse(msg)

	select {
	case <-ch:
		t.Fatal("should not deliver for empty ID")
	default:
		// expected
	}
}

func TestHandleResponse_UnknownID(t *testing.T) {
	a := &Adapter{
		pending: make(map[string]chan *rpcResponse),
	}

	ch := make(chan *rpcResponse, 1)
	a.pending["req-1"] = ch

	msg := rpcMessage{Type: "res", ID: "unknown-id", Result: json.RawMessage(`{}`)}
	a.handleResponse(msg)

	// The existing channel should not receive anything.
	select {
	case <-ch:
		t.Fatal("should not deliver for unknown ID")
	default:
		// expected
	}
}

// ---------------------------------------------------------------------------
// 8. nextID
// ---------------------------------------------------------------------------

func TestNextID_Sequential(t *testing.T) {
	a := &Adapter{}
	id1 := a.nextID()
	id2 := a.nextID()
	id3 := a.nextID()

	if id1 != "opc-1" {
		t.Fatalf("id1 = %q, want opc-1", id1)
	}
	if id2 != "opc-2" {
		t.Fatalf("id2 = %q, want opc-2", id2)
	}
	if id3 != "opc-3" {
		t.Fatalf("id3 = %q, want opc-3", id3)
	}
}

func TestNextID_ConcurrentSafety(t *testing.T) {
	a := &Adapter{}
	const n = 100
	var wg sync.WaitGroup
	ids := make([]string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids[idx] = a.nextID()
		}(i)
	}
	wg.Wait()

	// All IDs must be unique.
	seen := make(map[string]bool, n)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("expected %d unique IDs, got %d", n, len(seen))
	}
}

// ---------------------------------------------------------------------------
// 9. WebSocket integration test
// ---------------------------------------------------------------------------

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// mockGateway simulates the OpenClaw gateway WS protocol.
// It sends a challenge, validates the connect request, then handles agent requests.
func mockGateway(t *testing.T, nonce string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		// 1. Send challenge event.
		challenge := rpcMessage{
			Type:   "event",
			Event:  "connect.challenge",
			Params: mustJSON(t, map[string]interface{}{"nonce": nonce, "ts": time.Now().UnixMilli()}),
		}
		if err := conn.WriteJSON(challenge); err != nil {
			t.Errorf("write challenge: %v", err)
			return
		}

		// 2. Receive connect request.
		var connectReq rpcMessage
		if err := conn.ReadJSON(&connectReq); err != nil {
			t.Errorf("read connect: %v", err)
			return
		}

		if connectReq.Method != "connect" {
			t.Errorf("expected method 'connect', got %q", connectReq.Method)
			return
		}

		// Validate device signature fields are present.
		var params map[string]interface{}
		if err := json.Unmarshal(connectReq.Params, &params); err != nil {
			t.Errorf("unmarshal connect params: %v", err)
			return
		}
		device, ok := params["device"].(map[string]interface{})
		if !ok {
			t.Error("missing device in connect params")
			return
		}
		for _, field := range []string{"id", "publicKey", "signature", "signedAt", "nonce"} {
			if _, ok := device[field]; !ok {
				t.Errorf("missing device field: %s", field)
			}
		}

		// Verify nonce matches.
		if device["nonce"] != nonce {
			t.Errorf("nonce = %v, want %s", device["nonce"], nonce)
		}

		// Send hello-ok response.
		helloResp := rpcMessage{
			Type:   "res",
			ID:     connectReq.ID,
			Result: mustJSON(t, map[string]interface{}{"connId": "test-conn-123", "status": "ok"}),
		}
		if err := conn.WriteJSON(helloResp); err != nil {
			t.Errorf("write hello: %v", err)
			return
		}

		// 3. Receive agent request and respond with result.
		var agentReq rpcMessage
		if err := conn.ReadJSON(&agentReq); err != nil {
			// May be closed before sending agent request (e.g., Stop test).
			return
		}

		if agentReq.Method != "agent" {
			t.Errorf("expected method 'agent', got %q", agentReq.Method)
			return
		}

		agentResp := rpcMessage{
			Type: "res",
			ID:   agentReq.ID,
			Result: mustJSON(t, map[string]interface{}{
				"text":      "task completed successfully",
				"tokensIn":  float64(150),
				"tokensOut": float64(300),
				"cost":      float64(0.02),
			}),
		}
		if err := conn.WriteJSON(agentResp); err != nil {
			t.Errorf("write agent response: %v", err)
			return
		}

		// Keep connection open until client disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}
}

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func TestWebSocket_FullLifecycle(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Clear OS env so spec env is used.
	origURL := os.Getenv("OPENCLAW_GATEWAY_URL")
	os.Unsetenv("OPENCLAW_GATEWAY_URL")
	defer os.Setenv("OPENCLAW_GATEWAY_URL", origURL)

	origToken := os.Getenv("OPENCLAW_GATEWAY_TOKEN")
	os.Unsetenv("OPENCLAW_GATEWAY_TOKEN")
	defer os.Setenv("OPENCLAW_GATEWAY_TOKEN", origToken)

	nonce := "test-nonce-42"
	srv := httptest.NewServer(mockGateway(t, nonce))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	a := New()

	spec := v1.AgentSpec{
		Metadata: v1.Metadata{Name: "ws-test-agent"},
		Spec: v1.AgentSpecBody{
			Type: v1.AgentTypeOpenClaw,
			Env: map[string]string{
				"OPENCLAW_GATEWAY_URL":   wsURL,
				"OPENCLAW_GATEWAY_TOKEN": "test-token",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start
	if err := a.Start(ctx, spec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if a.Status() != v1.AgentPhaseRunning {
		t.Fatalf("phase = %s, want Running", a.Status())
	}

	health := a.Health()
	if !health.Healthy {
		t.Fatalf("health = %v, want healthy", health)
	}

	// Verify connID was set.
	oa := a.(*Adapter)
	oa.mu.RLock()
	connID := oa.connID
	oa.mu.RUnlock()
	if connID != "test-conn-123" {
		t.Fatalf("connID = %q, want test-conn-123", connID)
	}

	// Verify device identity was created.
	oa.mu.RLock()
	deviceID := oa.deviceID
	pubKey := oa.devicePub
	oa.mu.RUnlock()

	expectedHash := sha256.Sum256(pubKey)
	expectedDeviceID := hex.EncodeToString(expectedHash[:])
	if deviceID != expectedDeviceID {
		t.Fatalf("deviceID mismatch")
	}

	// Execute
	task := v1.TaskRecord{
		ID:      "task-001",
		Message: "do something useful",
	}
	result, err := a.Execute(ctx, task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Output != "task completed successfully" {
		t.Fatalf("output = %q, want 'task completed successfully'", result.Output)
	}
	if result.TokensIn != 150 {
		t.Fatalf("TokensIn = %d, want 150", result.TokensIn)
	}
	if result.TokensOut != 300 {
		t.Fatalf("TokensOut = %d, want 300", result.TokensOut)
	}
	if result.Cost != 0.02 {
		t.Fatalf("Cost = %f, want 0.02", result.Cost)
	}

	// Verify metrics updated.
	metrics := a.Metrics()
	if metrics.TasksCompleted != 1 {
		t.Fatalf("TasksCompleted = %d, want 1", metrics.TasksCompleted)
	}
	if metrics.TotalTokensIn != 150 {
		t.Fatalf("TotalTokensIn = %d, want 150", metrics.TotalTokensIn)
	}
	if metrics.UptimeSeconds <= 0 {
		t.Fatal("UptimeSeconds should be > 0")
	}

	// Stop
	if err := a.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if a.Status() != v1.AgentPhaseStopped {
		t.Fatalf("phase = %s, want Stopped", a.Status())
	}

	health = a.Health()
	if health.Healthy {
		t.Fatal("should not be healthy after stop")
	}
}

func TestExecute_NotRunning(t *testing.T) {
	a := New()
	task := v1.TaskRecord{ID: "t1", Message: "hello"}
	_, err := a.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when agent not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("error = %v, want 'not running'", err)
	}
}

func TestStream_NotRunning(t *testing.T) {
	a := New()
	task := v1.TaskRecord{ID: "t1", Message: "hello"}
	_, err := a.Stream(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when agent not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("error = %v, want 'not running'", err)
	}
}

func TestType_ReturnsOpenClaw(t *testing.T) {
	a := New()
	if a.Type() != v1.AgentTypeOpenClaw {
		t.Fatalf("Type() = %s, want %s", a.Type(), v1.AgentTypeOpenClaw)
	}
}

func TestNew_InitialPhase(t *testing.T) {
	a := New()
	if a.Status() != v1.AgentPhaseCreated {
		t.Fatalf("initial phase = %s, want Created", a.Status())
	}
}

// Verify the adapter satisfies the adapter.Adapter interface at compile time.
var _ adapter.Adapter = (*Adapter)(nil)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func TestRpcError_ErrorString(t *testing.T) {
	e := &rpcError{Code: 42, Message: "something went wrong"}
	got := e.Error()
	if got != "something went wrong" {
		t.Fatalf("Error() = %q, want 'something went wrong'", got)
	}
}

func TestExtractResult_CombinedFields(t *testing.T) {
	m := map[string]interface{}{
		"content":      "combined output",
		"inputTokens":  float64(10),
		"outputTokens": float64(20),
		"totalCost":    float64(0.99),
	}
	r := extractResult(m)
	if r.Output != "combined output" {
		t.Fatalf("Output = %q, want 'combined output'", r.Output)
	}
	if r.TokensIn != 10 || r.TokensOut != 20 {
		t.Fatalf("tokens = %d/%d, want 10/20", r.TokensIn, r.TokensOut)
	}
	if r.Cost != 0.99 {
		t.Fatalf("Cost = %f, want 0.99", r.Cost)
	}
}

func TestHandleEvent_FullChannelDrops(t *testing.T) {
	// Channel is full (buffer 1, already has one item).
	a := &Adapter{
		challengeCh: make(chan json.RawMessage, 1),
	}
	a.challengeCh <- json.RawMessage(`{"nonce":"first"}`)

	// Second challenge should be dropped (not block).
	msg := rpcMessage{Type: "event", Event: "connect.challenge", Params: json.RawMessage(`{"nonce":"second"}`)}
	done := make(chan struct{})
	go func() {
		a.handleEvent(msg)
		close(done)
	}()

	select {
	case <-done:
		// Good, didn't block.
	case <-time.After(1 * time.Second):
		t.Fatal("handleEvent blocked on full channel")
	}

	// The first challenge should still be there.
	got := <-a.challengeCh
	if string(got) != `{"nonce":"first"}` {
		t.Fatalf("got %s, want first nonce", got)
	}
}

func TestHandleResponse_WithError(t *testing.T) {
	a := &Adapter{
		pending: make(map[string]chan *rpcResponse),
	}
	ch := make(chan *rpcResponse, 1)
	a.pending["err-1"] = ch

	msg := rpcMessage{
		Type:  "res",
		ID:    "err-1",
		Error: &rpcError{Code: 500, Message: "internal error"},
	}
	a.handleResponse(msg)

	select {
	case resp := <-ch:
		if resp.Error == nil {
			t.Fatal("expected error in response")
		}
		if resp.Error.Message != "internal error" {
			t.Fatalf("error = %q, want 'internal error'", resp.Error.Message)
		}
	default:
		t.Fatal("response not delivered")
	}
}

func TestNextID_Format(t *testing.T) {
	a := &Adapter{}
	for i := 1; i <= 5; i++ {
		id := a.nextID()
		expected := fmt.Sprintf("opc-%d", i)
		if id != expected {
			t.Fatalf("nextID() = %q, want %q", id, expected)
		}
	}
}
