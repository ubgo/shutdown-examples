# 07-with-prom

> Export Prometheus metrics for shutdown phases + handlers via
> [`shutdown-prom`](https://github.com/ubgo/shutdown/tree/main/contrib/shutdown-prom).

```sh
go run .
```

## What it shows

Three metrics are populated as the shutdown sequence runs:

| Metric | Type | Labels |
|--------|------|--------|
| `shutdown_phase_duration_seconds` | Histogram | `phase` |
| `shutdown_handler_duration_seconds` | Histogram | `phase`, `name`, `status` |
| `shutdown_handlers_total` | Counter | `phase`, `name`, `status` |

`status` is `ok` or `error`. After shutdown completes, the example
dumps the metric values to stdout in Prometheus text format so you can
see what would have been scraped.

## Real-world: serve the metrics on /metrics

In production you wouldn't print metrics — you'd expose them on an
HTTP endpoint that Prometheus scrapes:

```go
import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/ubgo/shutdown"
    shutdownprom "github.com/ubgo/shutdown/contrib/shutdown-prom"
)

reg := prometheus.NewRegistry()
m := shutdownprom.NewMetrics(reg)

mgr := shutdown.New(shutdown.WithBudget(30 * time.Second))
mgr.Subscribe(shutdownprom.Observer(m))

// Expose /metrics on the same server you use for app metrics.
mux := http.NewServeMux()
mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
metricsSrv := &http.Server{Addr: ":9090", Handler: mux}

// Register the metrics endpoint with the manager too — and flush it
// LAST (PhaseFlushLogs) so a final scrape can catch the shutdown
// metrics themselves.
_ = mgr.Register("metrics-server",
    func(ctx context.Context) error { return metricsSrv.Shutdown(ctx) },
    shutdown.WithPhase(shutdown.PhaseFlushLogs),
)

go func() { _ = metricsSrv.ListenAndServe() }()
```

## Sample queries

```promql
# Wall-clock time spent shutting down, by phase, last 1h
sum(rate(shutdown_phase_duration_seconds_sum[1h])) by (phase)

# Handlers that have failed at shutdown in the last hour — alert on this
increase(shutdown_handlers_total{status="error"}[1h]) > 0

# 99th-percentile handler duration by name (catch slow drains)
histogram_quantile(0.99,
  sum(rate(shutdown_handler_duration_seconds_bucket[5m])) by (name, le)
)
```

## Why expose shutdown metrics at all?

- **Capacity planning** — if your `PhaseDrainTraffic` handler routinely
  takes 25s and your `terminationGracePeriodSeconds` is 30s, you're
  one slow request away from a SIGKILL. The histogram tells you.
- **Regression detection** — a dependency that started taking longer
  to close shows up here before it shows up as failed deploys.
- **Crash autopsy** — if the watchdog hard-exits a pod, the failure
  counter (`status="error"`) tells you which handler hung.

## Composing with other observers

```go
mgr.Subscribe(shutdownprom.Observer(metrics))
mgr.Subscribe(shutdownzap.Observer(logger))
mgr.Subscribe(shutdownotel.Observer(tracer))
```

Each observer is independent — none of them knows about the others.
