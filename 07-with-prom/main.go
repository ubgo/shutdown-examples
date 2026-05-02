// Example 07-with-prom — export Prometheus metrics for shutdown phases.
//
// After shutdown completes the program writes the metric snapshot to
// stdout. In a real service these are scraped via /metrics on your existing
// Prometheus handler.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/ubgo/shutdown"
	shutdownprom "github.com/ubgo/shutdown/contrib/shutdown-prom"
)

func main() {
	reg := prometheus.NewRegistry()
	m := shutdownprom.NewMetrics(reg)

	mgr := shutdown.New(shutdown.WithBudget(2 * time.Second))
	mgr.Subscribe(shutdownprom.Observer(m))

	_ = mgr.Register("db",    func(_ context.Context) error { return nil })
	_ = mgr.Register("redis", func(_ context.Context) error { return errors.New("connection refused") })

	_ = mgr.Shutdown(context.Background())

	// Dump the metric values.
	mfs, _ := reg.Gather()
	enc := expfmt.NewEncoder(textWriter{}, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, mf := range mfs {
		_ = enc.Encode(mf)
	}
	fmt.Println()
}

type textWriter struct{}

func (textWriter) Write(p []byte) (int, error) {
	fmt.Print(string(p))
	return len(p), nil
}
