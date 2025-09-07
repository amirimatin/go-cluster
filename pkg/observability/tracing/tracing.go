package tracing

import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var enabled bool

// Setup configures a global tracer provider when enable=true.
// It returns a shutdown function which should be deferred.
func Setup(enable bool) (func(context.Context) error, error) {
    enabled = enable
    if !enable {
        return func(context.Context) error { return nil }, nil
    }
    exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
    if err != nil {
        return nil, err
    }
    tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
    otel.SetTracerProvider(tp)
    return tp.Shutdown, nil
}

// StartSpan starts a tracing span if tracing is enabled.
func StartSpan(ctx context.Context, name string) (context.Context, func()) {
    if !enabled {
        return ctx, func() {}
    }
    tr := otel.Tracer("go-cluster")
    ctx, span := tr.Start(ctx, name)
    // wrap End to match func() signature
    return ctx, func() { span.End() }
}
