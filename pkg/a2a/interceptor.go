package a2a

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const metadataKeyAPIKey = "x-opc-api-key"

// APIKeyStore validates API keys for federation authentication.
type APIKeyStore interface {
	ValidateKey(key string) bool
}

// HMACUnaryInterceptor returns a gRPC unary server interceptor that validates
// the API key from incoming metadata against the provided store.
func HMACUnaryInterceptor(store APIKeyStore) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := validateAPIKey(ctx, store); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// HMACStreamInterceptor returns a gRPC stream server interceptor that validates
// the API key from incoming metadata against the provided store.
func HMACStreamInterceptor(store APIKeyStore) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := validateAPIKey(ss.Context(), store); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// WithAPIKey returns a gRPC unary client interceptor that attaches the given
// API key to outgoing metadata on every call.
func WithAPIKey(apiKey string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = metadata.AppendToOutgoingContext(ctx, metadataKeyAPIKey, apiKey)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// validateAPIKey extracts the API key from incoming gRPC metadata and validates
// it against the store. Returns a gRPC Unauthenticated error if missing or invalid.
func validateAPIKey(ctx context.Context, store APIKeyStore) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	keys := md.Get(metadataKeyAPIKey)
	if len(keys) == 0 {
		return status.Error(codes.Unauthenticated, "missing API key")
	}

	if !store.ValidateKey(keys[0]) {
		return status.Error(codes.Unauthenticated, "invalid API key")
	}

	return nil
}
