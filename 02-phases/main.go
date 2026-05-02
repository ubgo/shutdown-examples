// Example 02-phases — every predefined phase populated.
//
// Watch the output: handlers fire in phase order even though they were
// registered in random order. Within a phase they run in parallel.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ubgo/shutdown"
)

func handler(label string) shutdown.HandlerFunc {
	return func(_ context.Context) error {
		time.Sleep(50 * time.Millisecond)
		fmt.Printf("[%s] done\n", label)
		return nil
	}
}

func main() {
	mgr := shutdown.New(shutdown.WithBudget(10 * time.Second))

	_ = mgr.Register("logs",   handler("logs"),   shutdown.WithPhase(shutdown.PhaseFlushLogs))
	_ = mgr.Register("redis",  handler("redis"),  shutdown.WithPhase(shutdown.PhaseCloseClients))
	_ = mgr.Register("nats",   handler("nats"),   shutdown.WithPhase(shutdown.PhaseDrainTraffic))
	_ = mgr.Register("readiness-flip", handler("readiness-flip"), shutdown.WithPhase(shutdown.PhasePreShutdown))
	_ = mgr.Register("http",   handler("http"),   shutdown.WithPhase(shutdown.PhaseStopAccepting))
	_ = mgr.Register("db",     handler("db"),     shutdown.WithPhase(shutdown.PhaseCloseClients))
	_ = mgr.Register("queues", handler("queues"), shutdown.WithPhase(shutdown.PhaseFlushQueues))

	_ = mgr.Shutdown(context.Background())
}
