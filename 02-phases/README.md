# 02-phases

> Every predefined phase populated. Watch the output: handlers fire in
> phase order even though they were registered in random order.

```sh
go run .
```

## What it shows

- Seven handlers registered, one per predefined phase
  (`PhasePreShutdown` … `PhasePostShutdown`).
- Registration order is intentionally scrambled.
- The runner sorts by phase value, lower-first.

## Expected output

```
[readiness-flip] done    (PhasePreShutdown,    -100)
[http]           done    (PhaseStopAccepting,    0)
[nats]           done    (PhaseDrainTraffic,   100)
[queues]         done    (PhaseFlushQueues,    200)
[db]             done    (PhaseCloseClients,   300)  ← can run in parallel with...
[redis]          done    (PhaseCloseClients,   300)  ← ...this one
[logs]           done    (PhaseFlushLogs,      400)
```

(In practice the example registers each phase only once, so there's no
parallelism to observe — see `01-basic` for that.)

## Real-world phase mapping

| Phase | Typical handlers |
|-------|------------------|
| `PhasePreShutdown` | Flip readiness probe to "Down". Notify load balancer. |
| `PhaseStopAccepting` | `http.Server.Shutdown`. Close gRPC listener. Stop accepting NATS subscriptions. |
| `PhaseDrainTraffic` | NATS `Drain()`, wait for in-flight HTTP, await background-job WaitGroup. |
| `PhaseFlushQueues` | Flush Kafka producers, batched log shippers, async metric exporters. |
| `PhaseCloseClients` | DB connection pool close. Redis client close. S3 client close. |
| `PhaseFlushLogs` | OTEL TracerProvider.Shutdown. Final stdout sync. |
| `PhasePostShutdown` | Custom audit log. Exit-code reporting. Most apps don't need this. |

The full strategy table with common-mistake column is in the [core
README](https://github.com/ubgo/shutdown#strategy-which-phase-does-each-thing-go-in).

## Custom phase values

Phases are plain `int` — pass any value if you need to insert a step
between two predefined ones:

```go
const PhaseAfterDrain = 150 // between DrainTraffic (100) and FlushQueues (200)

mgr.Register("post-drain", fn, shutdown.WithPhase(PhaseAfterDrain))
```
