package a2a

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockKeyStore struct {
	validKeys map[string]bool
}

func (m *mockKeyStore) ValidateKey(key string) bool {
	return m.validKeys[key]
}

func TestHMACUnaryInterceptor_ValidKey(t *testing.T) {
	store := &mockKeyStore{validKeys: map[string]bool{"valid-key": true}}
	interceptor := HMACUnaryInterceptor(store)

	md := metadata.New(map[string]string{"x-opc-api-key": "valid-key"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return "ok", nil
	}

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
	if resp != "ok" {
		t.Fatalf("expected response 'ok', got %v", resp)
	}
}

func TestHMACUnaryInterceptor_InvalidKey(t *testing.T) {
	store := &mockKeyStore{validKeys: map[string]bool{"valid-key": true}}
	interceptor := HMACUnaryInterceptor(store)

	md := metadata.New(map[string]string{"x-opc-api-key": "wrong-key"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler should not be called for invalid key")
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestHMACUnaryInterceptor_MissingKey(t *testing.T) {
	store := &mockKeyStore{validKeys: map[string]bool{"valid-key": true}}
	interceptor := HMACUnaryInterceptor(store)

	// No metadata at all
	ctx := context.Background()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler should not be called when key is missing")
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestWithAPIKey(t *testing.T) {
	apiKey := "test-api-key"
	interceptor := WithAPIKey(apiKey)

	var capturedCtx context.Context
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtx = ctx
		return nil
	}

	ctx := context.Background()
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	if !ok {
		t.Fatal("expected outgoing metadata in context")
	}
	keys := md.Get("x-opc-api-key")
	if len(keys) == 0 {
		t.Fatal("expected x-opc-api-key in metadata")
	}
	if keys[0] != apiKey {
		t.Fatalf("expected key %q, got %q", apiKey, keys[0])
	}
}
