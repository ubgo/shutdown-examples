// Example 11-cron-gocron — a gocron scheduler that drains in-flight jobs.
//
// Demonstrates how ubgo/shutdown coordinates the shutdown of a
// long-running cron service:
//
//   PhaseDrainTraffic  — scheduler.StopJobs() blocks until in-flight
//                        gocron jobs finish (gocron's StopTimeout is the
//                        ceiling)
//   PhaseCloseClients  — scheduler.Shutdown() releases scheduler resources
//
// The actual "wait for in-flight jobs" behaviour is implemented by gocron
// (via WithStopTimeout). ubgo/shutdown's role is to call StopJobs at the
// right moment in the phase order, aggregate errors with anything else
// shutting down at the same time, and hard-exit via the watchdog if
// gocron itself hangs.
//
// Running this example
//
//   go run .
//
// You'll see a job fire every 1s. After ~3.5s the example sends itself a
// SIGTERM. The shutdown sequence runs: gocron waits for the in-flight job
// to finish (up to 1s, the simulated job duration), then the scheduler
// closes, then the process exits cleanly. The whole drain stays well
// under WithBudget.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/ubgo/shutdown"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	scheduler, err := gocron.NewScheduler(
		// gocron will wait up to this long for in-flight jobs on Shutdown
		// before forcibly cancelling them.
		//
		// Pair this with WithBudget on the manager: the manager budget
		// should be slightly larger than this value so the watchdog doesn't
		// hard-exit the process before gocron has had a chance to drain
		// gracefully.
		//
		//   gocron.StopTimeout (10s)  <  per-handler timeout (11s)  <  WithBudget (15s)
		gocron.WithStopTimeout(10 * time.Second),
	)
	if err != nil {
		panic(err)
	}

	var jobsRun atomic.Int64
	_, err = scheduler.NewJob(
		gocron.DurationJob(1*time.Second),
		gocron.NewTask(func() {
			n := jobsRun.Add(1)
			slog.Info("job started", "run", n)
			// Simulate a 1-second unit of work. If shutdown fires while
			// this is in-flight, gocron's StopJobs blocks until it
			// returns.
			time.Sleep(1 * time.Second)
			slog.Info("job finished", "run", n)
		}),
		gocron.WithName("example-job"),
		// LimitModeReschedule = if a previous run is still in flight when
		// the next tick fires, skip this tick and reschedule. The
		// alternative (LimitModeWait) would queue, which can pile up
		// during shutdown — Reschedule is almost always the right default
		// for periodic work.
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		panic(err)
	}
	scheduler.Start()
	slog.Info("scheduler started; will trigger shutdown in 3.5s")

	// Build the manager.
	//
	// WithBudget (15s) > gocron.WithStopTimeout (10s) by design: the
	// watchdog should only fire if gocron itself hangs past its own
	// declared deadline. Keeping a few seconds of headroom prevents
	// a clean drain from being killed prematurely by the watchdog.
	//
	// WithExitOnComplete makes Listen call os.Exit at the end so the
	// orchestrator (k8s/systemd) sees a clean exit code matching the
	// shutdown outcome.
	mgr := shutdown.New(
		shutdown.WithBudget(15*time.Second),
		shutdown.WithExitOnComplete(0, 1),
	)

	// Phase 1 (PhaseDrainTraffic): tell the scheduler to stop accepting
	// new tick fires, then block until any in-flight job completes.
	// This is the part that "waits for current cron jobs to finish".
	//
	// The waiting itself is gocron's behaviour, not ours — we just call
	// StopJobs at the right moment in the phase order and aggregate any
	// error it returns into the manager's error report.
	//
	// Per-handler timeout 11s > gocron's StopTimeout 10s by 1s so gocron
	// gets to honour its own declared deadline before the manager
	// short-circuits the handler context.
	_ = mgr.Register("scheduler-stop-jobs", func(_ context.Context) error {
		slog.Info("[DrainTraffic] StopJobs — waiting for in-flight to finish")
		return scheduler.StopJobs()
	},
		shutdown.WithPhase(shutdown.PhaseDrainTraffic),
		shutdown.WithTimeout(11*time.Second),
	)

	// Phase 2 (PhaseCloseClients): final teardown — release scheduler
	// resources (timer goroutines, internal channels). At this point no
	// jobs are running, so 2s is plenty.
	_ = mgr.Register("scheduler-shutdown", func(_ context.Context) error {
		slog.Info("[CloseClients] scheduler.Shutdown")
		return scheduler.Shutdown()
	},
		shutdown.WithPhase(shutdown.PhaseCloseClients),
		shutdown.WithTimeout(2*time.Second),
	)

	// Self-trigger shutdown after 3.5s so the example is observable
	// without manual Ctrl-C. In a real service this is just SIGTERM
	// from k8s / systemd.
	//
	// 3.5s is chosen deliberately: jobs fire every 1s and each takes
	// 1s, so a SIGTERM at T+3.5s is highly likely to land while a job
	// is in flight — letting you watch StopJobs actually wait for it.
	go func() {
		time.Sleep(3500 * time.Millisecond)
		fmt.Println("--- triggering shutdown via SIGTERM ---")
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	}()

	if err := mgr.Listen(context.Background()); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
