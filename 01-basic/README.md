# 01-basic

> The smallest possible use of `ubgo/shutdown` — programmatic
> `Shutdown(ctx)` without OS signals.

```sh
go run .
```

## What it shows

- Three handlers registered across two phases.
- `db` and `redis` run in parallel inside `PhaseCloseClients` (same phase).
- `logs` runs strictly after them in `PhaseFlushLogs`.
- The `redis` handler returns an error; it's joined into the aggregate
  return value via `errors.Join` rather than aborting the rest.

## Expected output

```
db: closing
redis: closing
logs: flushing
shutdown returned aggregate error: redis: connection already lost
```

`db` and `redis` lines may swap (they run concurrently). `logs` is
always last because it's in a higher-numbered phase.

## When you'd use this pattern

- **Tests** — drive the manager directly without spinning up a process
  + sending it a real signal.
- **HTTP `/admin/shutdown`** — see [Recipes in the core
  README](https://github.com/ubgo/shutdown#recipes).
- **Panic recovery middleware** — call `mgr.Shutdown(ctx)` after
  recovering an unrecoverable panic.
