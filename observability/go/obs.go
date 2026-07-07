// Package observability centraliza la instrumentación de telemetría para los
// microservicios de FoodRush. Ofrece inicialización de trazas OpenTelemetry,
// exposición de métricas Prometheus, logging estructurado con trace_id y
// helpers para crear spans de base de datos.
package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	logger     *zerolog.Logger
	logService string

	reqCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total de requests HTTP y gRPC por handler y status.",
		},
		[]string{"handler", "status"},
	)

	reqDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duración de requests HTTP y gRPC por handler y status.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"handler", "status"},
	)
)

func init() {
	var l zerolog.Logger
	if os.Getenv("LOG_FORMAT") == "console" {
		l = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			With().Timestamp().Logger()
	} else {
		l = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}
	logger = &l
}

// SetServiceName configura el nombre del servicio que se incluye en cada log.
func SetServiceName(name string) {
	logService = name
	l := logger.With().Str("service", name).Logger()
	logger = &l
}

// Logger devuelve el logger global del servicio.
func Logger() *zerolog.Logger {
	return logger
}

// CtxLogger devuelve un logger enriquecido con el trace_id activo en ctx,
// si existe. De esta forma logs, métricas y trazas quedan correlacionados.
func CtxLogger(ctx context.Context) *zerolog.Logger {
	l := logger
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		ll := l.With().Str("trace_id", span.SpanContext().TraceID().String()).Logger()
		l = &ll
	}
	return l
}

// InitTracer inicializa el TracerProvider OTLP para enviar trazas a Jaeger.
// El endpoint se lee de OTEL_EXPORTER_OTLP_ENDPOINT (default http://jaeger:4318).
// La tasa de muestreo se controla con OTEL_TRACES_SAMPLER_ARG (default 1.0).
func InitTracer(serviceName string) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	ctx := context.Background()

	SetServiceName(serviceName)

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://jaeger:4318"
	}
	exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, nil, fmt.Errorf("crear exporter OTLP: %w", err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
	)

	ratio := 1.0
	if r, err := strconv.ParseFloat(os.Getenv("OTEL_TRACES_SAMPLER_ARG"), 64); err == nil {
		ratio = r
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ratio)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logger.Info().Str("endpoint", endpoint).Float64("sampler", ratio).Msg("tracer inicializado")
	return tp, tp.Shutdown, nil
}

// StartDBSpan crea un span "db.query" con atributos del sistema y la sentencia.
// El caller debe llamar defer span.End() después de recibir el span.
func StartDBSpan(ctx context.Context, system, statement string) (context.Context, trace.Span) {
	tracer := otel.Tracer("db")
	ctx, span := tracer.Start(ctx, "db.query")
	span.SetAttributes(
		attribute.String("db.system", system),
		attribute.String("db.statement", statement),
	)
	return ctx, span
}

// TraceHTTPHandler envuelve un http.Handler con el instrumentation de OpenTelemetry.
func TraceHTTPHandler(handler http.Handler, operation string) http.Handler {
	return otelhttp.NewHandler(handler, operation)
}

// responseRecorder captura el status code HTTP escrito por el handler siguiente.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

// HTTPMetricsMiddleware registra contador e histograma para cada request HTTP.
// Compone bien con otelhttp.NewHandler si se aplica después del handler OpenTelemetry.
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		handler := r.Pattern
		if handler == "" {
			handler = r.Method + " " + r.URL.Path
		}
		status := strconv.Itoa(wrapped.statusCode)

		reqCounter.WithLabelValues(handler, status).Inc()
		reqDuration.WithLabelValues(handler, status).Observe(duration)
	})
}

// StartMetricsServer expone /metrics en un servidor HTTP paralelo.
// Los servicios gRPC deben usar un puerto independiente, p.ej. :9000.
func StartMetricsServer(addr string) {
	if addr == "" {
		addr = ":9000"
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info().Str("addr", addr).Msg("metrics server iniciado")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Str("addr", addr).Msg("metrics server detenido")
		}
	}()
}

// grpcMetricsInterceptor mide duración y status de las llamadas gRPC entrantes.
func grpcMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start).Seconds()

		st, _ := status.FromError(err)
		statusCode := st.Code().String()

		reqCounter.WithLabelValues(info.FullMethod, statusCode).Inc()
		reqDuration.WithLabelValues(info.FullMethod, statusCode).Observe(duration)

		return resp, err
	}
}

// GRPCServerOptions devuelve las opciones de servidor gRPC con tracing y métricas.
func GRPCServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(grpcMetricsInterceptor()),
	}
}

// GRPCClientStatsHandler devuelve una grpc.DialOption para propagar traceparent
// en las llamadas gRPC salientes.
func GRPCClientStatsHandler() grpc.DialOption {
	return grpc.WithStatsHandler(otelgrpc.NewClientHandler())
}
