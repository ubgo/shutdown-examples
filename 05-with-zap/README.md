# 05-with-zap

> Wire the [`shutdown-zap`](https://github.com/ubgo/shutdown/tree/main/contrib/shutdown-zap) observer into a zap logging pipeline.

```sh
go run .
```

## What it shows

The manager's *internal* logger uses `log/slog` by default. The
`shutdown-zap` contrib is a separate **observer** — every shutdown event
(signal, phase start/end, handler start/end, complete) flows through
your zap config in addition to whatever slog is doing.

In this example we mute the internal logger via `WithLogger(NoopLogger())`
so the zap-formatted output is the only thing you see.

## Real-world: production zap config

```go
import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

// Production-grade JSON encoder with ISO-8601 timestamps + caller info.
prodCfg := zap.NewProductionConfig()
prodCfg.EncoderConfig.TimeKey = "ts"
prodCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
prodCfg.EncoderConfig.MessageKey = "msg"
prodCfg.OutputPaths = []string{"stdout"}
prodCfg.Sampling = nil // shutdown events are rare; don't sample them away

logger, err := prodCfg.Build()
if err != nil { panic(err) }
defer logger.Sync()

mgr := shutdown.New(
    shutdown.WithBudget(30 * time.Second),
    // Decide whether you want a single source of truth or two:
    //
    //  Option A: one source — observer-only.
    //    shutdown.WithLogger(shutdown.NoopLogger()),  // mute internal
    //    mgr.Subscribe(shutdownzap.Observer(logger))  // zap is the channel
    //
    //  Option B: two sources — internal slog (terse) + zap observer (rich).
    //    Default WithLogger (slog) emits 4–5 lines; zap observer emits
    //    one structured event per phase/handler boundary. Useful when
    //    you want the slog summary in the log stream and the rich
    //    structured events for indexing / querying.
)
mgr.Subscribe(shutdownzap.Observer(logger))
```

## Events the observer logs

| Event | Log line | Level |
|-------|----------|-------|
| Signal arrived | `shutdown: signal received` | INFO |
| Phase started | `shutdown: phase start` | INFO |
| Phase ended | `shutdown: phase end` | INFO |
| Handler started | `shutdown: handler start` | INFO |
| Handler succeeded | `shutdown: handler end` | INFO |
| Handler failed | `shutdown: handler failed` | ERROR |
| Shutdown done (clean) | `shutdown: completed` | INFO |
| Shutdown done (with errors) | `shutdown: completed with errors` | ERROR |

Each line carries `phase`, `name`, `duration`, and `error` fields where
applicable.

## Composing with other observers

Observers are additive — subscribe as many as you want:

```go
mgr.Subscribe(shutdownzap.Observer(logger))
mgr.Subscribe(shutdownotel.Observer(tracer))
mgr.Subscribe(shutdownprom.Observer(promMetrics))
```

You get logs, traces, and metrics from a single shutdown sequence.
None of the contribs know about each other.
