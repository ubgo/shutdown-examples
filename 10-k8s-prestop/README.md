# 10-k8s-prestop

> A complete Kubernetes graceful-shutdown demo using `ubgo/shutdown`.
>
> **What you'll see:** a load-balancer-friendly drain (readiness flip
> first, then in-flight finish, then connection close, then process
> exit) — bounded by `terminationGracePeriodSeconds` so SIGKILL never
> fires.

## Prereqs

- Docker (running)
- `kubectl`
- `kind` — `brew install kind`
- `task` — `brew install go-task/tap/go-task`

## One-shot demo

```sh
task demo
```

This will:

1. Create a `kind` cluster (skipped if one already exists).
2. Build the Docker image and load it into the cluster.
3. Apply `deployment.yaml`.
4. Smoke-test `/readyz` from inside the cluster.
5. Delete the pod (k8s sends SIGTERM) and tail the logs so you can watch
   the phases run in order.

Tear down everything with `task clean`.

## What the phases do (mapped to k8s lifecycle)

| Phase (`shutdown`) | What it does | Why it matters in k8s |
|--------------------|--------------|------------------------|
| `PhasePreShutdown` | Flip `/readyz` to 503 + sleep 3s | Kubelet sees readiness drop; the Service stops sending new connections. The 3s sleep gives at least one probe interval to propagate. |
| `PhaseStopAccepting` | `http.Server.Shutdown(ctx)` | Listener closes — no new connections. In-flight requests keep their handler context. |
| `PhaseDrainTraffic` | Wait for in-flight `WaitGroup` | The 2-second simulated job in `/` finishes cleanly. |
| `PhaseFlushLogs` | Final `os.Stdout.Sync()` | Emits any buffered log lines before exit. |

## Lifecycle timeline

```
T+0    user runs `kubectl delete pod`
T+0    kubelet starts terminating the pod
T+0    kubelet sends SIGTERM to PID 1
       └─ shutdown.Manager.Listen receives SIGTERM
T+0    PhasePreShutdown — /readyz starts returning 503
T+2    next readinessProbe fires; pod marked NotReady; Service stops routing
T+3    PhasePreShutdown returns (slept 3s)
T+3    PhaseStopAccepting — http.Server.Shutdown begins
T+5    PhaseDrainTraffic — WaitGroup waits for the last 2s request
T+5    PhaseFlushLogs — final flush
T+5    process exits cleanly with status 0
   ───── well within terminationGracePeriodSeconds: 30 ─────
```

## Files

| File | Purpose |
|------|---------|
| `main.go` | The Go service. Registers handlers in 4 phases. |
| `Dockerfile` | Multi-stage build → distroless `static-debian12:nonroot`. |
| `deployment.yaml` | k8s Deployment + Service. `terminationGracePeriodSeconds: 30`. |
| `Taskfile.yml` | All commands: build, deploy, demo, clean. |

## Without preStop?

You'll notice `deployment.yaml` has no `lifecycle.preStop` block. With
`ubgo/shutdown`, you don't need one — `PhasePreShutdown` already flips
readiness on SIGTERM. preStop only adds value if you want to flip
*before* SIGTERM (a "head start" for slow load balancers); see the
comment in `deployment.yaml` if you want to add it.

## Without kind?

The Taskfile is just shell — you can swap `kind` for any local k8s tool.
Replace `kind create cluster` with `minikube start` / `k3d cluster create`
and `kind load docker-image` with the equivalent (`minikube image load`,
`k3d image import`).
