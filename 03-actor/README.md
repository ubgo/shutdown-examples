# 03-actor

> `RegisterActor` for long-running goroutines where the run loop and the
> cancel mechanism live in different goroutines (oklog/run-style).

```sh
go run .
```

## What it shows

The actor pattern is for "background worker" code — anything where the
work is happening in a goroutine the manager doesn't own. The manager
holds the **interrupt** function; the caller's goroutine signals
**completion** via the returned `*ActorHandle.Done(err)`.

```
worker: started
[trigger shutdown]
worker: interrupt called by shutdown
worker: clean exit
shutdown clean
```

## When you'd use this (vs plain Register)

| Use plain `Register` when | Use `RegisterActor` when |
|---------------------------|--------------------------|
| Your shutdown is a synchronous call (`db.Close`, `srv.Shutdown(ctx)`). | The thing you're stopping is in a goroutine you spawn. |
| The cleanup completes when your handler returns. | "Stopped" is a separate event from "interrupt fired". |
| Single-step. | Two-step handshake (interrupt → wait for completion). |

`gocron` and `http.Server` are *not* actors in this sense — they have
synchronous `Shutdown()` calls that block until done. They're plain
`Register`. See `08-http-nethttp` and `11-cron-gocron` for those.

## Real-world: queue consumer

A NATS or Kafka consumer that reads from a subscription in a long-lived
goroutine fits the actor shape exactly:

```go
sub, _ := nats.Subscribe("orders", handler)
stopped := make(chan struct{})

handle, _ := mgr.RegisterActor("orders-consumer",
    func(_ error) {
        // interrupt: stop the consumer, then signal the run loop.
        _ = sub.Unsubscribe()
        close(stopped)
    },
    shutdown.WithActorPhase(shutdown.PhaseDrainTraffic),
    shutdown.WithActorTimeout(30*time.Second),
)

go func() {
    // The run loop. Could be a select on subscription channel +
    // stopped channel, a worker pool, etc.
    for {
        select {
        case <-stopped:
            // Drain any in-flight handlers, flush metrics, etc.
            inflight.Wait()
            handle.Done(nil)
            return
        case msg := <-sub.Channel():
            inflight.Add(1)
            go func(m *nats.Msg) {
                defer inflight.Done()
                handler(m)
            }(msg)
        }
    }
}()
```

The manager calls `interrupt` during `PhaseDrainTraffic`, then waits up
to `WithActorTimeout` for `handle.Done` to be called. If the actor
hasn't called `Done` by then, the actor's handler returns `ctx.Err()`
and the manager moves on (the goroutine continues to run in the
background until the watchdog hard-exits the process).

## Why not just `mgr.Register("worker", stopFunc)`?

You can — but you'd have to either:

1. Block inside `stopFunc` until the run loop has actually exited
   (manual mutex or channel orchestration), or
2. Return immediately, and accept that subsequent phases may run
   while the worker is still mid-cleanup.

`RegisterActor` makes the wait-for-completion the default, with a
typed handle that can't be misused.
