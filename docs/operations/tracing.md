# HiveStack — OpenTelemetry Tracing Setup

## Architecture

```
Browser → API Server (Manager) → Node Agent (gRPC)
   ↓              ↓                      ↓
  Span  ──→  Span Context  ──→  Span Context
 (HTTP)      (gRPC Metadata)    (gRPC Metadata)
```

## Configuration

### 1. Manager (API Server)

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint("localhost:4317"),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil { return nil, err }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("hivestack-manager"),
        )),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)),
    )
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### 2. Node Agent (gRPC Client)

Trace propagation via gRPC metadata:

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

conn, err := grpc.Dial(target,
    grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
    grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
)
```

### 3. Span Sampling

| Environment | Sampler | Rate |
|-------------|---------|------|
| Development | AlwaysOn | 100% |
| Staging | TraceIDRatioBased | 50% |
| Production | TraceIDRatioBased | 10% |

### 4. Key Spans

| Span | Description | Attributes |
|------|-------------|------------|
| `vm.create` | VM provisioning | vm_id, host_id, tenant_id |
| `vm.start` | VM power on | vm_id, node_id |
| `vm.migrate` | Live migration | vm_id, source_host, target_host |
| `ha.failover` | Host failure failover | node_id, vm_count, duration_ms |
| `compliance.check` | HANA compliance validation | vm_id, check_type, result |

## Environment Variables

```
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_SAMPLER_RATIO=0.1
OTEL_SERVICE_NAME=hivestack-manager
```
