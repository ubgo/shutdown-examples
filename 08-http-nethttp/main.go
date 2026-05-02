// Example 08-http-nethttp — graceful shutdown of a stdlib *http.Server.
//
// Run, then `curl localhost:8080/`, then send SIGINT (Ctrl-C). Watch the
// server stop accepting new connections; the in-flight request is allowed
// to finish thanks to PhaseStopAccepting < PhaseDrainTraffic.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ubgo/shutdown"
	shutdownnethttp "github.com/ubgo/shutdown/contrib/shutdown-nethttp"
)

func main() {
	mgr := shutdown.New(shutdown.WithBudget(15 * time.Second))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		// Simulate a slow handler so the drain matters.
		time.Sleep(2 * time.Second)
		_, _ = fmt.Fprintln(w, "hello")
	})
	srv := &http.Server{Addr: ":8080", Handler: mux}

	if err := shutdownnethttp.Register(mgr, srv); err != nil {
		panic(err)
	}

	go func() {
		fmt.Println("listening on :8080 — Ctrl-C to shut down")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("listen:", err)
		}
	}()

	if err := mgr.Listen(context.Background()); err != nil {
		fmt.Println("shutdown:", err)
	}
}
