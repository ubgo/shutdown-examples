# 11-cron-gocron

> Graceful shutdown of a [gocron](https://github.com/go-co-op/gocron) scheduler with `ubgo/shutdown`.
>
> **What you'll see:** a periodic job firing every second; mid-run, the
> example sends itself SIGTERM; gocron blocks the in-flight job to
> completion before the scheduler closes; the process exits cleanly.

## Run

```sh
cd 11-cron-gocron
go run .
```

Expected output:

```
INFO scheduler started; will trigger shutdown in 3.5s
INFO job started run=1
INFO job finished run=1
INFO job started run=2
INFO job finished run=2
INFO job started run=3
--- triggering shutdown via SIGTERM ---
INFO [DrainTraffic] StopJobs — waiting for in-flight to finish
INFO job finished run=3                 ← waited for the in-flight job!
INFO [CloseClients] scheduler.Shutdown
INFO shutdown: completed cleanly duration=...
```

## How the timeouts compose

```
gocron.WithStopTimeout (10s)   — gocron waits this long for in-flight jobs
WithTimeout per handler (11s)  — manager gives the StopJobs handler 11s
WithBudget (15s)               — manager hard-exits after 15s if anything hangs
```

Why each value is bigger than the next: the per-handler timeout has to
exceed gocron's StopTimeout so gocron gets to honour its own deadline
before the manager's context cancels mid-call. The budget exceeds both
so the watchdog only fires if gocron itself hangs past its deadline.

## Mapping to phases

| Phase | What runs | Why this phase |
|-------|-----------|-----------------|
| `PhaseDrainTraffic` | `scheduler.StopJobs()` | "Stop accepting new ticks, wait for in-flight" — exactly the drain phase. |
| `PhaseCloseClients` | `scheduler.Shutdown()` | Final resource release, after the drain. |

If you also serve a gocron-ui HTTP dashboard, register it in
`PhaseStopAccepting` (use the `shutdown-nethttp` contrib) so the listener
closes *before* the scheduler is asked to drain — this keeps the UI from
showing stale state during the drain window.

## What about the "wait for current jobs" guarantee?

That comes from gocron, not from `ubgo/shutdown`. `scheduler.StopJobs()`
blocks until in-flight jobs return (bounded by gocron's `WithStopTimeout`).
The shutdown library's job is to:

1. Call `StopJobs` at the right moment in the phase order.
2. Aggregate its error with anything else shutting down at the same time.
3. Hard-exit via the watchdog if gocron itself stalls past `WithBudget`.

## Migration from `lace/shutdown`

If you have an existing service using `lace/shutdown` with a single
`WithShutdownHandler` doing both the HTTP UI shutdown and the scheduler
teardown, the move is mechanical:

```go
// before
shutdown.New(
    shutdown.WithShutdownHandler(func(ctx context.Context) error {
        httpSrv.Shutdown(ctx)
        scheduler.StopJobs()
        return scheduler.Shutdown()
    }),
).Listen()

// after
mgr := shutdown.New(shutdown.WithBudget(...))
shutdownnethttp.Register(mgr, httpSrv)                                              // PhaseStopAccepting
mgr.Register("stop-jobs", func(_ context.Context) error { return scheduler.StopJobs() },
    shutdown.WithPhase(shutdown.PhaseDrainTraffic))
mgr.Register("shutdown", func(_ context.Context) error { return scheduler.Shutdown() },
    shutdown.WithPhase(shutdown.PhaseCloseClients))
mgr.Listen(ctx)
```

The behaviour you gain: ordering, error aggregation, watchdog, observer
hooks for telemetry.
