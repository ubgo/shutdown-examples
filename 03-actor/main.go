// Example 03-actor — register a long-running goroutine via RegisterActor.
//
// The actor pattern fits "background worker" services: the run loop and
// the cancel mechanism are distinct. The manager calls the interrupt; the
// caller's goroutine signals completion via handle.Done.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ubgo/shutdown"
)

func main() {
	mgr := shutdown.New(shutdown.WithBudget(5 * time.Second))

	// Channel that the run loop watches for "stop" signal.
	stop := make(chan struct{})

	handle, err := mgr.RegisterActor("worker", func(_ error) {
		fmt.Println("worker: interrupt called by shutdown")
		close(stop)
	}, shutdown.WithActorPhase(shutdown.PhaseDrainTraffic))
	if err != nil {
		panic(err)
	}

	// The run loop. In real services this is the worker's main loop.
	go func() {
		fmt.Println("worker: started")
		<-stop
		// Imagine cleanup here — flushing buffers, etc.
		time.Sleep(20 * time.Millisecond)
		fmt.Println("worker: clean exit")
		handle.Done(nil)
	}()

	// Let the worker run for a moment, then trigger shutdown.
	time.Sleep(50 * time.Millisecond)
	if err := mgr.Shutdown(context.Background()); err != nil {
		fmt.Println("shutdown error:", err)
	} else {
		fmt.Println("shutdown clean")
	}
}
