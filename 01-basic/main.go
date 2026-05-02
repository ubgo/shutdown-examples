// Example 01-basic — programmatic Shutdown(ctx) without OS signals.
//
// Demonstrates the smallest possible use of ubgo/shutdown: register a
// couple of close functions, call Shutdown, observe phase ordering and
// the aggregated error.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ubgo/shutdown"
)

func main() {
	mgr := shutdown.New(shutdown.WithBudget(5 * time.Second))

	_ = mgr.Register("db", func(_ context.Context) error {
		fmt.Println("db: closing")
		return nil
	}, shutdown.WithPhase(shutdown.PhaseCloseClients))

	_ = mgr.Register("redis", func(_ context.Context) error {
		fmt.Println("redis: closing")
		return errors.New("redis: connection already lost")
	}, shutdown.WithPhase(shutdown.PhaseCloseClients))

	_ = mgr.Register("logs", func(_ context.Context) error {
		fmt.Println("logs: flushing")
		return nil
	}, shutdown.WithPhase(shutdown.PhaseFlushLogs))

	if err := mgr.Shutdown(context.Background()); err != nil {
		fmt.Println("shutdown returned aggregate error:", err)
	} else {
		fmt.Println("shutdown clean")
	}
}
