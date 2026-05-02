# 04-watchdog

> A deliberately broken handler that ignores `ctx`. Watch the watchdog
> hard-exit the process after `budget + grace`.

```sh
go run .
# exit code 99
```

## What it shows

The handler `hangs` calls `time.Sleep(5 * time.Second)` and never checks
its context. With `WithBudget(200ms)` + `WithWatchdogGrace(50ms)`, the
manager hard-exits the process at T+250ms via `os.Exit(99)`.

```
hangs: started, ignoring ctx for 5 seconds
[manager logs] shutdown: watchdog deadline exceeded — forcing exit
exit status 99
```

The "good" exit code we set via `WithExitOnComplete(0, 99)` for the
*failure* case is what fires here — not because the handler returned an
error (it never returned at all), but because the watchdog used the
same failure code.

## When you'd want this

The watchdog is a **last-resort safety net** for handlers that might
hang due to external dependency failures:

- A DB driver waiting forever for a connection that will never come.
- A NATS Drain that's blocked on a peer that's already gone.
- A third-party SDK with no timeout you can pass.

Without the watchdog, your process sits forever in a "Terminating"
state — orchestrators (k8s, systemd) eventually `SIGKILL` it but only
after their own grace period.

## Tuning the budget

The right budget depends on the expected longest-handler time + a
margin:

| Service shape | Suggested budget |
|---------------|------------------|
| HTTP API with DB pool | 30s — 1m (drain in-flight requests + close pool) |
| HTTP API with slow downstream calls | 60s — 5m (depends on call timeout) |
| Worker / cron drain (long jobs) | hours, paired with `gocron.WithStopTimeout` of similar size — see `11-cron-gocron` |
| Batch job that must not be interrupted | `WithBudget(0)` — disables the watchdog and relies on the orchestrator |

The cascade rule for any drain:

```
your blocking call's own deadline   <   per-handler WithTimeout   <   WithBudget
```

If you reverse the order, the watchdog will kill in-flight work that
the underlying library would have completed cleanly given another second.

## Disabling the watchdog

```go
shutdown.WithBudget(0)              // no manager-level deadline
shutdown.WithWatchdogGrace(0)       // no watchdog at all
```

Now your only safety net is the orchestrator's `terminationGracePeriodSeconds`
(k8s) or `TimeoutStopSec` (systemd). Use this when you'd rather have
the orchestrator decide than guess a budget that's hard to predict.
