// Example 10-k8s-prestop — a tiny HTTP service designed for Kubernetes.
//
// What it demonstrates
//
//   - The full k8s shutdown handshake: preStop (readiness flip) → SIGTERM
//     (drain + close) → terminationGracePeriodSeconds → SIGKILL.
//   - Phase ordering that maps 1:1 onto the k8s lifecycle:
//       PhasePreShutdown    — flip /readyz to Down
//       PhaseStopAccepting  — http.Server.Shutdown
//       PhaseDrainTraffic   — wait for in-flight jobs
//       PhaseFlushLogs      — final log flush
//   - WithBudget set a few seconds shorter than k8s'
//     terminationGracePeriodSeconds so we exit clean before SIGKILL.
//
// The /readyz endpoint reads the same atomic flag the PreShutdown handler
// flips, so kubelet sees readiness drop before the listener closes.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ubgo/shutdown"
	shutdownnethttp "github.com/ubgo/shutdown/contrib/shutdown-nethttp"
)

// ready is flipped to false by the PreShutdown phase so /readyz returns
// 503 even while the listener is still accepting connections. The load
// balancer in front of the pod stops sending new traffic; in-flight
// requests finish.
var ready atomic.Bool

func main() {
	ready.Store(true)

	mgr := shutdown.New(
		// Soft budget: must be < terminationGracePeriodSeconds in deployment.yaml.
		shutdown.WithBudget(25*time.Second),
		shutdown.WithExitOnComplete(0, 1),
	)

	// Track in-flight requests with a WaitGroup so PhaseDrainTraffic can wait.
	var inflight sync.WaitGroup

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		inflight.Add(1)
		defer inflight.Done()
		// Simulate a slow job so the drain matters.
		time.Sleep(2 * time.Second)
		_, _ = fmt.Fprintln(w, "hello from", os.Getenv("HOSTNAME"))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ready.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	// Phase 1: PreShutdown — flip readiness so the load balancer drains us.
	_ = mgr.Register("readiness-flip", func(_ context.Context) error {
		log.Println("[PreShutdown] flipping /readyz to 503")
		ready.Store(false)
		// Give kubelet at least one probe interval to notice. Production
		// services tune this from observed probe cadence; 3s is a safe
		// default for the Kubernetes default of `periodSeconds: 10`.
		time.Sleep(3 * time.Second)
		return nil
	}, shutdown.WithPhase(shutdown.PhasePreShutdown))

	// Phase 2: StopAccepting — http.Server.Shutdown.
	_ = shutdownnethttp.Register(mgr, srv)

	// Phase 3: DrainTraffic — wait for in-flight handlers to finish.
	_ = mgr.Register("inflight-drain", func(ctx context.Context) error {
		log.Println("[DrainTraffic] waiting for in-flight jobs")
		done := make(chan struct{})
		go func() {
			inflight.Wait()
			close(done)
		}()
		select {
		case <-done:
			log.Println("[DrainTraffic] all in-flight done")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	},
		shutdown.WithPhase(shutdown.PhaseDrainTraffic),
		shutdown.WithTimeout(15*time.Second),
	)

	// Phase 4: FlushLogs — final flush.
	_ = mgr.Register("flush-logs", func(_ context.Context) error {
		log.Println("[FlushLogs] flushing")
		_ = os.Stdout.Sync()
		return nil
	}, shutdown.WithPhase(shutdown.PhaseFlushLogs))

	go func() {
		log.Println("listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("listen:", err)
		}
	}()

	if err := mgr.Listen(context.Background()); err != nil {
		log.Println("shutdown error:", err)
	}
}
