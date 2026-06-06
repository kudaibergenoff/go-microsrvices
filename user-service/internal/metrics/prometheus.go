package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpRequestsInFlight)
}

// Middleware returns a Fiber middleware that collects Prometheus metrics.
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Path() == "/metrics" || c.Path() == "/health" {
			return c.Next()
		}

		start := time.Now()
		httpRequestsInFlight.Inc()

		err := c.Next()

		httpRequestsInFlight.Dec()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		path := c.Route().Path

		httpRequestsTotal.WithLabelValues(c.Method(), path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Method(), path).Observe(duration)

		return err
	}
}

// Handler returns a Fiber handler that serves Prometheus metrics.
func Handler() fiber.Handler {
	handler := promhttp.Handler()
	return func(c *fiber.Ctx) error {
		fasthttpadaptor.NewFastHTTPHandler(handler)(c.Context())
		return nil
	}
}
