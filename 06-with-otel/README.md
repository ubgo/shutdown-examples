# 06-with-otel

> Emit OpenTelemetry spans for shutdown phases + handlers via
> [`shutdown-otel`](https://github.com/ubgo/shutdown/tree/main/contrib/shutdown-otel).

```sh
go run .
```

## What it shows

A single `shutdown` root span; one child span per phase
(`shutdown.phase.<name>`); one leaf span per handler
(`shutdown.handler.<name>`). Errors are recorded on the leaf span via
`span.RecordError` + `span.SetStatus(codes.Error, …)`.

The example uses the **stdout exporter** so you can see the spans
directly in your terminal.

## Span hierarchy

```
shutdown                                (root)
└─ shutdown.phase.PreShutdown
└─ shutdown.phase.StopAccepting
   └─ shutdown.handler.http.Server
└─ shutdown.phase.CloseClients
   └─ shutdown.handler.db
   └─ shutdown.handler.redis            ← RecordError attached here
└─ shutdown.phase.FlushLogs
   └─ shutdown.handler.otel-flush
```

Each span carries `name`, `phase`, and `duration_ms` attributes.

## Real-world: production OTLP exporter

The stdout exporter is a debugging tool. In production you'd ship
spans to a real backend (Jaeger, Tempo, Honeycomb, Datadog, etc.) via
OTLP:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/resource"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

exp, err := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("otel-collector.observability:4317"),
    otlptracegrpc.WithInsecure(),
)
if err != nil { panic(err) }

res, _ := resource.Merge(resource.Default(), resource.NewWithAttributes(
    semconv.SchemaURL,
    semconv.ServiceName("my-service"),
    semconv.ServiceVersion("1.4.2"),
))

tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exp),
    sdktrace.WithResource(res),
)
otel.SetTracerProvider(tp)

mgr.Subscribe(shutdownotel.Observer(tp.Tracer("shutdown")))
```

## Important: flush spans LAST

OTEL's `TracerProvider.Shutdown(ctx)` flushes the batch processor's
queue. If you call it before the rest of shutdown is done, you'll lose
the spans for whatever happens after. Always register it in
`PhaseFlushLogs` (the highest predefined phase):

```go
_ = mgr.Register("otel", tp.Shutdown,
    shutdown.WithPhase(shutdown.PhaseFlushLogs),
    shutdown.WithTimeout(10*time.Second),
)
```

The `shutdown-otel` observer's own span lifecycle is independent — it
auto-closes the root span on `OnComplete`, so you're not racing the
TracerProvider.Shutdown call against the observer.

## Composing with other observers

```go
mgr.Subscribe(shutdownotel.Observer(tracer))
mgr.Subscribe(shutdownzap.Observer(logger))
mgr.Subscribe(shutdownprom.Observer(metrics))
```

Traces, structured logs, and Prometheus metrics from the same shutdown
sequence — observers are independent.
