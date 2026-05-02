# 08-http-nethttp

> Graceful shutdown of a stdlib `*http.Server` via the
> [`shutdown-nethttp`](https://github.com/ubgo/shutdown/tree/main/contrib/shutdown-nethttp) contrib.

```sh
go run .
# in another terminal:
curl localhost:8080/
# while the request is mid-flight, send SIGINT (Ctrl-C) to the example.
```

## What it shows

- The server's `/` handler sleeps 2 seconds before responding (simulates
  slow downstream work).
- `shutdownnethttp.Register` places `srv.Shutdown(ctx)` in
  `PhaseStopAccepting`.
- When you Ctrl-C: the listener stops accepting new connections, but
  the in-flight `curl` request continues until the handler returns.
- The 2-second response is delivered cleanly; only THEN does the
  process exit.

## Real-world: full production composition

A typical HTTP service has more than just the server to shut down — DB,
cache, message bus, telemetry. Compose them all under one manager:

```go
import (
    "github.com/ubgo/shutdown"
    shutdownnethttp "github.com/ubgo/shutdown/contrib/shutdown-nethttp"
    shutdownotel "github.com/ubgo/shutdown/contrib/shutdown-otel"
    shutdownzap "github.com/ubgo/shutdown/contrib/shutdown-zap"
    "go.opentelemetry.io/otel"
)

mgr := shutdown.New(
    shutdown.WithBudget(30 * time.Second),
    shutdown.WithExitOnComplete(0, 1),
)

// Optional: wire telemetry observers. They're fully composable.
mgr.Subscribe(shutdownzap.Observer(zapLogger))
mgr.Subscribe(shutdownotel.Observer(otel.Tracer("shutdown")))

// 1. Readiness flip — load balancer drains us before the listener closes.
var ready atomic.Bool
ready.Store(true)
_ = mgr.Register("readiness-flip", func(_ context.Context) error {
    ready.Store(false)
    time.Sleep(3 * time.Second) // give kubelet a probe interval to notice
    return nil
}, shutdown.WithPhase(shutdown.PhasePreShutdown))

// 2. HTTP server — listener stops accepting; handler ctx gets a deadline.
srv := &http.Server{Addr: ":8080", Handler: mux}
_ = shutdownnethttp.Register(mgr, srv, shutdownnethttp.WithTimeout(15*time.Second))

// 3. Wait for any worker goroutines / background jobs.
_ = mgr.Register("workers-drain", func(ctx context.Context) error {
    return workers.Drain(ctx)
}, shutdown.WithPhase(shutdown.PhaseDrainTraffic))

// 4. Close DB / cache / messaging clients in parallel.
_ = mgr.Register("db",    closeFn(db.Close),  shutdown.WithPhase(shutdown.PhaseCloseClients))
_ = mgr.Register("redis", closeFn(rdb.Close), shutdown.WithPhase(shutdown.PhaseCloseClients))
_ = mgr.Register("nats",  closeFn(nc.Drain),  shutdown.WithPhase(shutdown.PhaseCloseClients))

// 5. Flush traces LAST so prior phase errors actually leave the process.
_ = mgr.Register("otel-flush", tp.Shutdown, shutdown.WithPhase(shutdown.PhaseFlushLogs))

go func() { _ = srv.ListenAndServe() }()

if err := mgr.Listen(context.Background()); err != nil {
    log.Println("shutdown errors:", err)
}

func closeFn(fn func() error) shutdown.HandlerFunc {
    return func(_ context.Context) error { return fn() }
}
```

## Phase ordering matters

| If you... | Then... |
|-----------|---------|
| Close the DB before the listener stops | An in-flight request hits a closed connection → 500. |
| Flush OTEL before all phases run | You lose the trace span for everything after. |
| Skip the readiness flip | The load balancer keeps sending requests to a pod that's about to close its listener — guaranteed dropped requests during the SIGTERM-to-listener-close window. |
| Run all closes in `PhaseStopAccepting` | The listener-close blocks on `srv.Shutdown` waiting for in-flight handlers, who can't complete because their DB is already closed. Deadlock until WithBudget kills it. |

Phase ordering encodes these dependencies once, statically — the
runner enforces them.

## Server timeouts

`shutdown-nethttp.WithTimeout` is the time budget for `srv.Shutdown(ctx)`.
That's how long the server gets to drain in-flight handlers before its
context cancels. Pick a value that's:

- ≥ your slowest expected handler's `time.Sleep` / downstream call timeout
- ≤ your manager's `WithBudget` minus headroom for later phases

## See also

- `09-http-gin` — same pattern with Gin
- `10-k8s-prestop` — the full k8s lifecycle including `terminationGracePeriodSeconds`
- `11-cron-gocron` — gocron drain, same architectural shape
