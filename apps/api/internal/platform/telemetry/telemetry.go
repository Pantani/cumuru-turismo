package telemetry

import (
	"context"
	"errors"
	"net/http"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type Provider struct {
	tracerProvider *sdktrace.TracerProvider
}

func New(ctx context.Context, cfg config.TelemetryConfig, serviceName, serviceVersion string) (*Provider, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", serviceVersion),
		),
	)
	if err != nil {
		return nil, errors.New("telemetry resource initialization failed")
	}
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	}
	if cfg.Exporter == "otlp" {
		client := &http.Client{Timeout: cfg.Timeout}
		exporter, err := otlptracehttp.New(
			ctx,
			otlptracehttp.WithEndpointURL(cfg.Endpoint),
			otlptracehttp.WithHTTPClient(client),
		)
		if err != nil {
			return nil, errors.New("telemetry exporter initialization failed")
		}
		options = append(options, sdktrace.WithBatcher(exporter))
	}
	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	return &Provider{tracerProvider: provider}, nil
}

func (p *Provider) Tracer(name string) trace.Tracer {
	return p.tracerProvider.Tracer(name)
}

func (p *Provider) Shutdown(ctx context.Context) error {
	return p.tracerProvider.Shutdown(ctx)
}
