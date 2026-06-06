package tracing

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Init initializes the OpenTelemetry tracer provider and returns a shutdown function.
func Init(ctx context.Context, serviceName, jaegerEndpoint string) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(jaegerEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(time.Second)),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// Middleware returns a Fiber middleware that creates spans for HTTP requests.
func Middleware(serviceName string) fiber.Handler {
	tracer := otel.Tracer(serviceName)

	return func(c *fiber.Ctx) error {
		if c.Path() == "/metrics" || c.Path() == "/health" {
			return c.Next()
		}

		// Extract context from incoming headers
		ctx := otel.GetTextMapPropagator().Extract(c.Context(), &fiberHeaderCarrier{ctx: c})

		spanName := c.Method() + " " + c.Route().Path
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(c.Method()),
				semconv.URLPath(c.Path()),
				semconv.ServerAddress(c.Hostname()),
			),
		)
		defer span.End()

		c.SetUserContext(ctx)

		err := c.Next()

		status := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.response.status_code", status))
		if status >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
		}

		return err
	}
}

// InjectContext injects trace context into outgoing gRPC metadata.
// Use with c.UserContext() from Fiber handler.
func InjectContext(ctx context.Context) context.Context {
	return ctx
}

// fiberHeaderCarrier adapts Fiber request headers for OpenTelemetry propagation.
type fiberHeaderCarrier struct {
	ctx *fiber.Ctx
}

func (c *fiberHeaderCarrier) Get(key string) string {
	return c.ctx.Get(key)
}

func (c *fiberHeaderCarrier) Set(key, value string) {
	c.ctx.Set(key, value)
}

func (c *fiberHeaderCarrier) Keys() []string {
	keys := make([]string, 0)
	c.ctx.Request().Header.VisitAll(func(k, v []byte) {
		keys = append(keys, string(k))
	})
	return keys
}

// SpanFromContext creates a child span from context — useful for wrapping gRPC calls.
func SpanFromContext(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer("").Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient))
}

// SetStatusFromGRPC sets span status based on gRPC error.
func SetStatusFromGRPC(span trace.Span, err error) {
	if err != nil {
		span.SetAttributes(attribute.Bool("error", true))
		span.SetAttributes(attribute.String("error.message", err.Error()))
	}
}

// SetStatusFromHTTP sets span attributes for HTTP status code.
func SetStatusFromHTTP(span trace.Span, statusCode int) {
	span.SetAttributes(attribute.String("http.response.status_code", strconv.Itoa(statusCode)))
}
