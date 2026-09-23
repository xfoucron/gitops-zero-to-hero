package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

func otlpEnabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
}

var logger = slog.Default()

func Logger() *slog.Logger {
	return logger
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func Setup(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)

	var shutdownFuncs []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var errs error
		for _, fn := range shutdownFuncs {
			errs = errors.Join(errs, fn(ctx))
		}
		return errs
	}

	tp, err := setupTraces(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("setup traces: %w", err)
	}
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	mp, err := setupMetrics(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("setup metrics: %w", err)
	}
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	otel.SetMeterProvider(mp)

	lp, err := setupLogs(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("setup logs: %w", err)
	}
	shutdownFuncs = append(shutdownFuncs, lp.Shutdown)
	logger = otelslog.NewLogger(serviceName, otelslog.WithLoggerProvider(lp))

	return shutdown, nil
}

func setupTraces(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	if otlpEnabled() {
		exporter, err = otlptracehttp.New(ctx)
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	), nil
}

func setupMetrics(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	var exporter sdkmetric.Exporter
	var err error

	if otlpEnabled() {
		exporter, err = otlpmetrichttp.New(ctx)
	} else {
		exporter, err = stdoutmetric.New()
	}
	if err != nil {
		return nil, err
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second))),
	), nil
}

func setupLogs(ctx context.Context, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	var exporter sdklog.Exporter
	var err error

	if otlpEnabled() {
		exporter, err = otlploghttp.New(ctx)
	} else {
		exporter, err = stdoutlog.New()
	}
	if err != nil {
		return nil, err
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	), nil
}
