# 09-http-gin

> Graceful shutdown of a Gin engine wrapped in `*http.Server`, via the
> [`shutdown-gin`](https://github.com/ubgo/shutdown/tree/main/contrib/shutdown-gin) contrib.

```sh
go run .
curl localhost:8080/         # in another terminal
# Ctrl-C in the example terminal while the request is in flight
```

## What it shows

Gin engines don't own a server — you wrap `gin.Engine` in a stdlib
`*http.Server` you control. `shutdown-gin.Register` takes that same
`*http.Server` and registers `srv.Shutdown(ctx)` in
`PhaseStopAccepting`, exactly like the nethttp adapter.

## Why two contribs (`shutdown-gin` and `shutdown-nethttp`) when they're "the same"?

They are nearly identical — both end up calling `srv.Shutdown(ctx)` on
the same `*http.Server`. The reason for separate packages:

- **Discoverability** — users browsing for "Gin shutdown adapter" will
  find `shutdown-gin` by name and import path.
- **Documentation hooks** — `shutdown-gin/README.md` can mention Gin
  specifics (route groups, middleware), while `shutdown-nethttp` stays
  pure stdlib.
- **Future divergence** — if Gin ever adds an engine-level Shutdown
  method (or its own drain hooks), `shutdown-gin` can take advantage
  without affecting nethttp users.

If you don't care about the naming, importing `shutdown-nethttp`
directly works for any framework that hands you a `*http.Server`.

## Production composition

The same composition pattern from `08-http-nethttp` applies — see
[that README's "Real-world: full production composition" section](../08-http-nethttp/README.md#real-world-full-production-composition)
for the full HTTP + DB + Redis + OTEL example. Just swap the
`shutdownnethttp.Register` line for:

```go
import shutdowngin "github.com/ubgo/shutdown/contrib/shutdown-gin"

r := gin.Default()
srv := &http.Server{Addr: ":8080", Handler: r}
_ = shutdowngin.Register(mgr, srv, shutdowngin.WithTimeout(15*time.Second))
```

Everything else is identical.

## See also

- `08-http-nethttp` — the underlying pattern, no framework
- `10-k8s-prestop` — full k8s lifecycle composition
