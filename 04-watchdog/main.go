// Example 04-watchdog — a deliberately broken handler that ignores ctx.
//
// Run this and watch the watchdog hard-exit the process after the budget
// plus grace period. The "good" exit code we set via WithExitOnComplete
// is NEVER reached because the handler hangs; the watchdog's exit code
// (here 99) takes over.
//
// Try setting WithBudget high enough that the handler finishes — the
// watchdog won't fire and the program exits cleanly.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ubgo/shutdown"
)

func main() {
	mgr := shutdown.New(
		shutdown.WithBudget(200*time.Millisecond),
		shutdown.WithWatchdogGrace(50*time.Millisecond),
		shutdown.WithExitOnComplete(0, 99),
	)

	_ = mgr.Register("hangs", func(_ context.Context) error {
		fmt.Println("hangs: started, ignoring ctx for 5 seconds")
		time.Sleep(5 * time.Second) // ignores ctx — the bug we want to detect
		return nil
	}, shutdown.WithTimeout(5*time.Second))

	fmt.Println("triggering shutdown; watchdog will hard-exit after ~250ms")
	if err := mgr.Shutdown(context.Background()); err != nil {
		fmt.Println("shutdown error:", err)
	}
}
