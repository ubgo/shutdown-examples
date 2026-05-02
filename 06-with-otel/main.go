// Example 06-with-otel — emit OTEL spans for shutdown phases + handlers.
//
// Each shutdown gets a root "shutdown" span; one child span per phase;
// one leaf span per handler. Errors are recorded on the leaf span via
// span.RecordError + SetStatus(Error).
//
// This example uses the stdout exporter so spans land in your terminal.
// In production point your TracerProvider at OTLP / Jaeger / etc.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ubgo/shutdown"
	shutdownotel "github.com/ubgo/shutdown/contrib/shutdown-otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	mgr := shutdown.New(shutdown.WithBudget(2 * time.Second))
	mgr.Subscribe(shutdownotel.Observer(tp.Tracer("shutdown-example")))

	_ = mgr.Register("db",    func(_ context.Context) error { return nil })
	_ = mgr.Register("redis", func(_ context.Context) error { return errors.New("connection refused") })

	_ = mgr.Shutdown(context.Background())
	fmt.Println("done")
}
