.PHONY: proto build test clean

proto:
	@mkdir -p gen/a2a gen/opc
	protoc \
		--proto_path=proto \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		proto/a2a/a2a.proto \
		proto/opc/types.proto \
		proto/opc/agent_service.proto \
		proto/opc/federation_service.proto

build:
	go build ./...

test:
	go test ./... -race -cover

clean:
	rm -rf gen/
