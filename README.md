# shutdown-examples

> Runnable examples for [`github.com/ubgo/shutdown`](https://github.com/ubgo/shutdown).
>
> Each subdirectory is its own Go module so you can run any example
> standalone: `cd 01-basic && go run .`

## Examples

| # | Directory | What it shows |
|---|-----------|----------------|
| 01 | [`01-basic`](./01-basic) | Programmatic `Shutdown(ctx)` without OS signals — phase ordering + error aggregation. |
| 02 | [`02-phases`](./02-phases) | Every predefined phase populated; handlers fire in phase order. |
| 03 | [`03-actor`](./03-actor) | `RegisterActor` for long-running goroutines (run + interrupt pair). |
| 04 | [`04-watchdog`](./04-watchdog) | Deliberately broken handler ignores ctx; watchdog hard-exits the process. |
| 05 | [`05-with-zap`](./05-with-zap) | Wire the `shutdown-zap` observer into a zap pipeline. |
| 06 | [`06-with-otel`](./06-with-otel) | Emit OTEL spans (root + phase + handler) via `shutdown-otel`. |
| 07 | [`07-with-prom`](./07-with-prom) | Export Prometheus metrics via `shutdown-prom`. |
| 08 | [`08-http-nethttp`](./08-http-nethttp) | Graceful shutdown of a stdlib `*http.Server`. |
| 09 | [`09-http-gin`](./09-http-gin) | Graceful shutdown of a Gin engine wrapped in `*http.Server`. |
| 10 | [`10-k8s-prestop`](./10-k8s-prestop) | Full Kubernetes drain demo. Spins up `kind`, deploys, kills the pod, and shows the four phases firing in order — bounded by `terminationGracePeriodSeconds`. |

## Running

```sh
cd 01-basic
go run .
```

Each example logs to stdout. The HTTP examples bind to `:8080` — `Ctrl-C` to trigger shutdown.

## License

Apache-2.0 — see [`LICENSE`](./LICENSE).
