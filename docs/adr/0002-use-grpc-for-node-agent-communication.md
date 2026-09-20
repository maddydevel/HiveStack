# ADR-0002: Use gRPC for node-agent communication

- **Status:** Accepted
- **Date:** 2026-09-20 (recorded retroactively; decision made during initial design)

## Context

The manager control plane must drive node agents that run on each hypervisor
host: creating and lifecycle-managing VMs, reporting health and resource
usage, and executing HA actions such as fencing and failover. This traffic is
internal, latency-sensitive, and needs a well-defined, versioned contract
between components that are released and upgraded independently. Some
operations, such as health reporting and event delivery, benefit from
streaming. Node agents must also authenticate the manager and vice versa.

The alternatives considered were a REST/JSON API on each node, a message
broker, and an SSH-based command runner.

## Decision

We will use gRPC with Protocol Buffers for all communication between the
manager and node agents. Service definitions live in `proto/`, and the node
agent server is implemented in `internal/node/`. Connections are secured with
mutual TLS using certificates managed by `internal/tls/`.

The public-facing REST API (`internal/api/`) is a separate concern and is not
affected by this decision.

## Consequences

- The `.proto` files are a strongly typed, versioned contract, and Go client
  and server code is generated from them.
- HTTP/2 multiplexing and native streaming support health checks and event
  delivery without polling.
- Mutual TLS gives each side an authenticated identity.
- Contributors need protobuf tooling to regenerate code when the contract
  changes.
- gRPC endpoints are harder to debug with plain `curl` than REST endpoints.
- Generated code must be kept in sync with the `.proto` files.
