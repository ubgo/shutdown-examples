module github.com/ubgo/shutdown-examples/06-with-otel

go 1.24

require (
	github.com/ubgo/shutdown v0.2.0
	github.com/ubgo/shutdown/contrib/shutdown-otel v0.1.1
	go.opentelemetry.io/otel v1.32.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.32.0
	go.opentelemetry.io/otel/sdk v1.32.0
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/otel/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	golang.org/x/sys v0.27.0 // indirect
)
