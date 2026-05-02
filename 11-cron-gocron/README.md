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

## Real-world: long-running jobs (24h drain budget)

The example above uses tiny timeouts so it's quick to demo. A production
cron service usually wants the opposite: **finish whatever job is running,
no matter how long it takes** — bounded only by a wall-clock ceiling that
prevents zombie processes.

A common shape (matching the original `lace/shutdown` setup people are
migrating from):

```go
scheduler, err := gocron.NewScheduler(
    gocron.WithLogger(gocronlogger.New(gozap.GetLogger())),
    gocron.WithMonitor(gocronmonitor.New(gozap.GetLogger())),
    gocron.WithStopTimeout(24 * time.Hour), // wait up to 24h for in-flight jobs
)

mgr := shutdown.New(
    // 25h budget = StopTimeout + 1h headroom. The watchdog only fires
    // if gocron itself hangs past its own 24h declared deadline.
    shutdown.WithBudget(25 * time.Hour),
    shutdown.WithLogger(shutdown.SlogLogger(nil)),
    shutdown.WithExitOnComplete(0, 1),
)

// Optional: emit a structured log line on every phase boundary via zap.
mgr.Subscribe(shutdownzap.Observer(gozap.GetLogger()))

// Phase 1: HTTP UI stops accepting (if you serve gocron-ui dashboard).
if httpSrv != nil {
    _ = shutdownnethttp.Register(mgr, httpSrv)
}

// Phase 2: drain — wait up to 24h for the current job to finish.
_ = mgr.Register("scheduler-stop-jobs", func(_ context.Context) error {
    return scheduler.StopJobs()
},
    shutdown.WithPhase(shutdown.PhaseDrainTraffic),
    shutdown.WithTimeout(24*time.Hour + 30*time.Minute), // > gocron's StopTimeout
)

// Phase 3: scheduler resource teardown.
_ = mgr.Register("scheduler-shutdown", func(_ context.Context) error {
    return scheduler.Shutdown()
},
    shutdown.WithPhase(shutdown.PhaseCloseClients),
    shutdown.WithTimeout(30*time.Second),
)

if err := mgr.Listen(context.Background()); err != nil {
    gozap.GetLogger().Error("shutdown errors", zap.Error(err))
}
```

### Why those exact numbers?

The three timeouts form a cascade — each layer must outlive the one below
it so a clean drain isn't killed prematurely:

```
gocron.WithStopTimeout              24h        (gocron's own deadline)
  < per-handler WithTimeout         24h 30m    (manager gives gocron headroom)
    < WithBudget                    25h        (watchdog last-resort)
```

If you reverse the order you get bugs:

| Mistake | What happens |
|---------|--------------|
| `WithBudget(20h)` (less than StopTimeout) | Watchdog hard-exits at T+20h, killing an in-flight job that gocron was still patiently waiting on. Process exits with the watchdog's failure code; orchestrator treats it as a crash. |
| `WithTimeout(20h)` (less than StopTimeout) | Manager cancels the per-handler context at T+20h. `StopJobs` returns ctx.Err. The job goroutine keeps running but no one is waiting; result depends on whether the job respects ctx. |
| `WithBudget(0)` (no budget) | Watchdog disabled. If gocron hangs (driver bug, deadlocked job), only `kill -9` will stop the process. |

### "Forever" budget

If you really want the process to wait as long as it takes — no
watchdog, no upper bound — set:

```go
shutdown.WithBudget(0)              // disables the manager-level deadline
shutdown.WithWatchdogGrace(0)       // disables the watchdog (also)
```

You then rely entirely on the orchestrator (k8s `terminationGracePeriodSeconds`,
systemd `TimeoutStopSec`) to be the safety net. That's a reasonable
default for batch jobs that *must not* be interrupted, but means your
ops tooling has to know the right limit instead of the binary itself.

### Without an HTTP UI

If your cron service has no `gocron-ui` dashboard, drop the `shutdownnethttp.Register`
line — there's no listener to close. Phases 2 and 3 are all you need.

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
