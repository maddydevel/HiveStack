# ADR-0002: gRPC for Node Agent Communication

## Status

Accepted

## Context

HiveStack has two primary processes:

1. **Manager** — Central control plane; serves REST API to clients, manages tenants, orchestrates VM lifecycle.
2. **Node Agent** — Runs on each hypervisor; executes VM commands (create, start, stop, migrate, snapshot), reports resource usage.

The Manager and Node Agent must communicate over the network with the following requirements:

- **Low latency** — Heartbeat and status messages must be fast.
- **Strongly typed contracts** — Commands and responses must be well-defined to avoid runtime errors.
- **Bidirectional streaming** — Future: log streaming, live migration progress, real-time metrics.
- **Mutual TLS** — All traffic must be encrypted and authenticated (see `internal/tls/`).

Options considered:

1. **REST + JSON over HTTP/2** — Simple, widely understood. No streaming support without WebSockets. Manual OpenAPI spec maintenance.
2. **gRPC with Protocol Buffors** — Strongly typed, code-generated client/server, native bidirectional streaming, pluggable auth (mTLS via credentials.TransportCredentials).
3. **Message queue (NATS, RabbitMQ)** — Decoupled, good for fan-out. Adds infrastructure dependency; complicates request/response patterns.

## Decision

Use **gRPC** for all Manager ↔ Node Agent communication.

- Service definition: `proto/node.proto` (`NodeAgent` service).
- Go code generated via `protoc-gen-go` and `protoc-gen-go-grpc`.
- mTLS authentication: Manager presents client certificate, Node presents server certificate. Both verified against HiveStack internal CA (`internal/tls/ca.go`).

The `internal/node/grpc.go` file implements the Manager-side gRPC client. The Node Agent implements the server side.

## Consequences

### Positive

- Code generation eliminates manual serialization/deserialization bugs.
- Bidirectional streaming available for future features (live migration progress, exec sessions).
- mTLS built into gRPC transport credentials — no custom auth layer needed.
- Efficient binary serialization (protobuf) reduces bandwidth vs. JSON.

### Negative

- Requires `protoc` toolchain in CI and developer environments.
- Debugging raw protobuf payloads is harder than JSON (mitigated by `grpcurl` and `grpc_cli`).
- Browser clients cannot directly call gRPC (mitigated by REST API on the Manager side).

## References

- `proto/node.proto` — gRPC service and message definitions.
- `internal/node/grpc.go` — Manager-side gRPC client.
- `internal/tls/grpc.go` — mTLS credential setup for gRPC.
