// Example 05-with-zap — wire the shutdown-zap observer into your zap pipeline.
//
// The manager's internal Logger still uses log/slog by default; the zap
// observer is a *separate* event stream — every phase + handler boundary
// is logged through your zap config (encoder, level, sinks).
package main

import (
	"context"
	"errors"
	"time"

	"github.com/ubgo/shutdown"
	shutdownzap "github.com/ubgo/shutdown/contrib/shutdown-zap"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	mgr := shutdown.New(
		shutdown.WithLogger(shutdown.NoopLogger()), // mute internal logger; zap observer is the source of truth
		shutdown.WithBudget(2*time.Second),
	)
	mgr.Subscribe(shutdownzap.Observer(logger))

	_ = mgr.Register("ok", func(_ context.Context) error { return nil })
	_ = mgr.Register("fail", func(_ context.Context) error { return errors.New("boom") })

	_ = mgr.Shutdown(context.Background())
}
